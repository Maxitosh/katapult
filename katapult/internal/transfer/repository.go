package transfer

import (
	"context"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
)

// TransferRepository defines the persistence interface for transfer data.
//
// Callers are responsible for validating domain state transitions before calling
// mutation methods. The repository layer performs raw persistence operations and
// does not enforce state-machine invariants.
type TransferRepository interface {
	CreateTransfer(ctx context.Context, transfer *domain.Transfer) error
	GetTransferByID(ctx context.Context, id uuid.UUID) (*domain.Transfer, error)
	UpdateTransferState(ctx context.Context, id uuid.UUID, state domain.TransferState) error
	UpdateTransferStrategy(ctx context.Context, id uuid.UUID, strategy domain.TransferStrategy) error
	UpdateTransferProgress(ctx context.Context, id uuid.UUID, bytesTransferred, bytesTotal int64, chunksCompleted, chunksTotal int) error
	UpdateTransferStarted(ctx context.Context, id uuid.UUID) error
	UpdateTransferCompleted(ctx context.Context, id uuid.UUID) error
	UpdateTransferFailed(ctx context.Context, id uuid.UUID, errorMessage string) error
	IncrementRetryCount(ctx context.Context, id uuid.UUID) error
	CreateTransferEvent(ctx context.Context, event *domain.TransferEvent) error
	GetTransferEvents(ctx context.Context, transferID uuid.UUID) ([]domain.TransferEvent, error)
}
