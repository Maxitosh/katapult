package domain

import (
	"testing"

	"github.com/google/uuid"
)

func newTestTransfer(state TransferState) *Transfer {
	return &Transfer{
		ID:    uuid.New(),
		State: state,
	}
}

func TestTransferTransitionTo_ValidTransitions(t *testing.T) {
	tests := []struct {
		from TransferState
		to   TransferState
	}{
		{TransferStatePending, TransferStateValidating},
		{TransferStatePending, TransferStateCancelled},
		{TransferStateValidating, TransferStateTransferring},
		{TransferStateValidating, TransferStateFailed},
		{TransferStateValidating, TransferStateCancelled},
		{TransferStateTransferring, TransferStateCompleted},
		{TransferStateTransferring, TransferStateFailed},
		{TransferStateTransferring, TransferStateCancelled},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			tr := newTestTransfer(tt.from)
			if err := tr.TransitionTo(tt.to); err != nil {
				t.Fatalf("expected valid transition from %s to %s, got error: %v", tt.from, tt.to, err)
			}
			if tr.State != tt.to {
				t.Fatalf("expected state %s, got %s", tt.to, tr.State)
			}
		})
	}
}

func TestTransferTransitionTo_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from TransferState
		to   TransferState
	}{
		{"terminal completed->pending", TransferStateCompleted, TransferStatePending},
		{"terminal completed->validating", TransferStateCompleted, TransferStateValidating},
		{"terminal completed->transferring", TransferStateCompleted, TransferStateTransferring},
		{"terminal failed->pending", TransferStateFailed, TransferStatePending},
		{"terminal failed->validating", TransferStateFailed, TransferStateValidating},
		{"terminal cancelled->pending", TransferStateCancelled, TransferStatePending},
		{"terminal cancelled->transferring", TransferStateCancelled, TransferStateTransferring},
		{"backward transferring->validating", TransferStateTransferring, TransferStateValidating},
		{"backward transferring->pending", TransferStateTransferring, TransferStatePending},
		{"backward validating->pending", TransferStateValidating, TransferStatePending},
		{"skip pending->transferring", TransferStatePending, TransferStateTransferring},
		{"skip pending->completed", TransferStatePending, TransferStateCompleted},
		{"skip pending->failed", TransferStatePending, TransferStateFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTestTransfer(tt.from)
			if err := tr.TransitionTo(tt.to); err == nil {
				t.Fatalf("expected error for invalid transition from %s to %s", tt.from, tt.to)
			}
			if tr.State != tt.from {
				t.Fatalf("state should not change on invalid transition, expected %s got %s", tt.from, tr.State)
			}
		})
	}
}

func TestTransferIsTerminal(t *testing.T) {
	tests := []struct {
		state    TransferState
		terminal bool
	}{
		{TransferStatePending, false},
		{TransferStateValidating, false},
		{TransferStateTransferring, false},
		{TransferStateCompleted, true},
		{TransferStateFailed, true},
		{TransferStateCancelled, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			tr := newTestTransfer(tt.state)
			if got := tr.IsTerminal(); got != tt.terminal {
				t.Fatalf("IsTerminal() for %s: expected %v, got %v", tt.state, tt.terminal, got)
			}
		})
	}
}
