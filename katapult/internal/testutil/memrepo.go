package testutil

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
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

func (m *MemRepo) ListAgents(_ context.Context, filter domain.AgentFilter) ([]domain.Agent, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var all []domain.Agent
	for _, a := range m.Agents {
		if filter.ClusterID != nil && a.ClusterID != *filter.ClusterID {
			continue
		}
		if filter.State != nil && a.State != *filter.State {
			continue
		}
		cp := *a
		all = append(all, cp)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].ClusterID != all[j].ClusterID {
			return all[i].ClusterID < all[j].ClusterID
		}
		return all[i].NodeName < all[j].NodeName
	})

	total := len(all)
	if filter.Offset > 0 && filter.Offset < len(all) {
		all = all[filter.Offset:]
	} else if filter.Offset >= len(all) {
		all = nil
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit < len(all) {
		all = all[:limit]
	}

	return all, total, nil
}

func (m *MemRepo) ListClusters(_ context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]bool)
	for _, a := range m.Agents {
		seen[a.ClusterID] = true
	}

	var clusters []string
	for c := range seen {
		clusters = append(clusters, c)
	}
	sort.Strings(clusters)
	return clusters, nil
}
