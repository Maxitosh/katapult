package transfer

import (
	"context"
	"fmt"
)

// NoopCommander is a no-op implementation of AgentCommander.
// Used when the real agent communication layer is not yet available.
type NoopCommander struct{}

func (NoopCommander) IsPVCEmpty(_ context.Context, _, _ string) (bool, error) {
	return false, fmt.Errorf("agent commander not implemented")
}

func (NoopCommander) SendTransferCommand(_ context.Context, _, _ string, _ any) error {
	return fmt.Errorf("agent commander not implemented")
}

func (NoopCommander) SendCancelCommand(_ context.Context, _, _ string) error {
	return fmt.Errorf("agent commander not implemented")
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
	return false, fmt.Errorf("PVC finder not implemented")
}
