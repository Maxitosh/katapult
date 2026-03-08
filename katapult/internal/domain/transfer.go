package domain

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// TransferState represents the lifecycle state of a transfer.
type TransferState string

const (
	TransferStatePending      TransferState = "pending"
	TransferStateValidating   TransferState = "validating"
	TransferStateTransferring TransferState = "transferring"
	TransferStateCompleted    TransferState = "completed"
	TransferStateFailed       TransferState = "failed"
	TransferStateCancelled    TransferState = "cancelled"
)

// TransferStrategy represents the data transfer method.
type TransferStrategy string

const (
	TransferStrategyStream TransferStrategy = "stream"
	TransferStrategyS3     TransferStrategy = "s3"
	TransferStrategyDirect TransferStrategy = "direct"
)

// Transfer represents a volume transfer between Kubernetes PVCs.
// @cpt-dod:cpt-katapult-dod-transfer-engine-persistence:p1
type Transfer struct {
	ID                 uuid.UUID        `json:"id"`
	SourceCluster      string           `json:"source_cluster"`
	SourcePVC          string           `json:"source_pvc"`
	DestinationCluster string           `json:"destination_cluster"`
	DestinationPVC     string           `json:"destination_pvc"`
	Strategy           TransferStrategy `json:"strategy"`
	State              TransferState    `json:"state"`
	AllowOverwrite     bool             `json:"allow_overwrite"`
	BytesTransferred   int64            `json:"bytes_transferred"`
	BytesTotal         int64            `json:"bytes_total"`
	ChunksCompleted    int              `json:"chunks_completed"`
	ChunksTotal        int              `json:"chunks_total"`
	ErrorMessage       string           `json:"error_message"`
	RetryCount         int              `json:"retry_count"`
	RetryMax           int              `json:"retry_max"`
	CreatedBy          string           `json:"created_by"`
	CreatedAt          time.Time        `json:"created_at"`
	StartedAt          *time.Time       `json:"started_at,omitempty"`
	CompletedAt        *time.Time       `json:"completed_at,omitempty"`
}

// TransferEvent represents an audit event in the transfer timeline.
// @cpt-dod:cpt-katapult-dod-transfer-engine-persistence:p1
type TransferEvent struct {
	ID         uuid.UUID         `json:"id"`
	TransferID uuid.UUID         `json:"transfer_id"`
	EventType  string            `json:"event_type"`
	Message    string            `json:"message"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}

// TransferFilter holds criteria for listing transfers.
// @cpt-dod:cpt-katapult-dod-api-cli-rest-transfer-endpoints:p1
type TransferFilter struct {
	State   *TransferState
	Cluster *string
	Limit   int
	Offset  int
}

// TransferConfig holds configuration parameters for transfer orchestration.
type TransferConfig struct {
	RetryMaxAttempts      int           `json:"retry_max_attempts"`
	RetryBaseDelay        time.Duration `json:"retry_base_delay"`
	RetryMaxDelay         time.Duration `json:"retry_max_delay"`
	JitterFactor          float64       `json:"jitter_factor"`
	ChunkSize             int64         `json:"chunk_size"`
	ValidationTimeout     time.Duration `json:"validation_timeout"`
	TransferCommandTimeout time.Duration `json:"transfer_command_timeout"`
}

// DefaultTransferConfig returns the default configuration from FEATURE 1.3.
func DefaultTransferConfig() TransferConfig {
	return TransferConfig{
		RetryMaxAttempts:      3,
		RetryBaseDelay:        5 * time.Second,
		RetryMaxDelay:         5 * time.Minute,
		JitterFactor:          0.3,
		ChunkSize:             4 * 1024 * 1024 * 1024, // 4 GiB
		ValidationTimeout:     30 * time.Second,
		TransferCommandTimeout: 60 * time.Second,
	}
}

// validTransferTransitions defines the allowed state transitions for the transfer lifecycle.
// @cpt-state:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1
var validTransferTransitions = map[TransferState][]TransferState{
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-pending-to-validating
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-pending-to-cancelled
	TransferStatePending: {TransferStateValidating, TransferStateCancelled},
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-pending-to-cancelled
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-pending-to-validating
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-validating-to-transferring
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-validating-to-failed
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-validating-to-cancelled
	TransferStateValidating: {TransferStateTransferring, TransferStateFailed, TransferStateCancelled},
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-validating-to-cancelled
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-validating-to-failed
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-validating-to-transferring
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-transferring-to-completed
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-transferring-to-failed
	// @cpt-begin:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-transferring-to-cancelled
	TransferStateTransferring: {TransferStateCompleted, TransferStateFailed, TransferStateCancelled},
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-transferring-to-cancelled
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-transferring-to-failed
	// @cpt-end:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1:inst-transferring-to-completed
}

// TransitionTo attempts to transition the transfer to the target state.
// Returns an error if the transition is invalid.
// @cpt-state:cpt-katapult-state-transfer-engine-transfer-lifecycle:p1
func (t *Transfer) TransitionTo(target TransferState) error {
	allowed, ok := validTransferTransitions[t.State]
	if !ok {
		return fmt.Errorf("no transitions defined from state %q (terminal state)", t.State)
	}
	if !slices.Contains(allowed, target) {
		return fmt.Errorf("invalid transition from %q to %q", t.State, target)
	}
	t.State = target
	return nil
}

// IsTerminal returns true if the transfer is in a terminal state.
func (t *Transfer) IsTerminal() bool {
	switch t.State {
	case TransferStateCompleted, TransferStateFailed, TransferStateCancelled:
		return true
	default:
		return false
	}
}
