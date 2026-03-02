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
// Implements cpt-katapult-algo-agent-system-validate-registration.
func (s *Service) RegisterAgent(ctx context.Context, clusterID, nodeName string, tools domain.ToolVersions, pvcs []domain.PVCInfo, jwtNamespace string) (uuid.UUID, error) {
	if err := ValidateTools(tools); err != nil {
		return uuid.Nil, fmt.Errorf("registration rejected: %w", err)
	}

	existing, err := s.repo.GetAgentByClusterAndNode(ctx, clusterID, nodeName)
	if err != nil {
		return uuid.Nil, fmt.Errorf("checking existing agent: %w", err)
	}

	var agentID uuid.UUID
	var registeredAt time.Time

	if existing != nil {
		if existing.JWTNamespace != "" && existing.JWTNamespace != jwtNamespace {
			return uuid.Nil, fmt.Errorf("registration rejected: JWT namespace mismatch (expected %q, got %q)", existing.JWTNamespace, jwtNamespace)
		}
		agentID = existing.ID
		registeredAt = existing.RegisteredAt
		s.logger.Info("re-registering existing agent", "agent_id", agentID, "cluster", clusterID, "node", nodeName)
	} else {
		agentID = uuid.New()
		registeredAt = time.Now()
		s.logger.Info("registering new agent", "agent_id", agentID, "cluster", clusterID, "node", nodeName)
	}

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

	if err := s.repo.UpsertAgent(ctx, agent); err != nil {
		return uuid.Nil, fmt.Errorf("persisting agent: %w", err)
	}

	return agentID, nil
}

// GetAgent retrieves an agent by its ID. Returns (nil, nil) when not found.
func (s *Service) GetAgent(ctx context.Context, agentID uuid.UUID) (*domain.Agent, error) {
	return s.repo.GetAgentByID(ctx, agentID)
}

// Heartbeat processes a heartbeat from an agent, updating health and PVC inventory.
// Implements cpt-katapult-flow-agent-system-heartbeat.
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

	if err := s.repo.UpdateHeartbeat(ctx, agentID, pvcs); err != nil {
		return fmt.Errorf("updating heartbeat: %w", err)
	}

	return nil
}
