package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/observability"
	"github.com/maxitosh/katapult/internal/registry"
	"github.com/maxitosh/katapult/internal/testutil"
	"github.com/maxitosh/katapult/internal/transfer"
)

func setupTestServer(t *testing.T) (*httptest.Server, *testutil.MemTransferRepo, *testutil.MemRepo) {
	t.Helper()

	transferRepo := testutil.NewMemTransferRepo()
	agentRepo := testutil.NewMemRepo()

	logger := slog.Default()

	commander := transfer.NoopCommander{}
	credManager := transfer.NoopCredentialManager{}
	s3Client := transfer.NoopS3Client{}
	pvcFinder := transfer.NoopPVCFinder{}

	validator := transfer.NewValidator(pvcFinder, commander, logger)
	cleaner := transfer.NewCleaner(credManager, commander, s3Client, logger)
	orchestrator := transfer.NewOrchestrator(
		transferRepo, validator, cleaner, commander, credManager,
		transfer.S3Config{}, domain.DefaultTransferConfig(), logger,
	)
	registrySvc := registry.NewService(agentRepo, logger)

	tokens := map[string]UserInfo{
		"op-token":     {Subject: "operator-user", Role: RoleOperator},
		"viewer-token": {Subject: "viewer-user", Role: RoleViewer},
	}
	tokenValidator := NewStaticTokenValidator(tokens)
	srv := NewServer(orchestrator, registrySvc, logger)
	ts := httptest.NewServer(srv.Handler(tokenValidator))
	t.Cleanup(ts.Close)

	return ts, transferRepo, agentRepo
}

func TestAuthRequired(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1alpha1/transfers")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestInvalidToken(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/transfers", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestViewerCannotCreate(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	body := `{"source_cluster":"c1","source_pvc":"pvc1","destination_cluster":"c2","destination_pvc":"pvc2"}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1alpha1/transfers", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer viewer-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListTransfers(t *testing.T) {
	ts, repo, _ := setupTestServer(t)

	// Seed a transfer directly in the repo.
	id := uuid.New()
	_ = repo.CreateTransfer(context.Background(), &domain.Transfer{
		ID:                 id,
		SourceCluster:      "cluster-a",
		SourcePVC:          "pvc-1",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "pvc-2",
		State:              domain.TransferStatePending,
		CreatedAt:          time.Now(),
	})

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/transfers", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Items      []json.RawMessage `json:"items"`
		Pagination paginationMeta    `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 transfer, got %d", len(result.Items))
	}
	if result.Pagination.Total != 1 {
		t.Errorf("expected total 1, got %d", result.Pagination.Total)
	}
}

func TestGetTransferNotFound(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/transfers/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListAgents(t *testing.T) {
	ts, _, agentRepo := setupTestServer(t)

	// Seed an agent.
	agentID := uuid.New()
	_ = agentRepo.UpsertAgent(context.Background(), &domain.Agent{
		ID:            agentID,
		ClusterID:     "cluster-x",
		NodeName:      "node-1",
		State:         domain.AgentStateHealthy,
		Healthy:       true,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
		PVCs: []domain.PVCInfo{
			{PVCName: "data-0", SizeBytes: 1024, StorageClass: "standard"},
		},
	})

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/agents", nil)
	req.Header.Set("Authorization", "Bearer op-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 agent, got %d", len(result.Items))
	}
}

func TestGetAgentPVCs(t *testing.T) {
	ts, _, agentRepo := setupTestServer(t)

	agentID := uuid.New()
	_ = agentRepo.UpsertAgent(context.Background(), &domain.Agent{
		ID:            agentID,
		ClusterID:     "cluster-x",
		NodeName:      "node-1",
		State:         domain.AgentStateHealthy,
		Healthy:       true,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
		PVCs: []domain.PVCInfo{
			{PVCName: "data-0", SizeBytes: 1024, StorageClass: "standard"},
			{PVCName: "data-1", SizeBytes: 2048, StorageClass: "premium"},
		},
	})

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/agents/"+agentID.String()+"/pvcs", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		PVCs []json.RawMessage `json:"pvcs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.PVCs) != 2 {
		t.Errorf("expected 2 PVCs, got %d", len(result.PVCs))
	}
}

