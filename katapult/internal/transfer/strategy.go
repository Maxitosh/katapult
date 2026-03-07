package transfer

import (
	"fmt"

	"github.com/maxitosh/katapult/internal/domain"
)

// S3Config holds the S3 availability configuration for strategy selection.
type S3Config struct {
	Configured bool
}

// SelectStrategy selects the optimal transfer strategy based on cluster topology.
// @cpt-algo:cpt-katapult-algo-transfer-engine-select-strategy:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-strategy:p1
func SelectStrategy(sourceCluster, destCluster string, strategyOverride *string, s3Config S3Config) (domain.TransferStrategy, error) {
	// @cpt-begin:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-check-override
	if strategyOverride != nil {
		override := domain.TransferStrategy(*strategyOverride)
		// @cpt-begin:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-reject-invalid-strategy
		switch override {
		case domain.TransferStrategyStream, domain.TransferStrategyS3, domain.TransferStrategyDirect:
			// @cpt-begin:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-return-override
			return override, nil
			// @cpt-end:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-return-override
		default:
			return "", fmt.Errorf("invalid strategy: %s. Valid options: stream, s3, direct", *strategyOverride)
		}
		// @cpt-end:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-reject-invalid-strategy
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-check-override

	// @cpt-begin:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-select-stream
	if sourceCluster == destCluster {
		return domain.TransferStrategyStream, nil
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-select-stream

	// @cpt-begin:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-select-s3
	if s3Config.Configured {
		return domain.TransferStrategyS3, nil
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-select-s3

	// @cpt-begin:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-select-direct
	return domain.TransferStrategyDirect, nil
	// @cpt-end:cpt-katapult-algo-transfer-engine-select-strategy:p1:inst-select-direct
}
