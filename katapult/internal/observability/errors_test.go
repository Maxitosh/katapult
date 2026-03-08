package observability

import (
	"errors"
	"testing"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCat  ErrorCategory
	}{
		{"nil error", nil, ErrorCategoryUnknown},
		{"disk full", errors.New("no space left on device"), ErrorCategoryDiskFull},
		{"disk quota", errors.New("disk quota exceeded"), ErrorCategoryDiskFull},
		{"permission denied", errors.New("open /data/file: permission denied"), ErrorCategoryPermissionDenied},
		{"access denied", errors.New("access denied to resource"), ErrorCategoryPermissionDenied},
		{"network unreachable", errors.New("dial tcp: network unreachable"), ErrorCategoryNetworkUnreachable},
		{"connection refused", errors.New("connection refused"), ErrorCategoryNetworkUnreachable},
		{"connection reset", errors.New("connection reset by peer"), ErrorCategoryNetworkUnreachable},
		{"timeout", errors.New("context deadline exceeded"), ErrorCategoryTimeout},
		{"deadline exceeded", errors.New("operation timeout"), ErrorCategoryTimeout},
		{"s3 error", errors.New("S3 PutObject failed: NoSuchBucket"), ErrorCategoryS3Error},
		{"unknown error", errors.New("something unexpected happened"), ErrorCategoryUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyError(tc.err)
			if got != tc.wantCat {
				t.Errorf("ClassifyError() = %q, want %q", got, tc.wantCat)
			}
		})
	}
}

func TestEnrichError(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		ctx             ErrorContext
		wantCategory    ErrorCategory
		wantSummaryHas  string
		wantRemediation string
	}{
		{
			name:           "disk full with context",
			err:            errors.New("no space left on device"),
			ctx:            ErrorContext{AvailableSpace: "2.1 TB", RequiredSpace: "5.3 TB"},
			wantCategory:   ErrorCategoryDiskFull,
			wantSummaryHas: "2.1 TB available, 5.3 TB required",
		},
		{
			name:           "permission denied with path",
			err:            errors.New("permission denied"),
			ctx:            ErrorContext{FilePath: "/data/pvc-123/file.dat", Permissions: "0644"},
			wantCategory:   ErrorCategoryPermissionDenied,
			wantSummaryHas: "/data/pvc-123/file.dat",
		},
		{
			name:           "network with address",
			err:            errors.New("network unreachable"),
			ctx:            ErrorContext{TargetAddress: "10.0.0.5:50051"},
			wantCategory:   ErrorCategoryNetworkUnreachable,
			wantSummaryHas: "10.0.0.5:50051",
		},
		{
			name:           "timeout with details",
			err:            errors.New("context deadline exceeded"),
			ctx:            ErrorContext{OperationName: "rsync", Timeout: "30s", Elapsed: "30s"},
			wantCategory:   ErrorCategoryTimeout,
			wantSummaryHas: "rsync",
		},
		{
			name:           "s3 with bucket",
			err:            errors.New("S3 PutObject failed"),
			ctx:            ErrorContext{S3Operation: "PutObject", S3Bucket: "staging-bucket"},
			wantCategory:   ErrorCategoryS3Error,
			wantSummaryHas: "staging-bucket",
		},
		{
			name:           "unknown",
			err:            errors.New("weird failure"),
			ctx:            ErrorContext{},
			wantCategory:   ErrorCategoryUnknown,
			wantSummaryHas: "weird failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ae := EnrichError(tc.err, tc.ctx)
			if ae.Category != tc.wantCategory {
				t.Errorf("Category = %q, want %q", ae.Category, tc.wantCategory)
			}
			if tc.wantSummaryHas != "" {
				found := false
				if len(ae.Summary) >= len(tc.wantSummaryHas) {
					for i := 0; i <= len(ae.Summary)-len(tc.wantSummaryHas); i++ {
						if ae.Summary[i:i+len(tc.wantSummaryHas)] == tc.wantSummaryHas {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("Summary %q does not contain %q", ae.Summary, tc.wantSummaryHas)
				}
			}
			if ae.Remediation == "" {
				t.Error("Remediation should not be empty")
			}
			if ae.OriginalError != tc.err.Error() {
				t.Errorf("OriginalError = %q, want %q", ae.OriginalError, tc.err.Error())
			}

			// Test String() format
			s := ae.String()
			if s == "" {
				t.Error("String() should not be empty")
			}
		})
	}
}
