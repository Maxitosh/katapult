package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/maxitosh/katapult/internal/domain"
	"github.com/google/uuid"
)

// MemRepo is an in-memory AgentRepository for unit testing.
type MemRepo struct {
	mu     sync.Mutex
	Agents map[uuid.UUID]*domain.Agent
}

// NewMemRepo creates a new in-memory repository.
func NewMemRepo() *MemRepo {
	return &MemRepo{Agents: make(map[uuid.UUID]*domain.Agent)}
}

func (m *MemRepo) UpsertAgent(_ context.Context, agent *domain.Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, a := range m.Agents {
		if a.ClusterID == agent.ClusterID && a.NodeName == agent.NodeName && id != agent.ID {
			delete(m.Agents, id)
		}
	}
	pvcs := make([]domain.PVCInfo, len(agent.PVCs))
	copy(pvcs, agent.PVCs)
	stored := *agent
	stored.PVCs = pvcs
	m.Agents[agent.ID] = &stored
	return nil
}

func (m *MemRepo) GetAgentByID(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.Agents[id]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (m *MemRepo) GetAgentByClusterAndNode(_ context.Context, clusterID, nodeName string) (*domain.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.Agents {
		if a.ClusterID == clusterID && a.NodeName == nodeName {
			cp := *a
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *MemRepo) UpdateHeartbeat(_ context.Context, agentID uuid.UUID, pvcs []domain.PVCInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.Agents[agentID]
	if !ok {
		return nil
	}
	a.State = domain.AgentStateHealthy
	a.Healthy = true
	a.LastHeartbeat = time.Now()
	a.PVCs = make([]domain.PVCInfo, len(pvcs))
	copy(a.PVCs, pvcs)
	return nil
}

func (m *MemRepo) MarkUnhealthy(_ context.Context, cutoff time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, a := range m.Agents {
		if a.State == domain.AgentStateHealthy && a.LastHeartbeat.Before(cutoff) {
			a.State = domain.AgentStateUnhealthy
			a.Healthy = false
			count++
		}
	}
	return count, nil
}

func (m *MemRepo) MarkDisconnected(_ context.Context, cutoff time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, a := range m.Agents {
		if a.State == domain.AgentStateUnhealthy && a.LastHeartbeat.Before(cutoff) {
			a.State = domain.AgentStateDisconnected
			a.Healthy = false
			count++
		}
	}
	return count, nil
}
