package testutil

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
)

// MemTransferRepo is an in-memory TransferRepository for unit testing.
type MemTransferRepo struct {
	mu        sync.Mutex
	Transfers map[uuid.UUID]*domain.Transfer
	Events    map[uuid.UUID][]domain.TransferEvent
}

// NewMemTransferRepo creates a new in-memory transfer repository.
func NewMemTransferRepo() *MemTransferRepo {
	return &MemTransferRepo{
		Transfers: make(map[uuid.UUID]*domain.Transfer),
		Events:    make(map[uuid.UUID][]domain.TransferEvent),
	}
}

func (m *MemTransferRepo) CreateTransfer(_ context.Context, transfer *domain.Transfer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stored := *transfer
	m.Transfers[transfer.ID] = &stored
	return nil
}

func (m *MemTransferRepo) GetTransferByID(_ context.Context, id uuid.UUID) (*domain.Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return nil, nil
	}
	cp := *t
	return &cp, nil
}

func (m *MemTransferRepo) UpdateTransferState(_ context.Context, id uuid.UUID, state domain.TransferState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}
	t.State = state
	return nil
}

func (m *MemTransferRepo) UpdateTransferStrategy(_ context.Context, id uuid.UUID, strategy domain.TransferStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}
	t.Strategy = strategy
	return nil
}

func (m *MemTransferRepo) UpdateTransferProgress(_ context.Context, id uuid.UUID, bytesTransferred, bytesTotal int64, chunksCompleted, chunksTotal int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}
	t.BytesTransferred = bytesTransferred
	t.BytesTotal = bytesTotal
	t.ChunksCompleted = chunksCompleted
	t.ChunksTotal = chunksTotal
	return nil
}

func (m *MemTransferRepo) UpdateTransferStarted(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}
	now := time.Now()
	t.StartedAt = &now
	return nil
}

func (m *MemTransferRepo) UpdateTransferCompleted(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

func (m *MemTransferRepo) UpdateTransferFailed(_ context.Context, id uuid.UUID, errorMessage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}
	t.State = domain.TransferStateFailed
	t.ErrorMessage = errorMessage
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

func (m *MemTransferRepo) IncrementRetryCount(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.Transfers[id]
	if !ok {
		return fmt.Errorf("transfer %s not found", id)
	}
	t.RetryCount++
	return nil
}

func (m *MemTransferRepo) CreateTransferEvent(_ context.Context, event *domain.TransferEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stored := *event
	m.Events[event.TransferID] = append(m.Events[event.TransferID], stored)
	return nil
}

func (m *MemTransferRepo) GetTransferEvents(_ context.Context, transferID uuid.UUID) ([]domain.TransferEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	events := m.Events[transferID]
	result := make([]domain.TransferEvent, len(events))
	copy(result, events)
	return result, nil
}

func (m *MemTransferRepo) ListTransfers(_ context.Context, filter domain.TransferFilter) ([]domain.Transfer, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var all []domain.Transfer
	for _, t := range m.Transfers {
		if filter.State != nil && t.State != *filter.State {
			continue
		}
		if filter.Cluster != nil && t.SourceCluster != *filter.Cluster && t.DestinationCluster != *filter.Cluster {
			continue
		}
		cp := *t
		all = append(all, cp)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := len(all)
	if filter.Offset > 0 && filter.Offset < len(all) {
		all = all[filter.Offset:]
	} else if filter.Offset >= len(all) {
		all = nil
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit < len(all) {
		all = all[:limit]
	}

	return all, total, nil
}
