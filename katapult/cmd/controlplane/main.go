package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
	"github.com/maxitosh/katapult/internal/config"
	agentgrpc "github.com/maxitosh/katapult/internal/grpc"
	"github.com/maxitosh/katapult/internal/registry"
	"github.com/maxitosh/katapult/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil {
		logger.Error("control plane exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	databaseURL := config.EnvOrDefault("DATABASE_URL", "postgres://localhost:5432/katapult?sslmode=disable")
	listenAddr := config.EnvOrDefault("GRPC_LISTEN_ADDR", ":50051")
	unhealthyTimeoutStr := config.EnvOrDefault("UNHEALTHY_TIMEOUT", "90s")
	disconnectedTimeoutStr := config.EnvOrDefault("DISCONNECTED_TIMEOUT", "5m")
	healthCheckIntervalStr := config.EnvOrDefault("HEALTH_CHECK_INTERVAL", "30s")

	unhealthyTimeout, err := time.ParseDuration(unhealthyTimeoutStr)
	if err != nil {
		return fmt.Errorf("parsing UNHEALTHY_TIMEOUT: %w", err)
	}
	disconnectedTimeout, err := time.ParseDuration(disconnectedTimeoutStr)
	if err != nil {
		return fmt.Errorf("parsing DISCONNECTED_TIMEOUT: %w", err)
	}
	healthCheckInterval, err := time.ParseDuration(healthCheckIntervalStr)
	if err != nil {
		return fmt.Errorf("parsing HEALTH_CHECK_INTERVAL: %w", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}
	logger.Info("connected to database")

	repo := postgres.NewAgentRepository(pool)
	svc := registry.NewService(repo, logger)

	healthEval := registry.NewHealthEvaluator(repo, unhealthyTimeout, disconnectedTimeout, logger)
	go healthEval.RunLoop(ctx, healthCheckInterval)

	var serverOpts []grpc.ServerOption

	// JWT authentication.
	jwtPublicKeyPath := config.EnvOrDefault("JWT_PUBLIC_KEY_PATH", "")
	expectedAgentSA := config.EnvOrDefault("EXPECTED_AGENT_SA", "")
	if jwtPublicKeyPath != "" {
		keyData, err := os.ReadFile(jwtPublicKeyPath)
		if err != nil {
			return fmt.Errorf("reading JWT public key: %w", err)
		}
		keyFunc, err := agentgrpc.StaticKeyFunc(keyData)
		if err != nil {
			return fmt.Errorf("creating JWT key func: %w", err)
		}
		validator := agentgrpc.NewJWTValidator(expectedAgentSA, keyFunc)
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(agentgrpc.UnaryAuthInterceptor(validator)))
		logger.Info("JWT authentication enabled", "expected_sa", expectedAgentSA)
	} else {
		logger.Warn("JWT_PUBLIC_KEY_PATH not set, running without JWT authentication")
	}

	// TLS credentials.
	tlsCert := config.EnvOrDefault("TLS_CERT", "")
	tlsKey := config.EnvOrDefault("TLS_KEY", "")
	if tlsCert != "" && tlsKey != "" {
		creds, err := credentials.NewServerTLSFromFile(tlsCert, tlsKey)
		if err != nil {
			return fmt.Errorf("loading TLS credentials: %w", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		logger.Info("TLS enabled")
	} else {
		logger.Warn("TLS_CERT/TLS_KEY not set, running without TLS")
	}

	agentServer := agentgrpc.NewAgentServer(svc)
	grpcServer := grpc.NewServer(serverOpts...)
	pb.RegisterAgentServiceServer(grpcServer, agentServer)

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", listenAddr, err)
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down gRPC server")
		grpcServer.GracefulStop()
	}()

	logger.Info("control plane started", "addr", listenAddr)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server error: %w", err)
	}

	return nil
}
