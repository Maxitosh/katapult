package transfer

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
)

type mockCredManager struct {
	revoked []string
	err     error
}

func (m *mockCredManager) RevokeCredentials(_ context.Context, transferID string) error {
	if m.err != nil {
		return m.err
	}
	m.revoked = append(m.revoked, transferID)
	return nil
}

type mockS3Client struct {
	deleted []string
	err     error
}

func (m *mockS3Client) DeleteTransferObjects(_ context.Context, transferID string) error {
	if m.err != nil {
		return m.err
	}
	m.deleted = append(m.deleted, transferID)
	return nil
}

type trackingCommander struct {
	cancelCalls []string
	err         error
}

func (t *trackingCommander) IsPVCEmpty(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (t *trackingCommander) SendTransferCommand(_ context.Context, _, _ string, _ any) error {
	return nil
}

func (t *trackingCommander) SendCancelCommand(_ context.Context, agentID, transferID string) error {
	if t.err != nil {
		return t.err
	}
	t.cancelCalls = append(t.cancelCalls, agentID+"/"+transferID)
	return nil
}

func TestExecuteCleanup_Success(t *testing.T) {
	cred := &mockCredManager{}
	cmd := &trackingCommander{}
	s3 := &mockS3Client{}
	cleaner := NewCleaner(cred, cmd, s3, slog.Default())

	transfer := &domain.Transfer{
		ID:                 uuid.New(),
		SourceCluster:      "cluster-a",
		DestinationCluster: "cluster-b",
		Strategy:           domain.TransferStrategyS3,
	}

	err := cleaner.ExecuteCleanup(context.Background(), transfer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cred.revoked) != 1 {
		t.Fatalf("expected 1 credential revocation, got %d", len(cred.revoked))
	}
	if len(cmd.cancelCalls) != 2 {
		t.Fatalf("expected 2 cancel calls (source+dest), got %d", len(cmd.cancelCalls))
	}
	if len(s3.deleted) != 1 {
		t.Fatalf("expected 1 S3 deletion, got %d", len(s3.deleted))
	}
}

func TestExecuteCleanup_SkipsS3ForNonS3Strategy(t *testing.T) {
	s3 := &mockS3Client{}
	cleaner := NewCleaner(&mockCredManager{}, &trackingCommander{}, s3, slog.Default())

	transfer := &domain.Transfer{
		ID:       uuid.New(),
		Strategy: domain.TransferStrategyStream,
	}

	err := cleaner.ExecuteCleanup(context.Background(), transfer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s3.deleted) != 0 {
		t.Fatalf("expected no S3 deletions for stream strategy, got %d", len(s3.deleted))
	}
}

func TestExecuteCleanup_BestEffort(t *testing.T) {
	cred := &mockCredManager{err: fmt.Errorf("revoke failed")}
	cmd := &trackingCommander{err: fmt.Errorf("agent unreachable")}
	s3 := &mockS3Client{err: fmt.Errorf("s3 delete failed")}
	cleaner := NewCleaner(cred, cmd, s3, slog.Default())

	transfer := &domain.Transfer{
		ID:       uuid.New(),
		Strategy: domain.TransferStrategyS3,
	}

	// Best-effort: all steps fail but cleanup still returns nil.
	err := cleaner.ExecuteCleanup(context.Background(), transfer)
	if err != nil {
		t.Fatalf("cleanup should be best-effort, got error: %v", err)
	}
}