func TestListClusters(t *testing.T) {
	ts, _, agentRepo := setupTestServer(t)

	for _, a := range []struct{ cluster, node string }{
		{"cluster-a", "node-1"},
		{"cluster-b", "node-2"},
		{"cluster-a", "node-3"},
	} {
		_ = agentRepo.UpsertAgent(context.Background(), &domain.Agent{
			ID:            uuid.New(),
			ClusterID:     a.cluster,
			NodeName:      a.node,
			State:         domain.AgentStateHealthy,
			Healthy:       true,
			LastHeartbeat: time.Now(),
			RegisteredAt:  time.Now(),
		})
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/clusters", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Clusters []string `json:"clusters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(result.Clusters))
	}
}

func TestCancelTransferNotFound(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1alpha1/transfers/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer op-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCancelTerminalTransfer(t *testing.T) {
	ts, repo, _ := setupTestServer(t)

	id := uuid.New()
	_ = repo.CreateTransfer(context.Background(), &domain.Transfer{
		ID:                 id,
		SourceCluster:      "c1",
		SourcePVC:          "p1",
		DestinationCluster: "c2",
		DestinationPVC:     "p2",
		State:              domain.TransferStateCompleted,
		CreatedAt:          time.Now(),
	})

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1alpha1/transfers/"+id.String(), nil)
	req.Header.Set("Authorization", "Bearer op-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}

func setupTestServerWithHub(t *testing.T) (*httptest.Server, *testutil.MemTransferRepo, *testutil.MemRepo) {
	t.Helper()
	ts, repo, _, agentRepo := setupTestServerWithHubRepo(t)
	return ts, repo, agentRepo
}

func setupTestServerWithHubRepo(t *testing.T) (*httptest.Server, *testutil.MemTransferRepo, *observability.ProgressHub, *testutil.MemRepo) {
	t.Helper()

	transferRepo := testutil.NewMemTransferRepo()
	agentRepo := testutil.NewMemRepo()
	hub := observability.NewProgressHub()

	logger := slog.Default()

	commander := transfer.NoopCommander{}
	credManager := transfer.NoopCredentialManager{}
	s3Client := transfer.NoopS3Client{}
	pvcFinder := transfer.NoopPVCFinder{}

	validator := transfer.NewValidator(pvcFinder, commander, logger)
	cleaner := transfer.NewCleaner(credManager, commander, s3Client, logger)
	orchestrator := transfer.NewOrchestrator(
		transferRepo, validator, cleaner, commander, credManager,
		transfer.S3Config{}, domain.DefaultTransferConfig(), logger,
		transfer.WithProgressHub(hub),
	)
	registrySvc := registry.NewService(agentRepo, logger)

	tokens := map[string]UserInfo{
		"op-token":     {Subject: "operator-user", Role: RoleOperator},
		"viewer-token": {Subject: "viewer-user", Role: RoleViewer},
	}
	tokenValidator := NewStaticTokenValidator(tokens)
	srv := NewServer(orchestrator, registrySvc, logger, WithProgressHub(hub))
	ts := httptest.NewServer(srv.Handler(tokenValidator))
	t.Cleanup(ts.Close)

	return ts, transferRepo, hub, agentRepo
}

func TestGetTransferEventsNotFound(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/transfers/"+uuid.New().String()+"/events", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestValidationErrors(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	body := `{"source_cluster":"","source_pvc":"","destination_cluster":"","destination_pvc":""}`
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1alpha1/transfers", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer op-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	var result ValidationErrors
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(result.Errors) < 4 {
		t.Errorf("expected at least 4 validation errors, got %d", len(result.Errors))
	}
}
