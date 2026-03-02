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
// Implements cpt-katapult-algo-agent-system-validate-registration.
func (s *Service) RegisterAgent(ctx context.Context, clusterID, nodeName string, tools domain.ToolVersions, pvcs []domain.PVCInfo) (uuid.UUID, error) {
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
		State:         domain.AgentStateHealthy,
		Healthy:       true,
		LastHeartbeat: time.Now(),
		Tools:         tools,
		RegisteredAt:  registeredAt,
		PVCs:          pvcs,
	}

	if err := s.repo.UpsertAgent(ctx, agent); err != nil {
		return uuid.Nil, fmt.Errorf("persisting agent: %w", err)
	}

	return agentID, nil
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

	if err := s.repo.UpdateHeartbeat(ctx, agentID, pvcs); err != nil {
		return fmt.Errorf("updating heartbeat: %w", err)
	}

	return nil
}
