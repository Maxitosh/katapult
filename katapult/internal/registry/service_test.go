package registry

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/testutil"
	"github.com/google/uuid"
)

func TestRegisterAgent_NewAgent(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())

	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}
	pvcs := []domain.PVCInfo{{PVCName: "ns/pvc1", SizeBytes: 1024, StorageClass: "local", NodeAffinity: "node1"}}

	id, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, pvcs, "katapult-ns")
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
	if agent.State != domain.AgentStateHealthy {
		t.Fatalf("new agent state should be healthy, got %q", agent.State)
	}
	if len(agent.PVCs) != 1 {
		t.Fatalf("expected 1 PVC, got %d", len(agent.PVCs))
	}
	if agent.JWTNamespace != "katapult-ns" {
		t.Fatalf("expected jwt_namespace %q, got %q", "katapult-ns", agent.JWTNamespace)
	}
}

func TestRegisterAgent_ReRegistration(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	id1, _ := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "katapult-ns")
	id2, _ := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "katapult-ns")

	if id1 != id2 {
		t.Fatalf("re-registration should return same ID, got %s and %s", id1, id2)
	}
}

func TestRegisterAgent_InvalidTools(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.20", Zstd: "1.5.5", Stunnel: "5.72"}

	_, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "katapult-ns")
	if err == nil {
		t.Fatal("expected error for invalid tools")
	}
}

func TestRegisterAgent_JWTNamespaceMismatch(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	_, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "namespace-a")
	if err != nil {
		t.Fatalf("unexpected error on first registration: %v", err)
	}

	_, err = svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "namespace-b")
	if err == nil {
		t.Fatal("expected error for JWT namespace mismatch")
	}
}

func TestRegisterAgent_JWTNamespaceConsistent(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	id1, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "namespace-a")
	if err != nil {
		t.Fatalf("unexpected error on first registration: %v", err)
	}

	id2, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "namespace-a")
	if err != nil {
		t.Fatalf("unexpected error on re-registration with same namespace: %v", err)
	}

	if id1 != id2 {
		t.Fatalf("re-registration should return same ID, got %s and %s", id1, id2)
	}
}

func TestHeartbeat_UpdatesPVCs(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	id, _ := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "katapult-ns")

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
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())

	err := svc.Heartbeat(context.Background(), uuid.New(), nil)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestHealthEvaluator_MarksUnhealthy(t *testing.T) {
	repo := testutil.NewMemRepo()
	logger := slog.Default()

	oldAgent := &domain.Agent{
		ID:            uuid.New(),
		ClusterID:     "cluster-a",
		NodeName:      "node-1",
		State:         domain.AgentStateHealthy,
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
	if agent.State != domain.AgentStateUnhealthy {
		t.Fatalf("agent state should be unhealthy, got %q", agent.State)
	}
}

func TestHealthEvaluator_MarksDisconnected(t *testing.T) {
	repo := testutil.NewMemRepo()
	logger := slog.Default()

	// Seed an unhealthy agent with a 10-minute-old heartbeat.
	oldAgent := &domain.Agent{
		ID:            uuid.New(),
		ClusterID:     "cluster-b",
		NodeName:      "node-2",
		State:         domain.AgentStateUnhealthy,
		Healthy:       false,
		LastHeartbeat: time.Now().Add(-10 * time.Minute),
		Tools:         domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"},
		RegisteredAt:  time.Now().Add(-15 * time.Minute),
	}
	_ = repo.UpsertAgent(context.Background(), oldAgent)

	eval := NewHealthEvaluator(repo, 90*time.Second, 5*time.Minute, logger)
	_, disconnected, err := eval.Evaluate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if disconnected != 1 {
		t.Fatalf("expected 1 disconnected, got %d", disconnected)
	}

	agent, _ := repo.GetAgentByID(context.Background(), oldAgent.ID)
	if agent.State != domain.AgentStateDisconnected {
		t.Fatalf("agent state should be disconnected, got %q", agent.State)
	}
}

func TestHeartbeat_RecoveryFromUnhealthy(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	id, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "katapult-ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually set agent to unhealthy.
	repo.Agents[id].State = domain.AgentStateUnhealthy
	repo.Agents[id].Healthy = false

	// Heartbeat should recover the agent.
	if err := svc.Heartbeat(context.Background(), id, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent, _ := repo.GetAgentByID(context.Background(), id)
	if agent.State != domain.AgentStateHealthy {
		t.Fatalf("expected healthy after recovery, got %q", agent.State)
	}
	if !agent.Healthy {
		t.Fatal("expected healthy flag to be true after recovery")
	}
}

func TestHeartbeat_DisconnectedAgentRejected(t *testing.T) {
	repo := testutil.NewMemRepo()
	svc := NewService(repo, slog.Default())
	tools := domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"}

	id, err := svc.RegisterAgent(context.Background(), "cluster-a", "node-1", tools, nil, "katapult-ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually set agent to disconnected.
	repo.Agents[id].State = domain.AgentStateDisconnected
	repo.Agents[id].Healthy = false

	// Heartbeat from disconnected agent should be rejected.
	err = svc.Heartbeat(context.Background(), id, nil)
	if err == nil {
		t.Fatal("expected error for disconnected agent heartbeat")
	}
}
