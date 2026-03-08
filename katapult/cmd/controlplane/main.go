package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
	"github.com/maxitosh/katapult/internal/config"
	"github.com/maxitosh/katapult/internal/domain"
	agentgrpc "github.com/maxitosh/katapult/internal/grpc"
	katapulthttp "github.com/maxitosh/katapult/internal/http"
	"github.com/maxitosh/katapult/internal/registry"
	"github.com/maxitosh/katapult/internal/store/postgres"
	"github.com/maxitosh/katapult/internal/transfer"
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
	httpListenAddr := config.EnvOrDefault("HTTP_LISTEN_ADDR", ":8080")
	apiTokensFile := config.EnvOrDefault("API_TOKENS_FILE", "")
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

	agentRepo := postgres.NewAgentRepository(pool)
	registrySvc := registry.NewService(agentRepo, logger)

	healthEval := registry.NewHealthEvaluator(agentRepo, unhealthyTimeout, disconnectedTimeout, logger)
	go healthEval.RunLoop(ctx, healthCheckInterval)

	// Transfer orchestrator with no-op stubs for agent communication.
	transferRepo := postgres.NewTransferRepository(pool)
	commander := transfer.NoopCommander{}
	credManager := transfer.NoopCredentialManager{}
	s3Client := transfer.NoopS3Client{}
	pvcFinder := transfer.NoopPVCFinder{}

	validator := transfer.NewValidator(pvcFinder, commander, logger)
	cleaner := transfer.NewCleaner(credManager, commander, s3Client, logger)
	orchestrator := transfer.NewOrchestrator(
		transferRepo, validator, cleaner, commander, credManager,
		transfer.S3Config{}, domain.DefaultTransferConfig(), logger,
	)

	// HTTP REST API server.
	tokenValidator := loadTokenValidator(apiTokensFile, logger)
	httpServer := katapulthttp.NewServer(orchestrator, registrySvc, logger)
	httpSrv := &stdhttp.Server{
		Addr:    httpListenAddr,
		Handler: httpServer.Handler(tokenValidator),
	}

	go func() {
		logger.Info("HTTP API server starting", "addr", httpListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// gRPC server setup.
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
		jwtValidator := agentgrpc.NewJWTValidator(expectedAgentSA, keyFunc)
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(agentgrpc.UnaryAuthInterceptor(jwtValidator)))
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

	agentServer := agentgrpc.NewAgentServer(registrySvc, nil)
	grpcServer := grpc.NewServer(serverOpts...)
	pb.RegisterAgentServiceServer(grpcServer, agentServer)

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", listenAddr, err)
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down servers")
		grpcServer.GracefulStop()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	logger.Info("control plane started", "grpc_addr", listenAddr, "http_addr", httpListenAddr)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server error: %w", err)
	}

	return nil
}

// tokenFileEntry represents a single entry in the API tokens JSON file.
type tokenFileEntry struct {
	Token   string `json:"token"`
	Subject string `json:"subject"`
	Role    string `json:"role"`
}

func loadTokenValidator(path string, logger *slog.Logger) katapulthttp.TokenValidator {
	if path == "" {
		logger.Warn("API_TOKENS_FILE not set, using empty token map")
		return katapulthttp.NewStaticTokenValidator(nil)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		logger.Error("failed to read API tokens file", "path", path, "error", err)
		return katapulthttp.NewStaticTokenValidator(nil)
	}

	var entries []tokenFileEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		logger.Error("failed to parse API tokens file", "path", path, "error", err)
		return katapulthttp.NewStaticTokenValidator(nil)
	}

	tokens := make(map[string]katapulthttp.UserInfo, len(entries))
	for _, e := range entries {
		tokens[e.Token] = katapulthttp.UserInfo{
			Subject: e.Subject,
			Role:    katapulthttp.Role(e.Role),
		}
	}

	logger.Info("loaded API tokens", "count", len(tokens))
	return katapulthttp.NewStaticTokenValidator(tokens)
}
