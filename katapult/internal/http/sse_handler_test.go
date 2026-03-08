package http

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/observability"
)

func TestSSE_TransferNotFound(t *testing.T) {
	ts, _, _ := setupTestServerWithHub(t)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/transfers/"+uuid.New().String()+"/progress", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestSSE_TerminalTransferReturnsSnapshot(t *testing.T) {
	ts, repo, _, _ := setupTestServerWithHubRepo(t)

	id := uuid.New()
	_ = repo.CreateTransfer(context.Background(), &domain.Transfer{
		ID:                 id,
		SourceCluster:      "c1",
		SourcePVC:          "p1",
		DestinationCluster: "c2",
		DestinationPVC:     "p2",
		State:              domain.TransferStateCompleted,
		BytesTransferred:   1000,
		BytesTotal:         1000,
		CreatedAt:          time.Now(),
	})

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/transfers/"+id.String()+"/progress", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)
	var dataLine string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	if dataLine == "" {
		t.Fatal("expected SSE data line")
	}

	var progress observability.EnrichedProgress
	if err := json.Unmarshal([]byte(dataLine), &progress); err != nil {
		t.Fatalf("failed to unmarshal progress: %v", err)
	}
	if progress.Status != "completed" {
		t.Errorf("status = %q, want %q", progress.Status, "completed")
	}
	if progress.BytesTransferred != 1000 {
		t.Errorf("BytesTransferred = %d, want 1000", progress.BytesTransferred)
	}
}

func TestSSE_ActiveTransferStreamsProgress(t *testing.T) {
	ts, repo, hub, _ := setupTestServerWithHubRepo(t)

	id := uuid.New()
	_ = repo.CreateTransfer(context.Background(), &domain.Transfer{
		ID:                 id,
		SourceCluster:      "c1",
		SourcePVC:          "p1",
		DestinationCluster: "c2",
		DestinationPVC:     "p2",
		State:              domain.TransferStateTransferring,
		CreatedAt:          time.Now(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1alpha1/transfers/"+id.String()+"/progress", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Publish a progress event.
	hub.Publish(id, observability.EnrichedProgress{
		TransferID:       id,
		BytesTransferred: 500,
		BytesTotal:       1000,
		PercentComplete:  50.0,
		Status:           "in_progress",
	})

	// Read events: first is initial state, second is the published one.
	scanner := bufio.NewScanner(resp.Body)
	eventCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			eventCount++
			if eventCount >= 2 {
				data := strings.TrimPrefix(line, "data: ")
				var progress observability.EnrichedProgress
				if err := json.Unmarshal([]byte(data), &progress); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if progress.BytesTransferred != 500 {
					t.Errorf("BytesTransferred = %d, want 500", progress.BytesTransferred)
				}
				cancel()
				return
			}
		}
	}
}

func TestSSE_InvalidID(t *testing.T) {
	ts, _, _ := setupTestServerWithHub(t)

	req, _ := http.NewRequest("GET", ts.URL+"/api/v1alpha1/transfers/not-a-uuid/progress", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}
