package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maxitosh/katapult/internal/agent"
	"github.com/maxitosh/katapult/internal/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil {
		logger.Error("agent exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	controlPlaneAddr := config.EnvOrDefault("CONTROL_PLANE_ADDR", "localhost:50051")
	clusterID := config.EnvOrDefault("CLUSTER_ID", "")
	nodeName := config.EnvOrDefault("NODE_NAME", "")
	heartbeatIntervalStr := config.EnvOrDefault("HEARTBEAT_INTERVAL", "30s")

	if clusterID == "" {
		return fmt.Errorf("CLUSTER_ID environment variable is required")
	}
	if nodeName == "" {
		return fmt.Errorf("NODE_NAME environment variable is required")
	}

	heartbeatInterval, err := time.ParseDuration(heartbeatIntervalStr)
	if err != nil {
		return fmt.Errorf("parsing HEARTBEAT_INTERVAL: %w", err)
	}

	// Verify required tools on this node.
	tools, err := agent.VerifyTools()
	if err != nil {
		return fmt.Errorf("tool verification failed, aborting startup: %w", err)
	}
	logger.Info("tools verified", "tar", tools.Tar, "zstd", tools.Zstd, "stunnel", tools.Stunnel)

	// Create in-cluster Kubernetes client for PVC discovery.
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("creating in-cluster config: %w", err)
	}
	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}

	discoverer := agent.NewPVCDiscoverer(k8sClient, agent.DiscoveryConfig{
		NodeName: nodeName,
	}, logger)

	// Initial PVC discovery.
	pvcs, err := discoverer.Discover(ctx)
	if err != nil {
		return fmt.Errorf("initial PVC discovery failed: %w", err)
	}
	logger.Info("discovered PVCs", "count", len(pvcs))

	// Read ServiceAccount JWT token.
	jwtToken, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return fmt.Errorf("reading ServiceAccount JWT: %w", err)
	}

	// Build TLS config if CA cert is provided.
	var tlsCfg *agent.TLSConfig
	tlsCACert := config.EnvOrDefault("TLS_CA_CERT", "")
	if tlsCACert != "" {
		tlsCfg = &agent.TLSConfig{
			CACertPath:    tlsCACert,
			ClientCertPath: config.EnvOrDefault("TLS_CERT", ""),
			ClientKeyPath:  config.EnvOrDefault("TLS_KEY", ""),
		}
	}

	// Connect to control plane.
	client, err := agent.NewClient(controlPlaneAddr, tlsCfg, logger)
	if err != nil {
		return fmt.Errorf("connecting to control plane: %w", err)
	}
	defer client.Close()

	// Register with control plane.
	agentID, err := client.Register(ctx, clusterID, nodeName, string(jwtToken), tools, pvcs)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	logger.Info("agent registered", "agent_id", agentID)

	// Run heartbeat loop until shutdown.
	client.RunHeartbeatLoop(ctx, heartbeatInterval, discoverer)

	logger.Info("agent shutting down")
	return nil
}
