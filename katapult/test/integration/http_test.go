//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/maxitosh/katapult/internal/domain"
	katapulthttp "github.com/maxitosh/katapult/internal/http"
	"github.com/maxitosh/katapult/internal/registry"
	pgstore "github.com/maxitosh/katapult/internal/store/postgres"
	"github.com/maxitosh/katapult/internal/transfer"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-component-tests:p2
// @cpt-flow:cpt-katapult-flow-integration-tests-run-component-tests:p2

// @cpt-begin:cpt-katapult-dod-integration-tests-component-tests:p2:inst-http-tests

// testTokenValidator is a static token validator for integration tests.
var testTokenValidator = katapulthttp.NewStaticTokenValidator(map[string]katapulthttp.UserInfo{
	"test-operator-token": {Subject: "test-operator", Role: katapulthttp.RoleOperator},
	"test-viewer-token":   {Subject: "test-viewer", Role: katapulthttp.RoleViewer},
})

// startTestHTTPServer wires real repos and returns an httptest.Server.
func startTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	pool := getTestPool(t)
	logger := slog.Default()

	agentRepo := pgstore.NewAgentRepository(pool)
	transferRepo := pgstore.NewTransferRepository(pool)

	registrySvc := registry.NewService(agentRepo, logger)

	cmd := testCommander{}
	validator := transfer.NewValidator(testPVCFinder{}, cmd, logger)
	cleaner := transfer.NewCleaner(transfer.NoopCredentialManager{}, cmd, transfer.NoopS3Client{}, logger)
	orchestrator := transfer.NewOrchestrator(
		transferRepo, validator, cleaner, cmd,
		transfer.NoopCredentialManager{}, transfer.S3Config{},
		domain.DefaultTransferConfig(), logger,
	)

	srv := katapulthttp.NewServer(orchestrator, registrySvc, logger)
	handler := srv.Handler(testTokenValidator)

	return httptest.NewServer(handler)
}

// httpDoJSON makes an authenticated HTTP request and returns the response.
func httpDoJSON(t *testing.T, server *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling body: %v", err)
		}
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req, err := http.NewRequest(method, server.URL+path, reqBody)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-operator-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("executing request: %v", err)
	}
	return resp
}

func TestHTTP_CreateTransfer_ReturnsCreated(t *testing.T) {
	server := startTestHTTPServer(t)
	defer server.Close()

	body := map[string]any{
		"source_cluster":      "http-cluster-a",
		"source_pvc":          "ns/http-src-" + uuid.New().String()[:8],
		"destination_cluster": "http-cluster-a",
		"destination_pvc":     "ns/http-dst-" + uuid.New().String()[:8],
	}
	resp := httpDoJSON(t, server, http.MethodPost, "/api/v1alpha1/transfers", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if result["id"] == nil || result["id"] == "" {
		t.Fatal("expected non-empty id in response")
	}
	if result["state"] != string(domain.TransferStateTransferring) {
		t.Fatalf("expected state %q, got %v", domain.TransferStateTransferring, result["state"])
	}
}

func TestHTTP_ListTransfers_ReturnsAll(t *testing.T) {
	server := startTestHTTPServer(t)
	defer server.Close()

	// Seed a transfer.
	createBody := map[string]any{
		"source_cluster":      "http-cluster-list",
		"source_pvc":          "ns/list-src-" + uuid.New().String()[:8],
		"destination_cluster": "http-cluster-list",
		"destination_pvc":     "ns/list-dst-" + uuid.New().String()[:8],
	}
	createResp := httpDoJSON(t, server, http.MethodPost, "/api/v1alpha1/transfers", createBody)
	createResp.Body.Close()

	// List transfers.
	resp := httpDoJSON(t, server, http.MethodGet, "/api/v1alpha1/transfers", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		Items      []json.RawMessage `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if result.Pagination.Total < 1 {
		t.Fatalf("expected at least 1 transfer in total, got %d", result.Pagination.Total)
	}
}

func TestHTTP_CancelTransfer_ReturnsOK(t *testing.T) {
	server := startTestHTTPServer(t)
	defer server.Close()

	// Create a transfer first.
	createBody := map[string]any{
		"source_cluster":      "http-cluster-cancel",
		"source_pvc":          "ns/cancel-src-" + uuid.New().String()[:8],
		"destination_cluster": "http-cluster-cancel",
		"destination_pvc":     "ns/cancel-dst-" + uuid.New().String()[:8],
	}
	createResp := httpDoJSON(t, server, http.MethodPost, "/api/v1alpha1/transfers", createBody)
	var createResult map[string]any
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()

	transferID := createResult["id"].(string)

	// Cancel the transfer.
	resp := httpDoJSON(t, server, http.MethodDelete, "/api/v1alpha1/transfers/"+transferID, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var cancelResult map[string]any
	json.NewDecoder(resp.Body).Decode(&cancelResult)
	if cancelResult["state"] != string(domain.TransferStateCancelled) {
		t.Fatalf("expected state %q, got %v", domain.TransferStateCancelled, cancelResult["state"])
	}

	// Verify in DB.
	pool := getTestPool(t)
	transferRepo := pgstore.NewTransferRepository(pool)
	tid, _ := uuid.Parse(transferID)
	tr, _ := transferRepo.GetTransferByID(context.Background(), tid)
	if tr.State != domain.TransferStateCancelled {
		t.Fatalf("expected DB state cancelled, got %q", tr.State)
	}
}

func TestHTTP_GetTransferEvents(t *testing.T) {
	server := startTestHTTPServer(t)
	defer server.Close()

	// Create a transfer (events are auto-created by the orchestrator).
	createBody := map[string]any{
		"source_cluster":      "http-cluster-events",
		"source_pvc":          "ns/events-src-" + uuid.New().String()[:8],
		"destination_cluster": "http-cluster-events",
		"destination_pvc":     "ns/events-dst-" + uuid.New().String()[:8],
	}
	createResp := httpDoJSON(t, server, http.MethodPost, "/api/v1alpha1/transfers", createBody)
	var createResult map[string]any
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()

	transferID := createResult["id"].(string)

	// Get events.
	resp := httpDoJSON(t, server, http.MethodGet, "/api/v1alpha1/transfers/"+transferID+"/events", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var eventsResult struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eventsResult); err != nil {
		t.Fatalf("decoding events: %v", err)
	}
	// Orchestrator creates "created", "validated", and "started" events.
	if len(eventsResult.Events) < 1 {
		t.Fatalf("expected at least 1 event, got %d", len(eventsResult.Events))
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-component-tests:p2:inst-http-tests
