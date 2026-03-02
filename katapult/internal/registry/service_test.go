package registry

import (
	"context"
	"log/slog"
	"testing"
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
	// Check for existing by cluster+node and remove old entry if ID changed.
	for id, a := range m.agents {
		if a.ClusterID == agent.ClusterID && a.NodeName == agent.NodeName && id != agent.ID {
			delete(m.agents, id)
		}
	}
	// Deep copy PVCs.
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

func TestRegisterAgent_NewAgent(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, slog.Default())

	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}
	pvcs := []domain.PVCInfo{{PVCName: "ns/pvc1", SizeBytes: 1024, StorageClass: "local", NodeAffinity: "node1"}}

	id, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, pvcs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected non-nil agent ID")
	}

	agent, _ := repo.GetAgentByID(context.Background(), id)
	if agent == nil {
		t.Fatal("agent not found in repo")
	}
	if agent.ClusterID != "cluster-a" || agent.NodeName != "node-1" {
		t.Fatal("agent identity mismatch")
	}
	if !agent.Healthy {
		t.Fatal("new agent should be healthy")
	}
	if len(agent.PVCs) != 1 {
		t.Fatalf("expected 1 PVC, got %d", len(agent.PVCs))
	}
}

func TestRegisterAgent_ReRegistration(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	id1, _ := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil)
	id2, _ := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil)

	if id1 != id2 {
		t.Fatalf("re-registration should return same ID, got %s and %s", id1, id2)
	}
}

func TestRegisterAgent_InvalidTools(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.20", Zstd: "1.5.5", Stunnel: "5.72"}

	_, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil)
	if err == nil {
		t.Fatal("expected error for invalid tools")
	}
}

func TestHeartbeat_UpdatesPVCs(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	id, _ := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil)

	newPVCs := []domain.PVCInfo{
		{PVCName: "ns/pvc1", SizeBytes: 2048, StorageClass: "local", NodeAffinity: "node-1"},
		{PVCName: "ns/pvc2", SizeBytes: 4096, StorageClass: "local", NodeAffinity: "node-1"},
	}
	if err := svc.Heartbeat(context.Background(), id, newPVCs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, _ := repo.GetAgentByID(context.Background(), id)
	if len(agent.PVCs) != 2 {
		t.Fatalf("expected 2 PVCs after heartbeat, got %d", len(agent.PVCs))
	}
}

func TestHeartbeat_UnknownAgent(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, slog.Default())

	err := svc.Heartbeat(context.Background(), uuid.New(), nil)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestHealthEvaluator_MarksUnhealthy(t *testing.T) {
	repo := newMemRepo()
	logger := slog.Default()

	// Seed an agent with old heartbeat.
	oldAgent := &domain.Agent{
		ID:            uuid.New(),
		ClusterID:     "cluster-a",
		NodeName:      "node-1",
		Healthy:       true,
		LastHeartbeat: time.Now().Add(-2 * time.Minute),
		Tools:         domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"},
		RegisteredAt:  time.Now().Add(-5 * time.Minute),
	}
	_ = repo.UpsertAgent(context.Background(), oldAgent)

	eval := NewHealthEvaluator(repo, 90*time.Second, 5*time.Minute, logger)
	unhealthy, _, err := eval.Evaluate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unhealthy != 1 {
		t.Fatalf("expected 1 unhealthy, got %d", unhealthy)
	}

	agent, _ := repo.GetAgentByID(context.Background(), oldAgent.ID)
	if agent.Healthy {
		t.Fatal("agent should be unhealthy after evaluation")
	}
}
