package http

// @cpt-dod:cpt-katapult-dod-api-cli-input-validation:p1
// @cpt-algo:cpt-katapult-algo-api-cli-validate-transfer-request:p1

// FieldError represents a single validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors holds a list of field-level errors.
type ValidationErrors struct {
	Errors []FieldError `json:"errors"`
}

func (v *ValidationErrors) add(field, message string) {
	v.Errors = append(v.Errors, FieldError{Field: field, Message: message})
}

func (v *ValidationErrors) hasErrors() bool {
	return len(v.Errors) > 0
}

// createTransferInput represents the JSON body for POST /transfers.
type createTransferInput struct {
	SourceCluster      string  `json:"source_cluster"`
	SourcePVC          string  `json:"source_pvc"`
	DestinationCluster string  `json:"destination_cluster"`
	DestinationPVC     string  `json:"destination_pvc"`
	Strategy           *string `json:"strategy,omitempty"`
	AllowOverwrite     bool    `json:"allow_overwrite"`
	RetryMax           *int    `json:"retry_max,omitempty"`
}

// ValidateCreateTransferInput validates a transfer creation request.
// @cpt-algo:cpt-katapult-algo-api-cli-validate-transfer-request:p1
func ValidateCreateTransferInput(req createTransferInput) *ValidationErrors {
	errs := &ValidationErrors{}

	// @cpt-begin:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-source
	if req.SourceCluster == "" {
		errs.add("source_cluster", "source cluster is required")
	}
	if req.SourcePVC == "" {
		errs.add("source_pvc", "source PVC is required")
	}
	// @cpt-end:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-source

	// @cpt-begin:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-dest
	if req.DestinationCluster == "" {
		errs.add("destination_cluster", "destination cluster is required")
	}
	if req.DestinationPVC == "" {
		errs.add("destination_pvc", "destination PVC is required")
	}
	// @cpt-end:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-dest

	// @cpt-begin:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-same
	if req.SourceCluster != "" && req.DestinationCluster != "" &&
		req.SourcePVC != "" && req.DestinationPVC != "" &&
		req.SourceCluster == req.DestinationCluster && req.SourcePVC == req.DestinationPVC {
		errs.add("destination_pvc", "source and destination must differ")
	}
	// @cpt-end:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-same

	// @cpt-begin:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-strategy
	if req.Strategy != nil {
		switch *req.Strategy {
		case "stream", "s3", "direct":
			// valid
		default:
			errs.add("strategy", "invalid strategy; must be one of: stream, s3, direct")
		}
	}
	// @cpt-end:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-strategy

	// @cpt-begin:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-retry
	if req.RetryMax != nil && *req.RetryMax < 1 {
		errs.add("retry_max", "maxAttempts must be >= 1")
	}
	// @cpt-end:cpt-katapult-algo-api-cli-validate-transfer-request:p1:inst-check-retry

	if errs.hasErrors() {
		return errs
	}
	return nil
}
