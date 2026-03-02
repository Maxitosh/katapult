package grpc

import (
	"context"
	"time"

	"github.com/chainstack/katapult/internal/domain"
	"github.com/google/uuid"
)

// memRepo is an in-memory AgentRepository for unit testing.
type memRepo struct {
	agents map[uuid.UUID]*domain.Agent
}

func newMemRepo() *memRepo {
	return &memRepo{agents: make(map[uuid.UUID]*domain.Agent)}
}

func (m *memRepo) UpsertAgent(_ context.Context, agent *domain.Agent) error {
	for id, a := range m.agents {
		if a.ClusterID == agent.ClusterID && a.NodeName == agent.NodeName && id != agent.ID {
			delete(m.agents, id)
		}
	}
	pvcs := make([]domain.PVCInfo, len(agent.PVCs))
	copy(pvcs, agent.PVCs)
	stored := *agent
	stored.PVCs = pvcs
	m.agents[agent.ID] = &stored
	return nil
}

func (m *memRepo) GetAgentByID(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
	a, ok := m.agents[id]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (m *memRepo) GetAgentByClusterAndNode(_ context.Context, clusterID, nodeName string) (*domain.Agent, error) {
	for _, a := range m.agents {
		if a.ClusterID == clusterID && a.NodeName == nodeName {
			cp := *a
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *memRepo) UpdateHeartbeat(_ context.Context, agentID uuid.UUID, pvcs []domain.PVCInfo) error {
	a, ok := m.agents[agentID]
	if !ok {
		return nil
	}
	a.Healthy = true
	a.LastHeartbeat = time.Now()
	a.PVCs = make([]domain.PVCInfo, len(pvcs))
	copy(a.PVCs, pvcs)
	return nil
}

func (m *memRepo) MarkUnhealthy(_ context.Context, cutoff time.Time) (int, error) {
	count := 0
	for _, a := range m.agents {
		if a.Healthy && a.LastHeartbeat.Before(cutoff) {
			a.Healthy = false
			count++
		}
	}
	return count, nil
}

func (m *memRepo) MarkDisconnected(_ context.Context, _ time.Time) (int, error) {
	return 0, nil
}
