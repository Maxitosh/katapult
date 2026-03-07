package transfer

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/testutil"
)

func newTestOrchestrator(repo TransferRepository, commander AgentCommander) *Orchestrator {
	finder := &mockPVCFinder{pvcs: map[string]bool{
		"cluster-a/ns/pvc-src":  true,
		"cluster-b/ns/pvc-dest": true,
	}}
	validator := NewValidator(finder, commander, slog.Default())
	cleaner := NewCleaner(&mockCredManager{}, commander, &mockS3Client{}, slog.Default())

	return NewOrchestrator(
		repo, validator, cleaner, commander,
		&mockCredManager{}, S3Config{Configured: true},
		domain.DefaultTransferConfig(), slog.Default(),
	)
}

func TestCreateTransfer_Success(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}}
	orch := newTestOrchestrator(repo, cmd)

	resp, err := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
		CreatedBy:          "test-user",
	})
	if err != nil {
		t.Fatalf("CreateTransfer() error: %v", err)
	}
	if resp.State != domain.TransferStateTransferring {
		t.Fatalf("expected state transferring, got %s", resp.State)
	}

	// Verify transfer was persisted.
	transfer, err := repo.GetTransferByID(context.Background(), resp.TransferID)
	if err != nil {
		t.Fatalf("GetTransferByID() error: %v", err)
	}
	if transfer == nil {
		t.Fatal("expected transfer to be persisted")
	}
	if transfer.Strategy != domain.TransferStrategyS3 {
		t.Fatalf("expected s3 strategy for cross-cluster, got %s", transfer.Strategy)
	}
}

func TestCreateTransfer_ValidationFailure(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	// Commander returns non-empty PVC.
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": false}}
	orch := newTestOrchestrator(repo, cmd)

	_, err := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
		AllowOverwrite:     false,
	})
	if err == nil {
		t.Fatal("expected validation error for non-empty dest without overwrite")
	}
}

func TestCreateTransfer_IntraClusterStream(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	finder := &mockPVCFinder{pvcs: map[string]bool{
		"cluster-a/ns/pvc-src":  true,
		"cluster-a/ns/pvc-dest": true,
	}}
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-a/ns/pvc-dest": true}}
	validator := NewValidator(finder, cmd, slog.Default())
	cleaner := NewCleaner(&mockCredManager{}, cmd, &mockS3Client{}, slog.Default())
	orch := NewOrchestrator(repo, validator, cleaner, cmd, &mockCredManager{},
		S3Config{Configured: true}, domain.DefaultTransferConfig(), slog.Default())

	resp, err := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-a",
		DestinationPVC:     "ns/pvc-dest",
	})
	if err != nil {
		t.Fatalf("CreateTransfer() error: %v", err)
	}

	transfer, _ := repo.GetTransferByID(context.Background(), resp.TransferID)
	if transfer.Strategy != domain.TransferStrategyStream {
		t.Fatalf("expected stream strategy for intra-cluster, got %s", transfer.Strategy)
	}
}

func TestCancelTransfer_Success(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}}
	orch := newTestOrchestrator(repo, cmd)

	// Create a transfer first.
	resp, err := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
	})
	if err != nil {
		t.Fatalf("CreateTransfer() error: %v", err)
	}

	// Cancel it.
	if err := orch.CancelTransfer(context.Background(), resp.TransferID); err != nil {
		t.Fatalf("CancelTransfer() error: %v", err)
	}

	transfer, _ := repo.GetTransferByID(context.Background(), resp.TransferID)
	if transfer.State != domain.TransferStateCancelled {
		t.Fatalf("expected cancelled state, got %s", transfer.State)
	}
}

func TestCancelTransfer_TerminalState(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}}
	orch := newTestOrchestrator(repo, cmd)

	// Create and complete a transfer manually.
	id := uuid.New()
	transfer := &domain.Transfer{
		ID:        id,
		State:     domain.TransferStateCompleted,
		CreatedAt: time.Now(),
	}
	_ = repo.CreateTransfer(context.Background(), transfer)

	err := orch.CancelTransfer(context.Background(), id)
	if err == nil {
		t.Fatal("expected error when cancelling terminal transfer")
	}
}

func TestCancelTransfer_NotFound(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{}
	orch := newTestOrchestrator(repo, cmd)

	err := orch.CancelTransfer(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent transfer")
	}
}

