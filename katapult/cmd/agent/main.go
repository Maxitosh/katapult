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

const (
	jwtTokenPath        = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	registrationRetries = 10
	retryBaseDelay      = 2 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil {
		logger.Error("agent exited with error", "error", err)
		os.Exit(1)
	}
}

// @cpt-flow:cpt-katapult-flow-agent-system-register:p1
// @cpt-flow:cpt-katapult-flow-agent-system-heartbeat:p1
// @cpt-flow:cpt-katapult-flow-agent-system-discover-pvcs:p1
func run(logger *slog.Logger) error {
	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-agent-start
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
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-agent-start

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-verify-tools
	// Verify required tools on this node.
	tools, err := agent.VerifyTools()
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-verify-tools
	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-check-tools-fail
	if err != nil {
		return fmt.Errorf("tool verification failed, aborting startup: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-check-tools-fail
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

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-initial-discover
	// @cpt-begin:cpt-katapult-flow-agent-system-discover-pvcs:p1:inst-query-pvcs
	// Initial PVC discovery.
	pvcs, err := discoverer.Discover(ctx)
	if err != nil {
		return fmt.Errorf("initial PVC discovery failed: %w", err)
	}
	logger.Info("discovered PVCs", "count", len(pvcs))
	// @cpt-end:cpt-katapult-flow-agent-system-discover-pvcs:p1:inst-query-pvcs
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-initial-discover

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-read-jwt
	// Read ServiceAccount JWT token.
	jwtToken, err := os.ReadFile(jwtTokenPath)
	if err != nil {
		return fmt.Errorf("reading ServiceAccount JWT: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-read-jwt

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-grpc-connect
	// Build TLS config if CA cert is provided.
	var tlsCfg *agent.TLSConfig
	tlsCACert := config.EnvOrDefault("TLS_CA_CERT", "")
	if tlsCACert != "" {
		tlsCfg = &agent.TLSConfig{
			CACertPath:     tlsCACert,
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
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-grpc-connect

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-grpc-register
	// Register with control plane (with retry).
	var agentID string
	if err := retryRegistration(ctx, logger, func() error {
		id, rerr := client.Register(ctx, clusterID, nodeName, string(jwtToken), tools, pvcs)
		if rerr != nil {
			return rerr
		}
		agentID = id
		return nil
	}); err != nil {
		return fmt.Errorf("registration failed after %d retries: %w", registrationRetries, err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-grpc-register

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-return-registered
	logger.Info("agent registered", "agent_id", agentID)
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-return-registered

	// Build re-registration closure for the heartbeat loop.
	// Re-reads the JWT token (it may have been rotated) and re-discovers PVCs.
	regFunc := func() error {
		token, rerr := os.ReadFile(jwtTokenPath)
		if rerr != nil {
			return fmt.Errorf("re-reading JWT token: %w", rerr)
		}
		freshPVCs, rerr := discoverer.Discover(ctx)
		if rerr != nil {
			logger.Warn("PVC discovery failed during re-registration, using empty PVCs", "error", rerr)
			freshPVCs = nil
		}
		_, rerr = client.Register(ctx, clusterID, nodeName, string(token), tools, freshPVCs)
		return rerr
	}

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-wait-interval
	// Run heartbeat loop until shutdown.
	client.RunHeartbeatLoop(ctx, heartbeatInterval, discoverer, regFunc)
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-wait-interval

	logger.Info("agent shutting down")
	return nil
}

// retryRegistration attempts registration with exponential backoff.
func retryRegistration(ctx context.Context, logger *slog.Logger, regFunc func() error) error {
	var lastErr error
	for attempt := range registrationRetries {
		if err := regFunc(); err != nil {
			lastErr = err
			logger.Warn("registration attempt failed", "attempt", attempt+1, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryBaseDelay * time.Duration(1<<attempt)):
			}
			continue
		}
		return nil
	}
	return lastErr
}
