package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/observability"
	"github.com/maxitosh/katapult/internal/transfer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// @cpt-dod:cpt-katapult-dod-api-cli-rest-transfer-endpoints:p1
// @cpt-dod:cpt-katapult-dod-api-cli-rest-agent-endpoints:p1

// TransferService defines the transfer operations used by the HTTP handlers.
type TransferService interface {
	CreateTransfer(ctx context.Context, req transfer.CreateTransferRequest) (*transfer.CreateTransferResponse, error)
	CancelTransfer(ctx context.Context, transferID uuid.UUID) error
	GetTransfer(ctx context.Context, id uuid.UUID) (*domain.Transfer, error)
	ListTransfers(ctx context.Context, filter domain.TransferFilter) ([]domain.Transfer, int, error)
	GetTransferEvents(ctx context.Context, transferID uuid.UUID) ([]domain.TransferEvent, error)
}

// RegistryService defines the agent registry operations used by the HTTP handlers.
type RegistryService interface {
	GetAgent(ctx context.Context, agentID uuid.UUID) (*domain.Agent, error)
	ListAgents(ctx context.Context, filter domain.AgentFilter) ([]domain.Agent, int, error)
	ListClusters(ctx context.Context) ([]string, error)
}

// Server is the HTTP REST API server for the control plane.
type Server struct {
	orchestrator   TransferService
	registrySvc    RegistryService
	progressHub    observability.ProgressSubscriber
	metricsHandler http.Handler
	logger         *slog.Logger
}

// ServerOption configures optional Server dependencies.
type ServerOption func(*Server)

// WithProgressHub sets the progress hub for SSE streaming.
func WithProgressHub(hub observability.ProgressSubscriber) ServerOption {
	return func(s *Server) {
		s.progressHub = hub
	}
}

// WithMetricsHandler sets the Prometheus metrics handler.
func WithMetricsHandler(reg *prometheus.Registry) ServerOption {
	return func(s *Server) {
		s.metricsHandler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	}
}

// NewServer creates a new HTTP API server.
func NewServer(orchestrator TransferService, registrySvc RegistryService, logger *slog.Logger, opts ...ServerOption) *Server {
	s := &Server{
		orchestrator: orchestrator,
		registrySvc:  registrySvc,
		logger:       logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Handler returns an http.Handler with all routes and middleware wired up.
func (s *Server) Handler(validator TokenValidator) http.Handler {
	mux := http.NewServeMux()

	auth := AuthMiddleware(validator)
	viewer := RequireRole(RoleViewer)
	operator := RequireRole(RoleOperator)

	// Transfer endpoints
	mux.Handle("POST /api/v1alpha1/transfers", auth(operator(http.HandlerFunc(s.handleCreateTransfer))))
	mux.Handle("GET /api/v1alpha1/transfers", auth(viewer(http.HandlerFunc(s.handleListTransfers))))
	mux.Handle("GET /api/v1alpha1/transfers/{id}", auth(viewer(http.HandlerFunc(s.handleGetTransfer))))
	mux.Handle("DELETE /api/v1alpha1/transfers/{id}", auth(operator(http.HandlerFunc(s.handleCancelTransfer))))
	mux.Handle("GET /api/v1alpha1/transfers/{id}/events", auth(viewer(http.HandlerFunc(s.handleGetTransferEvents))))

	// Agent endpoints
	mux.Handle("GET /api/v1alpha1/agents", auth(viewer(http.HandlerFunc(s.handleListAgents))))
	mux.Handle("GET /api/v1alpha1/agents/{id}", auth(viewer(http.HandlerFunc(s.handleGetAgent))))
	mux.Handle("GET /api/v1alpha1/agents/{id}/pvcs", auth(viewer(http.HandlerFunc(s.handleGetAgentPVCs))))

	// Cluster endpoints
	mux.Handle("GET /api/v1alpha1/clusters", auth(viewer(http.HandlerFunc(s.handleListClusters))))

	// SSE progress streaming
	// @cpt-flow:cpt-katapult-flow-observability-stream-progress:p1
	mux.Handle("GET /api/v1alpha1/transfers/{id}/progress", auth(viewer(http.HandlerFunc(s.handleStreamProgress))))

	// @cpt-begin:cpt-katapult-flow-observability-scrape-metrics:p2:inst-prom-scrape
	// Prometheus metrics (no auth, cluster-internal)
	if s.metricsHandler != nil {
		mux.Handle("GET /metrics", s.metricsHandler)
	}
	// @cpt-end:cpt-katapult-flow-observability-scrape-metrics:p2:inst-prom-scrape

	return mux
}
