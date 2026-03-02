package registry

import (
	"context"
	"time"

	"github.com/maxitosh/katapult/internal/domain"
	"github.com/google/uuid"
)

// AgentRepository defines the persistence interface for agent data.
//
// Callers are responsible for validating domain state transitions before calling
// mutation methods. The repository layer performs raw persistence operations and
// does not enforce state-machine invariants (e.g., that only healthy agents can
// become unhealthy). Use [domain.Agent.TransitionTo] to validate transitions
// before persisting.
type AgentRepository interface {
	// UpsertAgent creates or updates an agent record and replaces its PVC inventory.
	UpsertAgent(ctx context.Context, agent *domain.Agent) error

	// GetAgentByID retrieves an agent by its UUID.
	// Returns (nil, nil) when no agent matches.
	GetAgentByID(ctx context.Context, id uuid.UUID) (*domain.Agent, error)

	// GetAgentByClusterAndNode retrieves an agent by cluster ID and node name.
	// Returns (nil, nil) when no agent matches.
	GetAgentByClusterAndNode(ctx context.Context, clusterID, nodeName string) (*domain.Agent, error)

	// UpdateHeartbeat updates the agent's health status and replaces its PVC inventory.
	//
	// Precondition: the agent must exist and be in a state that allows receiving
	// heartbeats (i.e., not disconnected). The caller must verify state eligibility
	// via the service layer before calling this method.
	UpdateHeartbeat(ctx context.Context, agentID uuid.UUID, pvcs []domain.PVCInfo) error

	// MarkUnhealthy marks all healthy agents whose last heartbeat is older than
	// the cutoff as unhealthy. Only affects agents in the "healthy" state.
	// Returns the number of agents marked unhealthy.
	MarkUnhealthy(ctx context.Context, cutoff time.Time) (int, error)

	// MarkDisconnected marks all unhealthy agents whose last heartbeat is older
	// than the cutoff as disconnected. Only affects agents in the "unhealthy" state.
	// Returns the number of agents marked disconnected.
	MarkDisconnected(ctx context.Context, cutoff time.Time) (int, error)
}
