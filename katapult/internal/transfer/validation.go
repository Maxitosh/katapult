package transfer

import (
	"context"
	"fmt"
	"log/slog"
)

// PVCFinder checks whether a PVC exists in a cluster with a healthy agent.
type PVCFinder interface {
	FindHealthyPVC(ctx context.Context, clusterID, pvcName string) (found bool, err error)
}

// AgentCommander sends commands to agents and queries PVC status.
type AgentCommander interface {
	IsPVCEmpty(ctx context.Context, clusterID, pvcName string) (bool, error)
	SendTransferCommand(ctx context.Context, agentID, transferID string, cmd any) error
	SendCancelCommand(ctx context.Context, agentID, transferID string) error
}

// ValidateRequest holds the parameters for transfer request validation.
type ValidateRequest struct {
	TransferID         string
	SourceCluster      string
	SourcePVC          string
	DestinationCluster string
	DestinationPVC     string
	AllowOverwrite     bool
}

// Validator validates transfer requests against the agent registry.
type Validator struct {
	pvcFinder PVCFinder
	commander AgentCommander
	logger    *slog.Logger
}

// NewValidator creates a new transfer request validator.
func NewValidator(pvcFinder PVCFinder, commander AgentCommander, logger *slog.Logger) *Validator {
	return &Validator{
		pvcFinder: pvcFinder,
		commander: commander,
		logger:    logger,
	}
}

// ValidateTransferRequest validates a transfer request against the agent registry.
// @cpt-algo:cpt-katapult-algo-transfer-engine-validate-request:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-initiation:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-dest-safety:p1
func (v *Validator) ValidateTransferRequest(ctx context.Context, req ValidateRequest) error {
	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-same-pvc
	if req.SourceCluster == req.DestinationCluster && req.SourcePVC == req.DestinationPVC {
		return fmt.Errorf("source and destination PVC cannot be the same")
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-same-pvc

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-lookup-source-pvc
	sourceFound, err := v.pvcFinder.FindHealthyPVC(ctx, req.SourceCluster, req.SourcePVC)
	if err != nil {
		return fmt.Errorf("looking up source PVC: %w", err)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-lookup-source-pvc

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-no-source
	if !sourceFound {
		return fmt.Errorf("source PVC %s not found in cluster %s. Verify the PVC exists and the agent on the owning node is healthy", req.SourcePVC, req.SourceCluster)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-no-source

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-lookup-dest-pvc
	destFound, err := v.pvcFinder.FindHealthyPVC(ctx, req.DestinationCluster, req.DestinationPVC)
	if err != nil {
		return fmt.Errorf("looking up destination PVC: %w", err)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-lookup-dest-pvc

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-no-dest
	if !destFound {
		return fmt.Errorf("destination PVC %s not found in cluster %s. Verify the PVC exists and the agent on the owning node is healthy", req.DestinationPVC, req.DestinationCluster)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-no-dest

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-check-dest-empty
	empty, err := v.commander.IsPVCEmpty(ctx, req.DestinationCluster, req.DestinationPVC)
	if err != nil {
		return fmt.Errorf("checking destination PVC empty status: %w", err)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-check-dest-empty

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-non-empty
	if !empty && !req.AllowOverwrite {
		return fmt.Errorf("destination PVC %s is not empty. Set allow_overwrite=true to overwrite existing data", req.DestinationPVC)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-reject-non-empty

	// @cpt-begin:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-return-valid
	v.logger.Info("transfer request validated", "transfer_id", req.TransferID)
	return nil
	// @cpt-end:cpt-katapult-algo-transfer-engine-validate-request:p1:inst-return-valid
}
