package registry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/maxitosh/katapult/internal/domain"
	"github.com/google/uuid"
)

// Service implements the Agent Registry domain logic.
type Service struct {
	repo   AgentRepository
	logger *slog.Logger
}

// NewService creates a new Agent Registry service.
func NewService(repo AgentRepository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// RegisterAgent validates and registers an agent, returning the assigned agent ID.
// jwtNamespace is the Kubernetes namespace extracted from the agent's JWT token,
// binding the agent identity to a specific namespace.
// @cpt-flow:cpt-katapult-flow-agent-system-register:p1
// @cpt-dod:cpt-katapult-dod-agent-system-registration:p1
func (s *Service) RegisterAgent(ctx context.Context, clusterID, nodeName string, tools domain.ToolVersions, pvcs []domain.PVCInfo, jwtNamespace string) (uuid.UUID, error) {
	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-validate-reg
	if err := ValidateTools(tools); err != nil {
		return uuid.Nil, fmt.Errorf("registration rejected: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-validate-reg

	// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-db-check-existing
	existing, err := s.repo.GetAgentByClusterAndNode(ctx, clusterID, nodeName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("checking existing agent: %w", err)
	}
	// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-db-check-existing

	var agentID uuid.UUID
	var registeredAt time.Time

	// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-handle-reregister
	if existing != nil {
		if existing.JWTNamespace != "" && existing.JWTNamespace != jwtNamespace {
			return uuid.Nil, fmt.Errorf("registration rejected: JWT namespace mismatch (expected %q, got %q)", existing.JWTNamespace, jwtNamespace)
		}
		agentID = existing.ID
		registeredAt = existing.RegisteredAt
		s.logger.Info("re-registering existing agent", "agent_id", agentID, "cluster", clusterID, "node", nodeName)
	} else {
		// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-generate-id
		agentID = uuid.New()
		// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-generate-id
		registeredAt = time.Now()
		s.logger.Info("registering new agent", "agent_id", agentID, "cluster", clusterID, "node", nodeName)
	}
	// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-handle-reregister

	agent := &domain.Agent{
		ID:            agentID,
		ClusterID:     clusterID,
		NodeName:      nodeName,
		State:         domain.AgentStateRegistering,
		Healthy:       false,
		LastHeartbeat: time.Now(),
		Tools:         tools,
		RegisteredAt:  registeredAt,
		PVCs:          pvcs,
		JWTNamespace:  jwtNamespace,
	}

	if err := agent.TransitionTo(domain.AgentStateHealthy); err != nil {
		return uuid.Nil, fmt.Errorf("state transition failed: %w", err)
	}

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-db-persist-registration
	if err := s.repo.UpsertAgent(ctx, agent); err != nil {
		return uuid.Nil, fmt.Errorf("persisting agent: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-db-persist-registration

	// @cpt-begin:cpt-katapult-algo-agent-system-validate-registration:p1:inst-return-success
	return agentID, nil
	// @cpt-end:cpt-katapult-algo-agent-system-validate-registration:p1:inst-return-success
}

// GetAgent retrieves an agent by its ID. Returns (nil, nil) when not found.
func (s *Service) GetAgent(ctx context.Context, agentID uuid.UUID) (*domain.Agent, error) {
	return s.repo.GetAgentByID(ctx, agentID)
}

// ListAgents returns a filtered list of agents with total count.
// @cpt-flow:cpt-katapult-flow-api-cli-list-agents:p1
// @cpt-dod:cpt-katapult-dod-api-cli-rest-agent-endpoints:p1
func (s *Service) ListAgents(ctx context.Context, filter domain.AgentFilter) ([]domain.Agent, int, error) {
	// @cpt-begin:cpt-katapult-flow-api-cli-list-agents:p1:inst-delegate-agents
	return s.repo.ListAgents(ctx, filter)
	// @cpt-end:cpt-katapult-flow-api-cli-list-agents:p1:inst-delegate-agents
}

// ListClusters returns all distinct cluster IDs.
// @cpt-dod:cpt-katapult-dod-api-cli-rest-agent-endpoints:p1
func (s *Service) ListClusters(ctx context.Context) ([]string, error) {
	return s.repo.ListClusters(ctx)
}

// Heartbeat processes a heartbeat from an agent, updating health and PVC inventory.
// @cpt-flow:cpt-katapult-flow-agent-system-heartbeat:p1
// @cpt-dod:cpt-katapult-dod-agent-system-heartbeat:p1
func (s *Service) Heartbeat(ctx context.Context, agentID uuid.UUID, pvcs []domain.PVCInfo) error {
	agent, err := s.repo.GetAgentByID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("looking up agent: %w", err)
	}
	if agent == nil {
		return fmt.Errorf("agent %s not found", agentID)
	}

	if agent.State == domain.AgentStateDisconnected {
		return fmt.Errorf("agent %s is disconnected: must re-register", agentID)
	}

	if agent.State == domain.AgentStateUnhealthy {
		if err := agent.TransitionTo(domain.AgentStateHealthy); err != nil {
			return fmt.Errorf("recovery transition failed: %w", err)
		}
		s.logger.Info("agent recovered from unhealthy", "agent_id", agentID)
	}

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-db-update-heartbeat
	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-db-replace-pvcs-heartbeat
	if err := s.repo.UpdateHeartbeat(ctx, agentID, pvcs); err != nil {
		return fmt.Errorf("updating heartbeat: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-db-replace-pvcs-heartbeat
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-db-update-heartbeat

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-return-ack
	return nil
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-return-ack
}
