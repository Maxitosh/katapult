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

	"github.com/maxitosh/katapult/internal/registry"
	"github.com/maxitosh/katapult/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	agentgrpc "github.com/maxitosh/katapult/internal/grpc"
	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
	"google.golang.org/grpc"
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

	databaseURL := envOrDefault("DATABASE_URL", "postgres://localhost:5432/katapult?sslmode=disable")
	listenAddr := envOrDefault("GRPC_LISTEN_ADDR", ":50051")
	unhealthyTimeoutStr := envOrDefault("UNHEALTHY_TIMEOUT", "90s")
	disconnectedTimeoutStr := envOrDefault("DISCONNECTED_TIMEOUT", "5m")
	healthCheckIntervalStr := envOrDefault("HEALTH_CHECK_INTERVAL", "30s")

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

	agentServer := agentgrpc.NewAgentServer(svc)
	grpcServer := grpc.NewServer()
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

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
