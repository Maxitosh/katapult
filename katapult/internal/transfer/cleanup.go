package transfer

import (
	"context"
	"log/slog"

	"github.com/maxitosh/katapult/internal/domain"
)

// CredentialManager issues and revokes transfer credentials.
type CredentialManager interface {
	RevokeCredentials(ctx context.Context, transferID string) error
}

// S3Client manages S3 objects for transfer staging.
type S3Client interface {
	DeleteTransferObjects(ctx context.Context, transferID string) error
}

// Cleaner executes resource cleanup for completed, failed, or cancelled transfers.
type Cleaner struct {
	credManager CredentialManager
	commander   AgentCommander
	s3Client    S3Client
	logger      *slog.Logger
}

// NewCleaner creates a new resource cleaner.
func NewCleaner(credManager CredentialManager, commander AgentCommander, s3Client S3Client, logger *slog.Logger) *Cleaner {
	return &Cleaner{
		credManager: credManager,
		commander:   commander,
		s3Client:    s3Client,
		logger:      logger,
	}
}

// ExecuteCleanup performs best-effort resource cleanup for a transfer.
// @cpt-algo:cpt-katapult-algo-transfer-engine-cleanup:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-cleanup:p1
func (c *Cleaner) ExecuteCleanup(ctx context.Context, transfer *domain.Transfer) error {
	transferID := transfer.ID.String()

	// @cpt-begin:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-revoke-credentials
	if err := c.credManager.RevokeCredentials(ctx, transferID); err != nil {
		// @cpt-begin:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-warn
		c.logger.Warn("failed to revoke credentials", "transfer_id", transferID, "error", err)
		// @cpt-end:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-warn
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-revoke-credentials

	// @cpt-begin:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-source
	if err := c.commander.SendCancelCommand(ctx, transfer.SourceCluster, transferID); err != nil {
		c.logger.Warn("failed to signal source agent cleanup", "transfer_id", transferID, "error", err)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-source

	// @cpt-begin:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-dest
	if err := c.commander.SendCancelCommand(ctx, transfer.DestinationCluster, transferID); err != nil {
		c.logger.Warn("failed to signal destination agent cleanup", "transfer_id", transferID, "error", err)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-dest

	// @cpt-begin:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-s3
	if transfer.Strategy == domain.TransferStrategyS3 {
		if err := c.s3Client.DeleteTransferObjects(ctx, transferID); err != nil {
			c.logger.Warn("failed to delete S3 objects", "transfer_id", transferID, "error", err)
		}
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-cleanup-s3

	// @cpt-begin:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-return-cleanup
	c.logger.Info("resource cleanup completed", "transfer_id", transferID)
	return nil
	// @cpt-end:cpt-katapult-algo-transfer-engine-cleanup:p1:inst-return-cleanup
}
