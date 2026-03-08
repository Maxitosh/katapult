package observability

import (
	"testing"

	"github.com/google/uuid"
)

func TestEnrichProgress(t *testing.T) {
	transferID := uuid.New()

	tests := []struct {
		name           string
		raw            RawProgress
		wantPercent    float64
		wantETA        string
		wantSpeed      string
	}{
		{
			name: "normal progress",
			raw: RawProgress{
				TransferID:       transferID,
				BytesTransferred: 500,
				BytesTotal:       1000,
				SpeedBytesPerSec: 100,
				ChunksCompleted:  1,
				ChunksTotal:      2,
				Status:           "in_progress",
			},
			wantPercent: 50.0,
			wantETA:     "5s",
			wantSpeed:   "100 B/s",
		},
		{
			name: "zero speed",
			raw: RawProgress{
				TransferID:       transferID,
				BytesTransferred: 500,
				BytesTotal:       1000,
				SpeedBytesPerSec: 0,
				Status:           "in_progress",
			},
			wantPercent: 50.0,
			wantETA:     "unknown",
			wantSpeed:   "0 B/s",
		},
		{
			name: "zero total bytes",
			raw: RawProgress{
				TransferID:       transferID,
				BytesTransferred: 0,
				BytesTotal:       0,
				SpeedBytesPerSec: 100,
				Status:           "in_progress",
			},
			wantPercent: 0,
			wantETA:     "unknown",
			wantSpeed:   "100 B/s",
		},
		{
			name: "GB/s speed",
			raw: RawProgress{
				TransferID:       transferID,
				BytesTransferred: 500_000_000_000,
				BytesTotal:       1_000_000_000_000,
				SpeedBytesPerSec: 2_000_000_000,
				Status:           "in_progress",
			},
			wantPercent: 50.0,
			wantSpeed:   "1.86 GB/s",
		},
		{
			name: "MB/s speed",
			raw: RawProgress{
				TransferID:       transferID,
				BytesTransferred: 100,
				BytesTotal:       1000,
				SpeedBytesPerSec: 5_000_000,
				Status:           "in_progress",
			},
			wantPercent: 10.0,
			wantSpeed:   "4.77 MB/s",
		},
		{
			name: "KB/s speed",
			raw: RawProgress{
				TransferID:       transferID,
				BytesTransferred: 100,
				BytesTotal:       1000,
				SpeedBytesPerSec: 5_000,
				Status:           "in_progress",
			},
			wantPercent: 10.0,
			wantSpeed:   "4.88 KB/s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := EnrichProgress(tc.raw, "corr-123")

			if result.PercentComplete != tc.wantPercent {
				t.Errorf("PercentComplete = %f, want %f", result.PercentComplete, tc.wantPercent)
			}
			if tc.wantETA != "" && result.EstimatedTimeRemaining != tc.wantETA {
				t.Errorf("EstimatedTimeRemaining = %q, want %q", result.EstimatedTimeRemaining, tc.wantETA)
			}
			if result.FormattedSpeed != tc.wantSpeed {
				t.Errorf("FormattedSpeed = %q, want %q", result.FormattedSpeed, tc.wantSpeed)
			}
			if result.CorrelationID != "corr-123" {
				t.Errorf("CorrelationID = %q, want %q", result.CorrelationID, "corr-123")
			}
		})
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"zero", 0, "0 B/s"},
		{"bytes", 512, "512 B/s"},
		{"kilobytes", 1024, "1.00 KB/s"},
		{"megabytes", 1048576, "1.00 MB/s"},
		{"gigabytes", 1073741824, "1.00 GB/s"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatSpeed(tc.input)
			if got != tc.want {
				t.Errorf("FormatSpeed(%f) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
