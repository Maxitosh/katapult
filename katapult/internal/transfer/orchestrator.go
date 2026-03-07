package transfer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
)

// CreateTransferRequest holds the parameters for creating a new transfer.
type CreateTransferRequest struct {
	SourceCluster      string
	SourcePVC          string
	DestinationCluster string
	DestinationPVC     string
	StrategyOverride   *string
	AllowOverwrite     bool
	RetryMax           *int
	CreatedBy          string
}

// CreateTransferResponse holds the result of creating a transfer.
type CreateTransferResponse struct {
	TransferID uuid.UUID
	State      domain.TransferState
}

// ProgressReport holds progress data reported by an agent.
type ProgressReport struct {
	TransferID       uuid.UUID
	BytesTransferred int64
	BytesTotal       int64
	Speed            float64
	ChunksCompleted  int
	ChunksTotal      int
	Status           string // "in_progress", "completed", "failed"
	ErrorMessage     string
}

// Orchestrator coordinates transfer lifecycle operations.
// @cpt-dod:cpt-katapult-dod-transfer-engine-initiation:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-cancellation:p1
type Orchestrator struct {
	repo        TransferRepository
	validator   *Validator
	cleaner     *Cleaner
	commander   AgentCommander
	credManager CredentialManager
	s3Config    S3Config
	config      domain.TransferConfig
	logger      *slog.Logger
}

// NewOrchestrator creates a new transfer orchestrator.
func NewOrchestrator(
	repo TransferRepository,
	validator *Validator,
	cleaner *Cleaner,
	commander AgentCommander,
	credManager CredentialManager,
	s3Config S3Config,
	config domain.TransferConfig,
	logger *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		repo:        repo,
		validator:   validator,
		cleaner:     cleaner,
		commander:   commander,
		credManager: credManager,
		s3Config:    s3Config,
		config:      config,
		logger:      logger,
	}
}