func TestHandleProgress_Completed(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}}
	orch := newTestOrchestrator(repo, cmd)

	resp, _ := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
	})

	err := orch.HandleProgress(context.Background(), ProgressReport{
		TransferID:       resp.TransferID,
		BytesTransferred: 1000,
		BytesTotal:       1000,
		Status:           "completed",
	})
	if err != nil {
		t.Fatalf("HandleProgress() error: %v", err)
	}

	transfer, _ := repo.GetTransferByID(context.Background(), resp.TransferID)
	if transfer.State != domain.TransferStateCompleted {
		t.Fatalf("expected completed state, got %s", transfer.State)
	}
}

func TestHandleProgress_FailedWithRetry(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}}
	orch := newTestOrchestrator(repo, cmd)

	resp, _ := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
	})

	err := orch.HandleProgress(context.Background(), ProgressReport{
		TransferID:   resp.TransferID,
		Status:       "failed",
		ErrorMessage: "connection lost",
	})
	if err != nil {
		t.Fatalf("HandleProgress() error: %v", err)
	}

	// Transfer should still be transferring (retry, not exhausted).
	transfer, _ := repo.GetTransferByID(context.Background(), resp.TransferID)
	if transfer.State == domain.TransferStateFailed {
		t.Fatal("should not be failed yet — retries available")
	}
}

func TestHandleProgress_FailedExhausted(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}}
	orch := newTestOrchestrator(repo, cmd)

	// Create transfer with 0 max retries.
	zeroRetries := 0
	resp, _ := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
		RetryMax:           &zeroRetries,
	})

	err := orch.HandleProgress(context.Background(), ProgressReport{
		TransferID:   resp.TransferID,
		Status:       "failed",
		ErrorMessage: "disk full",
	})
	if err != nil {
		t.Fatalf("HandleProgress() error: %v", err)
	}

	transfer, _ := repo.GetTransferByID(context.Background(), resp.TransferID)
	if transfer.State != domain.TransferStateFailed {
		t.Fatalf("expected failed state after retry exhaustion, got %s", transfer.State)
	}
}

func TestHandleProgress_InProgress(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true}}
	orch := newTestOrchestrator(repo, cmd)

	resp, _ := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
	})

	err := orch.HandleProgress(context.Background(), ProgressReport{
		TransferID:       resp.TransferID,
		BytesTransferred: 500,
		BytesTotal:       1000,
		Status:           "in_progress",
	})
	if err != nil {
		t.Fatalf("HandleProgress() error: %v", err)
	}

	transfer, _ := repo.GetTransferByID(context.Background(), resp.TransferID)
	if transfer.BytesTransferred != 500 {
		t.Fatalf("expected 500 bytes transferred, got %d", transfer.BytesTransferred)
	}
}

func TestHandleProgress_NotFound(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &mockCommander{}
	orch := newTestOrchestrator(repo, cmd)

	err := orch.HandleProgress(context.Background(), ProgressReport{
		TransferID: uuid.New(),
		Status:     "in_progress",
	})
	if err == nil {
		t.Fatal("expected error for non-existent transfer")
	}
}

func TestCreateTransfer_CommandFailure(t *testing.T) {
	repo := testutil.NewMemTransferRepo()
	cmd := &failingCommander{
		emptyPVCs: map[string]bool{"cluster-b/ns/pvc-dest": true},
		failSend:  true,
	}
	finder := &mockPVCFinder{pvcs: map[string]bool{
		"cluster-a/ns/pvc-src":  true,
		"cluster-b/ns/pvc-dest": true,
	}}
	validator := NewValidator(finder, cmd, slog.Default())
	cleaner := NewCleaner(&mockCredManager{}, cmd, &mockS3Client{}, slog.Default())
	orch := NewOrchestrator(repo, validator, cleaner, cmd, &mockCredManager{},
		S3Config{Configured: true}, domain.DefaultTransferConfig(), slog.Default())

	_, err := orch.CreateTransfer(context.Background(), CreateTransferRequest{
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-src",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
	})
	if err == nil {
		t.Fatal("expected error when agent command fails")
	}
}

type failingCommander struct {
	emptyPVCs map[string]bool
	failSend  bool
}

func (f *failingCommander) IsPVCEmpty(_ context.Context, clusterID, pvcName string) (bool, error) {
	key := clusterID + "/" + pvcName
	empty, ok := f.emptyPVCs[key]
	if !ok {
		return true, nil
	}
	return empty, nil
}

func (f *failingCommander) SendTransferCommand(_ context.Context, _, _ string, _ any) error {
	if f.failSend {
		return fmt.Errorf("agent unreachable")
	}
	return nil
}

func (f *failingCommander) SendCancelCommand(_ context.Context, _, _ string) error {
	return nil
}
