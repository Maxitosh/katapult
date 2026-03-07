//go:build integration

package postgres

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
)

func setupTransferTestDB(t *testing.T) *TransferRepository {
	t.Helper()
	pool := setupTestDB(t)

	ctx := context.Background()
	migrationsDir := filepath.Join("migrations")
	transferMigrations := []string{
		"005_create_transfers.up.sql",
		"006_create_transfer_events.up.sql",
	}
	for _, m := range transferMigrations {
		data, err := os.ReadFile(filepath.Join(migrationsDir, m))
		if err != nil {
			t.Fatalf("reading migration %s: %v", m, err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("running migration %s: %v", m, err)
		}
	}

	return NewTransferRepository(pool)
}

func newTestTransferRecord() *domain.Transfer {
	return &domain.Transfer{
		ID:                 uuid.New(),
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-source",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
		State:              domain.TransferStatePending,
		AllowOverwrite:     false,
		RetryMax:           3,
		CreatedBy:          "operator@example.com",
		CreatedAt:          time.Now(),
	}
}

func TestCreateTransfer_AndGetByID(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	got, err := repo.GetTransferByID(ctx, transfer.ID)
	if err != nil {
		t.Fatalf("getting transfer: %v", err)
	}
	if got == nil {
		t.Fatal("expected transfer, got nil")
	}
	if got.SourceCluster != "cluster-a" {
		t.Fatalf("expected source_cluster %q, got %q", "cluster-a", got.SourceCluster)
	}
	if got.State != domain.TransferStatePending {
		t.Fatalf("expected state %q, got %q", domain.TransferStatePending, got.State)
	}
}

func TestGetTransferByID_NotFound(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	got, err := repo.GetTransferByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent transfer")
	}
}

func TestUpdateTransferState(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	if err := repo.UpdateTransferState(ctx, transfer.ID, domain.TransferStateValidating); err != nil {
		t.Fatalf("updating state: %v", err)
	}

	got, _ := repo.GetTransferByID(ctx, transfer.ID)
	if got.State != domain.TransferStateValidating {
		t.Fatalf("expected state %q, got %q", domain.TransferStateValidating, got.State)
	}
}

func TestUpdateTransferStrategy(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	if err := repo.UpdateTransferStrategy(ctx, transfer.ID, domain.TransferStrategyS3); err != nil {
		t.Fatalf("updating strategy: %v", err)
	}

	got, _ := repo.GetTransferByID(ctx, transfer.ID)
	if got.Strategy != domain.TransferStrategyS3 {
		t.Fatalf("expected strategy %q, got %q", domain.TransferStrategyS3, got.Strategy)
	}
}

func TestUpdateTransferProgress(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	if err := repo.UpdateTransferProgress(ctx, transfer.ID, 500, 1000, 2, 4); err != nil {
		t.Fatalf("updating progress: %v", err)
	}

	got, _ := repo.GetTransferByID(ctx, transfer.ID)
	if got.BytesTransferred != 500 || got.BytesTotal != 1000 {
		t.Fatalf("expected bytes 500/1000, got %d/%d", got.BytesTransferred, got.BytesTotal)
	}
	if got.ChunksCompleted != 2 || got.ChunksTotal != 4 {
		t.Fatalf("expected chunks 2/4, got %d/%d", got.ChunksCompleted, got.ChunksTotal)
	}
}

func TestUpdateTransferStarted(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	if err := repo.UpdateTransferStarted(ctx, transfer.ID); err != nil {
		t.Fatalf("updating started: %v", err)
	}

	got, _ := repo.GetTransferByID(ctx, transfer.ID)
	if got.StartedAt == nil {
		t.Fatal("expected started_at to be set")
	}
}

func TestUpdateTransferCompleted(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	if err := repo.UpdateTransferCompleted(ctx, transfer.ID); err != nil {
		t.Fatalf("updating completed: %v", err)
	}

	got, _ := repo.GetTransferByID(ctx, transfer.ID)
	if got.CompletedAt == nil {
		t.Fatal("expected completed_at to be set")
	}
	// State is set separately via UpdateTransferState; UpdateTransferCompleted only sets completed_at.
	if got.State != domain.TransferStatePending {
		t.Fatalf("expected state unchanged (pending), got %q", got.State)
	}
}

func TestUpdateTransferFailed(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	if err := repo.UpdateTransferFailed(ctx, transfer.ID, "connection timeout"); err != nil {
		t.Fatalf("updating failed: %v", err)
	}

	got, _ := repo.GetTransferByID(ctx, transfer.ID)
	if got.State != domain.TransferStateFailed {
		t.Fatalf("expected state failed, got %q", got.State)
	}
	if got.ErrorMessage != "connection timeout" {
		t.Fatalf("expected error message %q, got %q", "connection timeout", got.ErrorMessage)
	}
}

func TestIncrementRetryCount(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	if err := repo.IncrementRetryCount(ctx, transfer.ID); err != nil {
		t.Fatalf("incrementing retry: %v", err)
	}
	if err := repo.IncrementRetryCount(ctx, transfer.ID); err != nil {
		t.Fatalf("incrementing retry: %v", err)
	}

	got, _ := repo.GetTransferByID(ctx, transfer.ID)
	if got.RetryCount != 2 {
		t.Fatalf("expected retry_count 2, got %d", got.RetryCount)
	}
}

func TestCreateAndGetTransferEvents(t *testing.T) {
	repo := setupTransferTestDB(t)
	ctx := context.Background()

	transfer := newTestTransferRecord()
	if err := repo.CreateTransfer(ctx, transfer); err != nil {
		t.Fatalf("creating transfer: %v", err)
	}

	event := &domain.TransferEvent{
		ID:         uuid.New(),
		TransferID: transfer.ID,
		EventType:  "created",
		Message:    "Transfer created",
		Metadata:   map[string]string{"source": "cluster-a"},
		CreatedAt:  time.Now(),
	}
	if err := repo.CreateTransferEvent(ctx, event); err != nil {
		t.Fatalf("creating event: %v", err)
	}

	events, err := repo.GetTransferEvents(ctx, transfer.ID)
	if err != nil {
		t.Fatalf("getting events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "created" {
		t.Fatalf("expected event_type %q, got %q", "created", events[0].EventType)
	}
	if events[0].Metadata["source"] != "cluster-a" {
		t.Fatalf("expected metadata source %q, got %q", "cluster-a", events[0].Metadata["source"])
	}
}
