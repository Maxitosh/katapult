package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/transfer"
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
	orchestrator TransferService
	registrySvc  RegistryService
	logger       *slog.Logger
}

// NewServer creates a new HTTP API server.
func NewServer(orchestrator TransferService, registrySvc RegistryService, logger *slog.Logger) *Server {
	return &Server{
		orchestrator: orchestrator,
		registrySvc:  registrySvc,
		logger:       logger,
	}
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

	return mux
}