// CreateTransfer orchestrates the creation of a new transfer.
// @cpt-flow:cpt-katapult-flow-transfer-engine-initiate:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-initiation:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-persistence:p1
func (o *Orchestrator) CreateTransfer(ctx context.Context, req CreateTransferRequest) (*CreateTransferResponse, error) {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-submit-request
	retryMax := o.config.RetryMaxAttempts
	if req.RetryMax != nil {
		retryMax = *req.RetryMax
	}

	transferID := uuid.New()
	now := time.Now()

	transfer := &domain.Transfer{
		ID:                 transferID,
		SourceCluster:      req.SourceCluster,
		SourcePVC:          req.SourcePVC,
		DestinationCluster: req.DestinationCluster,
		DestinationPVC:     req.DestinationPVC,
		State:              domain.TransferStatePending,
		AllowOverwrite:     req.AllowOverwrite,
		RetryMax:           retryMax,
		CreatedBy:          req.CreatedBy,
		CreatedAt:          now,
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-submit-request

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-create-transfer
	if err := o.repo.CreateTransfer(ctx, transfer); err != nil {
		return nil, fmt.Errorf("creating transfer record: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-create-transfer

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-event-created
	o.createEvent(ctx, transferID, "created", "Transfer created", nil)
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-event-created

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-transition-validating
	if err := transfer.TransitionTo(domain.TransferStateValidating); err != nil {
		return nil, fmt.Errorf("transition to validating: %w", err)
	}
	if err := o.repo.UpdateTransferState(ctx, transferID, domain.TransferStateValidating); err != nil {
		return nil, fmt.Errorf("persisting validating state: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-transition-validating

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-run-validate
	err := o.validator.ValidateTransferRequest(ctx, ValidateRequest{
		TransferID:         transferID.String(),
		SourceCluster:      req.SourceCluster,
		SourcePVC:          req.SourcePVC,
		DestinationCluster: req.DestinationCluster,
		DestinationPVC:     req.DestinationPVC,
		AllowOverwrite:     req.AllowOverwrite,
	})
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-run-validate

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-validation-fail
	if err != nil {
		_ = transfer.TransitionTo(domain.TransferStateFailed)
		_ = o.repo.UpdateTransferFailed(ctx, transferID, err.Error())
		o.createEvent(ctx, transferID, "failed", fmt.Sprintf("Validation failed: %s", err.Error()), nil)
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-validation-fail

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-db-event-validated
	o.createEvent(ctx, transferID, "validated", "Transfer request validated successfully", nil)
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-db-event-validated

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-run-strategy
	strategy, err := SelectStrategy(req.SourceCluster, req.DestinationCluster, req.StrategyOverride, o.s3Config)
	if err != nil {
		_ = transfer.TransitionTo(domain.TransferStateFailed)
		_ = o.repo.UpdateTransferFailed(ctx, transferID, err.Error())
		return nil, fmt.Errorf("strategy selection: %w", err)
	}
	transfer.Strategy = strategy
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-run-strategy

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-strategy
	if err := o.repo.UpdateTransferStrategy(ctx, transferID, strategy); err != nil {
		return nil, fmt.Errorf("persisting strategy: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-strategy

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-request-credentials
	o.logger.Info("requesting credentials for transfer", "transfer_id", transferID, "strategy", strategy)
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-request-credentials

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-transition-transferring
	if err := transfer.TransitionTo(domain.TransferStateTransferring); err != nil {
		return nil, fmt.Errorf("transition to transferring: %w", err)
	}
	if err := o.repo.UpdateTransferState(ctx, transferID, domain.TransferStateTransferring); err != nil {
		return nil, fmt.Errorf("persisting transferring state: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-transition-transferring

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-started
	if err := o.repo.UpdateTransferStarted(ctx, transferID); err != nil {
		return nil, fmt.Errorf("persisting started_at: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-started

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-event-started
	o.createEvent(ctx, transferID, "started", fmt.Sprintf("Data transfer started with %s strategy", strategy), nil)
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-event-started

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-command-source
	if err := o.commander.SendTransferCommand(ctx, req.SourceCluster, transferID.String(), nil); err != nil {
		// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-command-timeout
		_ = transfer.TransitionTo(domain.TransferStateFailed)
		_ = o.repo.UpdateTransferFailed(ctx, transferID, fmt.Sprintf("source agent command failed: %s", err.Error()))
		o.createEvent(ctx, transferID, "failed", fmt.Sprintf("Source agent command failed: %s", err.Error()), nil)
		return nil, fmt.Errorf("sending command to source agent: %w", err)
		// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-command-timeout
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-command-source

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-command-dest
	if err := o.commander.SendTransferCommand(ctx, req.DestinationCluster, transferID.String(), nil); err != nil {
		_ = transfer.TransitionTo(domain.TransferStateFailed)
		_ = o.repo.UpdateTransferFailed(ctx, transferID, fmt.Sprintf("destination agent command failed: %s", err.Error()))
		o.createEvent(ctx, transferID, "failed", fmt.Sprintf("Destination agent command failed: %s", err.Error()), nil)
		return nil, fmt.Errorf("sending command to destination agent: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-command-dest

	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-return-transfer
	o.logger.Info("transfer initiated", "transfer_id", transferID, "strategy", strategy)
	return &CreateTransferResponse{
		TransferID: transferID,
		State:      domain.TransferStateTransferring,
	}, nil
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-return-transfer
}

// CancelTransfer cancels an active transfer.
// @cpt-flow:cpt-katapult-flow-transfer-engine-cancel:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-cancellation:p1
func (o *Orchestrator) CancelTransfer(ctx context.Context, transferID uuid.UUID) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-submit-cancel
	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-db-load-transfer
	transfer, err := o.repo.GetTransferByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("loading transfer: %w", err)
	}
	if transfer == nil {
		return fmt.Errorf("transfer %s not found", transferID)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-db-load-transfer
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-submit-cancel

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-reject-terminal
	if transfer.IsTerminal() {
		return fmt.Errorf("transfer already in terminal state: %s", transfer.State)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-reject-terminal

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-transition-cancelled
	if err := transfer.TransitionTo(domain.TransferStateCancelled); err != nil {
		return fmt.Errorf("transition to cancelled: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-transition-cancelled

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-db-set-cancelled
	now := time.Now()
	transfer.CompletedAt = &now
	if err := o.repo.UpdateTransferState(ctx, transferID, domain.TransferStateCancelled); err != nil {
		return fmt.Errorf("persisting cancelled state: %w", err)
	}
	if err := o.repo.UpdateTransferCompleted(ctx, transferID); err != nil {
		return fmt.Errorf("persisting completed_at: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-db-set-cancelled

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-db-event-cancelled
	o.createEvent(ctx, transferID, "cancelled", "Transfer cancelled by operator", nil)
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-db-event-cancelled

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-cancel-source
	if err := o.commander.SendCancelCommand(ctx, transfer.SourceCluster, transferID.String()); err != nil {
		o.logger.Warn("failed to send cancel to source agent", "transfer_id", transferID, "error", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-cancel-source

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-cancel-dest
	if err := o.commander.SendCancelCommand(ctx, transfer.DestinationCluster, transferID.String()); err != nil {
		o.logger.Warn("failed to send cancel to destination agent", "transfer_id", transferID, "error", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-cancel-dest

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-run-cleanup
	if err := o.cleaner.ExecuteCleanup(ctx, transfer); err != nil {
		o.logger.Warn("cleanup failed during cancellation", "transfer_id", transferID, "error", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-run-cleanup

	// @cpt-begin:cpt-katapult-flow-transfer-engine-cancel:p1:inst-return-cancelled
	o.logger.Info("transfer cancelled", "transfer_id", transferID)
	return nil
	// @cpt-end:cpt-katapult-flow-transfer-engine-cancel:p1:inst-return-cancelled
}

// HandleProgress processes a progress report from an agent.
// @cpt-flow:cpt-katapult-flow-transfer-engine-report-progress:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-autonomy:p1
func (o *Orchestrator) HandleProgress(ctx context.Context, report ProgressReport) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-receive-progress
	transfer, err := o.repo.GetTransferByID(ctx, report.TransferID)
	if err != nil {
		return fmt.Errorf("loading transfer: %w", err)
	}
	if transfer == nil {
		return fmt.Errorf("transfer %s not found", report.TransferID)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-receive-progress

	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-update-progress
	if err := o.repo.UpdateTransferProgress(ctx, report.TransferID,
		report.BytesTransferred, report.BytesTotal,
		report.ChunksCompleted, report.ChunksTotal); err != nil {
		return fmt.Errorf("updating progress: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-update-progress

	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-progress
	o.createEvent(ctx, report.TransferID, "progress", "Progress update", map[string]string{
		"bytes_transferred": fmt.Sprintf("%d", report.BytesTransferred),
		"bytes_total":       fmt.Sprintf("%d", report.BytesTotal),
		"speed":             fmt.Sprintf("%.2f", report.Speed),
		"chunks_completed":  fmt.Sprintf("%d", report.ChunksCompleted),
		"chunks_total":      fmt.Sprintf("%d", report.ChunksTotal),
	})
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-progress

	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-completed
	if report.Status == "completed" {
		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-transition-completed
		if err := transfer.TransitionTo(domain.TransferStateCompleted); err != nil {
			return fmt.Errorf("transition to completed: %w", err)
		}
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-transition-completed

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-completed
		if err := o.repo.UpdateTransferState(ctx, report.TransferID, domain.TransferStateCompleted); err != nil {
			return fmt.Errorf("persisting completed state: %w", err)
		}
		if err := o.repo.UpdateTransferCompleted(ctx, report.TransferID); err != nil {
			return fmt.Errorf("persisting completed_at: %w", err)
		}
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-completed

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-completed
		o.createEvent(ctx, report.TransferID, "completed", "Transfer completed successfully", nil)
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-completed

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-cleanup-on-complete
		if err := o.cleaner.ExecuteCleanup(ctx, transfer); err != nil {
			o.logger.Warn("cleanup failed after completion", "transfer_id", report.TransferID, "error", err)
		}
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-cleanup-on-complete

		return nil
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-completed

	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-failed
	if report.Status == "failed" {
		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-run-retry
		decision := ApplyRetryBackoff(
			transfer.RetryCount, transfer.RetryMax,
			o.config.RetryBaseDelay, o.config.RetryMaxDelay,
			o.config.JitterFactor,
		)
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-run-retry

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-retry
		if decision.Action == "retry" {
			// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-increment-retry
			if err := o.repo.IncrementRetryCount(ctx, report.TransferID); err != nil {
				return fmt.Errorf("incrementing retry count: %w", err)
			}
			// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-increment-retry

			// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-retried
			o.createEvent(ctx, report.TransferID, "retried",
				fmt.Sprintf("Retrying transfer, attempt %d", transfer.RetryCount+1), nil)
			// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-retried

			// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-resend-command
			o.logger.Info("retrying transfer after backoff",
				"transfer_id", report.TransferID,
				"delay", decision.Delay,
				"attempt", transfer.RetryCount+1,
			)
			// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-resend-command

			return nil
		}
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-retry

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-exhausted
		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-transition-failed
		if err := transfer.TransitionTo(domain.TransferStateFailed); err != nil {
			return fmt.Errorf("transition to failed: %w", err)
		}
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-transition-failed

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-failed
		errMsg := fmt.Sprintf("Transfer failed after %d retries: %s", transfer.RetryMax, report.ErrorMessage)
		if err := o.repo.UpdateTransferFailed(ctx, report.TransferID, errMsg); err != nil {
			return fmt.Errorf("persisting failed state: %w", err)
		}
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-failed

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-failed
		o.createEvent(ctx, report.TransferID, "failed", errMsg, nil)
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-event-failed

		// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-cleanup-on-fail
		if err := o.cleaner.ExecuteCleanup(ctx, transfer); err != nil {
			o.logger.Warn("cleanup failed after exhaustion", "transfer_id", report.TransferID, "error", err)
		}
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-cleanup-on-fail
		// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-exhausted

		return nil
	}
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-check-failed

	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-return-ack
	return nil
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-return-ack
}

func (o *Orchestrator) createEvent(ctx context.Context, transferID uuid.UUID, eventType, message string, metadata map[string]string) {
	event := &domain.TransferEvent{
		ID:         uuid.New(),
		TransferID: transferID,
		EventType:  eventType,
		Message:    message,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
	}
	if err := o.repo.CreateTransferEvent(ctx, event); err != nil {
		o.logger.Warn("failed to create transfer event",
			"transfer_id", transferID,
			"event_type", eventType,
			"error", err,
		)
	}
}
