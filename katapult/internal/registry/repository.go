package registry

import (
	"context"
	"time"

	"github.com/chainstack/katapult/internal/domain"
	"github.com/google/uuid"
)

// AgentRepository defines the persistence interface for agent data.
type AgentRepository interface {
	// UpsertAgent creates or updates an agent record and replaces its PVC inventory.
	UpsertAgent(ctx context.Context, agent *domain.Agent) error

	// GetAgentByID retrieves an agent by its UUID.
	GetAgentByID(ctx context.Context, id uuid.UUID) (*domain.Agent, error)

	// GetAgentByClusterAndNode retrieves an agent by cluster ID and node name.
	GetAgentByClusterAndNode(ctx context.Context, clusterID, nodeName string) (*domain.Agent, error)

	// UpdateHeartbeat updates the agent's health status and replaces its PVC inventory.
	UpdateHeartbeat(ctx context.Context, agentID uuid.UUID, pvcs []domain.PVCInfo) error

	// MarkUnhealthy marks all agents whose last heartbeat is older than the cutoff as unhealthy.
	// Returns the number of agents marked unhealthy.
	MarkUnhealthy(ctx context.Context, cutoff time.Time) (int, error)

	// MarkDisconnected marks all unhealthy agents whose last heartbeat is older than the cutoff as disconnected.
	// Returns the number of agents marked disconnected.
	MarkDisconnected(ctx context.Context, cutoff time.Time) (int, error)
}
