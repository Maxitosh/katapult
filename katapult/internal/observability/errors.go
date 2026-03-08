package observability

import (
	"fmt"
	"strings"
)

// @cpt-algo:cpt-katapult-algo-observability-enrich-error:p1
// @cpt-dod:cpt-katapult-dod-observability-actionable-errors:p1

// ErrorCategory classifies transfer errors for remediation.
type ErrorCategory string

const (
	ErrorCategoryDiskFull           ErrorCategory = "disk_full"
	ErrorCategoryPermissionDenied   ErrorCategory = "permission_denied"
	ErrorCategoryNetworkUnreachable ErrorCategory = "network_unreachable"
	ErrorCategoryTimeout            ErrorCategory = "timeout"
	ErrorCategoryS3Error            ErrorCategory = "s3_error"
	ErrorCategoryUnknown            ErrorCategory = "unknown"
)

// ErrorContext holds optional context fields per error category.
type ErrorContext struct {
	AvailableSpace string
	RequiredSpace  string
	FilePath       string
	Permissions    string
	TargetAddress  string
	LastConnected  string
	OperationName  string
	Timeout        string
	Elapsed        string
	S3Operation    string
	S3Bucket       string
	S3KeyPrefix    string
	S3HTTPStatus   string
}

// ActionableError is an enriched error with classification and remediation.
type ActionableError struct {
	Category      ErrorCategory `json:"category"`
	Summary       string        `json:"summary"`
	Remediation   string        `json:"remediation"`
	OriginalError string        `json:"original_error"`
}

// String returns a human-readable representation of the actionable error.
func (a ActionableError) String() string {
	return fmt.Sprintf("%s. Suggested action: %s", a.Summary, a.Remediation)
}

// ClassifyError determines the error category by matching against known patterns.
// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-receive-error
// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-classify-error
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no space left") || strings.Contains(msg, "disk full") || strings.Contains(msg, "disk quota"):
		return ErrorCategoryDiskFull
	case strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied"):
		return ErrorCategoryPermissionDenied
	case strings.Contains(msg, "network unreachable") || strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no route to host") || strings.Contains(msg, "connection reset"):
		return ErrorCategoryNetworkUnreachable
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "context deadline"):
		return ErrorCategoryTimeout
	case strings.Contains(msg, "s3") || strings.Contains(msg, "nosuchbucket") ||
		strings.Contains(msg, "accessdenied") || strings.Contains(msg, "nosuchkey"):
		return ErrorCategoryS3Error
	default:
		return ErrorCategoryUnknown
	}
}

// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-classify-error
// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-receive-error

// EnrichError creates an ActionableError with category-specific context and remediation.
// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-compose-message
func EnrichError(err error, ctx ErrorContext) ActionableError {
	category := ClassifyError(err)
	original := ""
	if err != nil {
		original = err.Error()
	}

	var summary, remediation string

	switch category {
	// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-disk
	case ErrorCategoryDiskFull:
		summary = "Destination disk full"
		if ctx.AvailableSpace != "" && ctx.RequiredSpace != "" {
			summary = fmt.Sprintf("Destination disk full: %s available, %s required", ctx.AvailableSpace, ctx.RequiredSpace)
		}
		remediation = "Free disk space on the destination node or expand the PVC"
	// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-disk

	// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-permissions
	case ErrorCategoryPermissionDenied:
		summary = "Permission denied"
		if ctx.FilePath != "" {
			summary = fmt.Sprintf("Permission denied on %s", ctx.FilePath)
			if ctx.Permissions != "" {
				summary += fmt.Sprintf(" (current: %s)", ctx.Permissions)
			}
		}
		remediation = "Check file ownership and permissions on source and destination PVCs"
	// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-permissions

	// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-network
	case ErrorCategoryNetworkUnreachable:
		summary = "Network unreachable"
		if ctx.TargetAddress != "" {
			summary = fmt.Sprintf("Network unreachable: target %s", ctx.TargetAddress)
		}
		if ctx.LastConnected != "" {
			summary += fmt.Sprintf(" (last connected: %s)", ctx.LastConnected)
		}
		remediation = "Verify network connectivity between clusters and check firewall rules"
	// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-network

	// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-timeout
	case ErrorCategoryTimeout:
		summary = "Operation timed out"
		if ctx.OperationName != "" {
			summary = fmt.Sprintf("Operation %q timed out", ctx.OperationName)
		}
		if ctx.Timeout != "" && ctx.Elapsed != "" {
			summary += fmt.Sprintf(" (timeout: %s, elapsed: %s)", ctx.Timeout, ctx.Elapsed)
		}
		remediation = "Increase timeout or check for network/disk performance issues"
	// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-timeout

	// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-s3
	case ErrorCategoryS3Error:
		summary = "S3 operation failed"
		if ctx.S3Operation != "" {
			summary = fmt.Sprintf("S3 %s failed", ctx.S3Operation)
		}
		if ctx.S3Bucket != "" {
			summary += fmt.Sprintf(" (bucket: %s", ctx.S3Bucket)
			if ctx.S3KeyPrefix != "" {
				summary += fmt.Sprintf(", prefix: %s", ctx.S3KeyPrefix)
			}
			summary += ")"
		}
		if ctx.S3HTTPStatus != "" {
			summary += fmt.Sprintf(" HTTP %s", ctx.S3HTTPStatus)
		}
		remediation = "Verify S3 credentials, bucket existence, and IAM permissions"
	// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-s3

	// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-unknown
	default:
		summary = fmt.Sprintf("Transfer failed: %s", original)
		remediation = "Check agent logs for details and retry the transfer"
	// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-enrich-unknown
	}

	// @cpt-begin:cpt-katapult-algo-observability-enrich-error:p1:inst-return-actionable
	return ActionableError{
		Category:      category,
		Summary:       summary,
		Remediation:   remediation,
		OriginalError: original,
	}
	// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-return-actionable
}

// @cpt-end:cpt-katapult-algo-observability-enrich-error:p1:inst-compose-message
