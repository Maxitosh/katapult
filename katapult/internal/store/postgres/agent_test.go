//go:build integration

package postgres

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/testutil"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, cleanup := testutil.SetupPostgres(t, filepath.Join("migrations"))
	t.Cleanup(cleanup)
	return pool
}

func newTestAgent(clusterID, nodeName, jwtNamespace string) *domain.Agent {
	return &domain.Agent{
		ID:            uuid.New(),
		ClusterID:     clusterID,
		NodeName:      nodeName,
		State:         domain.AgentStateHealthy,
		Healthy:       true,
		LastHeartbeat: time.Now(),
		Tools:         domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"},
		RegisteredAt:  time.Now(),
		JWTNamespace:  jwtNamespace,
		PVCs: []domain.PVCInfo{
			{PVCName: "ns/pvc1", SizeBytes: 1024, StorageClass: "local", NodeAffinity: nodeName},
		},
	}
}

func TestUpsertAgent_InsertNew(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	agent := newTestAgent("cluster-a", "node-1", "katapult-ns")
	if err := repo.UpsertAgent(ctx, agent); err != nil {
		t.Fatalf("upserting agent: %v", err)
	}

	got, err := repo.GetAgentByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("getting agent: %v", err)
	}
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.ClusterID != "cluster-a" {
		t.Fatalf("expected cluster_id %q, got %q", "cluster-a", got.ClusterID)
	}
	if got.JWTNamespace != "katapult-ns" {
		t.Fatalf("expected jwt_namespace %q, got %q", "katapult-ns", got.JWTNamespace)
	}
	if len(got.PVCs) != 1 {
		t.Fatalf("expected 1 PVC, got %d", len(got.PVCs))
	}
}

func TestUpsertAgent_UpdateOnConflict(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	agent1 := newTestAgent("cluster-a", "node-1", "ns1")
	if err := repo.UpsertAgent(ctx, agent1); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Upsert with same cluster_id/node_name but new ID and namespace.
	agent2 := newTestAgent("cluster-a", "node-1", "ns2")
	if err := repo.UpsertAgent(ctx, agent2); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := repo.GetAgentByClusterAndNode(ctx, "cluster-a", "node-1")
	if err != nil {
		t.Fatalf("getting agent: %v", err)
	}
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	// The ON CONFLICT updates the ID and jwt_namespace.
	if got.ID != agent2.ID {
		t.Fatalf("expected ID %s, got %s", agent2.ID, got.ID)
	}
	if got.JWTNamespace != "ns2" {
		t.Fatalf("expected jwt_namespace %q, got %q", "ns2", got.JWTNamespace)
	}
}

func TestUpdateHeartbeat_ReplacesPVCs(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	agent := newTestAgent("cluster-a", "node-1", "ns")
	if err := repo.UpsertAgent(ctx, agent); err != nil {
		t.Fatalf("upserting agent: %v", err)
	}

	newPVCs := []domain.PVCInfo{
		{PVCName: "ns/pvc-new-1", SizeBytes: 2048, StorageClass: "ssd", NodeAffinity: "node-1"},
		{PVCName: "ns/pvc-new-2", SizeBytes: 4096, StorageClass: "ssd", NodeAffinity: "node-1"},
	}
	if err := repo.UpdateHeartbeat(ctx, agent.ID, newPVCs); err != nil {
		t.Fatalf("updating heartbeat: %v", err)
	}

	got, err := repo.GetAgentByID(ctx, agent.ID)
	if err != nil {
		t.Fatalf("getting agent: %v", err)
	}
	if len(got.PVCs) != 2 {
		t.Fatalf("expected 2 PVCs, got %d", len(got.PVCs))
	}
	if !got.Healthy {
		t.Fatal("expected agent to be healthy after heartbeat")
	}
}

func TestMarkUnhealthy_OnlyAffectsHealthyStale(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Stale healthy agent.
	stale := newTestAgent("cluster-a", "node-stale", "ns")
	stale.LastHeartbeat = time.Now().Add(-5 * time.Minute)
	if err := repo.UpsertAgent(ctx, stale); err != nil {
		t.Fatalf("upserting stale agent: %v", err)
	}

	// Fresh healthy agent.
	fresh := newTestAgent("cluster-a", "node-fresh", "ns")
	if err := repo.UpsertAgent(ctx, fresh); err != nil {
		t.Fatalf("upserting fresh agent: %v", err)
	}

	cutoff := time.Now().Add(-2 * time.Minute)
	count, err := repo.MarkUnhealthy(ctx, cutoff)
	if err != nil {
		t.Fatalf("marking unhealthy: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unhealthy, got %d", count)
	}

	// Fresh agent should still be healthy.
	gotFresh, _ := repo.GetAgentByID(ctx, fresh.ID)
	if gotFresh.State != domain.AgentStateHealthy {
		t.Fatalf("fresh agent should still be healthy, got %q", gotFresh.State)
	}
}

func TestMarkDisconnected_OnlyAffectsUnhealthyStale(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Insert an unhealthy agent with old heartbeat.
	agent := newTestAgent("cluster-a", "node-1", "ns")
	agent.State = domain.AgentStateUnhealthy
	agent.Healthy = false
	agent.LastHeartbeat = time.Now().Add(-10 * time.Minute)
	if err := repo.UpsertAgent(ctx, agent); err != nil {
		t.Fatalf("upserting agent: %v", err)
	}

	// Insert a healthy agent (should not be affected).
	healthy := newTestAgent("cluster-a", "node-2", "ns")
	if err := repo.UpsertAgent(ctx, healthy); err != nil {
		t.Fatalf("upserting healthy agent: %v", err)
	}

	cutoff := time.Now().Add(-5 * time.Minute)
	count, err := repo.MarkDisconnected(ctx, cutoff)
	if err != nil {
		t.Fatalf("marking disconnected: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 disconnected, got %d", count)
	}

	got, _ := repo.GetAgentByID(ctx, agent.ID)
	if got.State != domain.AgentStateDisconnected {
		t.Fatalf("expected disconnected, got %q", got.State)
	}

	gotHealthy, _ := repo.GetAgentByID(ctx, healthy.ID)
	if gotHealthy.State != domain.AgentStateHealthy {
		t.Fatalf("healthy agent should not be affected, got %q", gotHealthy.State)
	}
}

func TestGetAgentByClusterAndNode_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	got, err := repo.GetAgentByClusterAndNode(ctx, "nonexistent", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent agent")
	}
}

func TestGetAgentByClusterAndNode_Found(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	agent := newTestAgent("cluster-b", "node-2", "ns")
	if err := repo.UpsertAgent(ctx, agent); err != nil {
		t.Fatalf("upserting agent: %v", err)
	}

	got, err := repo.GetAgentByClusterAndNode(ctx, "cluster-b", "node-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.ID != agent.ID {
		t.Fatalf("expected ID %s, got %s", agent.ID, got.ID)
	}
}

func TestGetAgentByID_NotFound(t *testing.T) {
	pool := setupTestDB(t)
	repo := NewAgentRepository(pool)
	ctx := context.Background()

	got, err := repo.GetAgentByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent agent")
	}
}
