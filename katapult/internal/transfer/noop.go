package transfer

import (
	"context"
)

// NoopCommander is a no-op implementation of AgentCommander.
// Used when the real agent communication layer is not yet available.
type NoopCommander struct{}

func (NoopCommander) IsPVCEmpty(_ context.Context, _, _ string) (bool, error) {
	// In no-op mode, assume PVCs are empty to allow API-level testing.
	return true, nil
}

func (NoopCommander) SendTransferCommand(_ context.Context, _, _ string, _ any) error {
	// No-op: transfer command accepted but no actual data movement.
	return nil
}

func (NoopCommander) SendCancelCommand(_ context.Context, _, _ string) error {
	// No-op: cancel command accepted.
	return nil
}

// NoopCredentialManager is a no-op implementation of CredentialManager.
type NoopCredentialManager struct{}

func (NoopCredentialManager) RevokeCredentials(_ context.Context, _ string) error {
	return nil
}

// NoopS3Client is a no-op implementation of S3Client.
type NoopS3Client struct{}

func (NoopS3Client) DeleteTransferObjects(_ context.Context, _ string) error {
	return nil
}

// NoopPVCFinder is a no-op implementation of PVCFinder.
type NoopPVCFinder struct{}

func (NoopPVCFinder) FindHealthyPVC(_ context.Context, _, _ string) (bool, error) {
	// In no-op mode, assume PVCs exist to allow API-level testing.
	return true, nil
}
