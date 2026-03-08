package observability

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// @cpt-algo:cpt-katapult-algo-observability-emit-progress:p1
// @cpt-dod:cpt-katapult-dod-observability-realtime-progress:p1

// EnrichedProgress holds the raw progress report plus derived fields.
type EnrichedProgress struct {
	TransferID            uuid.UUID     `json:"transfer_id"`
	BytesTransferred      int64         `json:"bytes_transferred"`
	BytesTotal            int64         `json:"bytes_total"`
	PercentComplete       float64       `json:"percent_complete"`
	SpeedBytesPerSec      float64       `json:"speed_bytes_sec"`
	FormattedSpeed        string        `json:"formatted_speed"`
	EstimatedTimeRemaining string       `json:"estimated_time_remaining"`
	ChunksCompleted       int           `json:"chunks_completed"`
	ChunksTotal           int           `json:"chunks_total"`
	CorrelationID         string        `json:"correlation_id"`
	Status                string        `json:"status"`
	ErrorMessage          string        `json:"error_message,omitempty"`
}

// RawProgress holds the raw data from an agent progress report.
type RawProgress struct {
	TransferID       uuid.UUID
	BytesTransferred int64
	BytesTotal       int64
	SpeedBytesPerSec float64
	ChunksCompleted  int
	ChunksTotal      int
	Status           string
	ErrorMessage     string
}

// EnrichProgress calculates derived fields from a raw progress report.
// @cpt-begin:cpt-katapult-algo-observability-emit-progress:p1:inst-receive-raw
// @cpt-end:cpt-katapult-algo-observability-emit-progress:p1:inst-receive-raw
func EnrichProgress(raw RawProgress, correlationID string) EnrichedProgress {
	ep := EnrichedProgress{
		TransferID:       raw.TransferID,
		BytesTransferred: raw.BytesTransferred,
		BytesTotal:       raw.BytesTotal,
		SpeedBytesPerSec: raw.SpeedBytesPerSec,
		ChunksCompleted:  raw.ChunksCompleted,
		ChunksTotal:      raw.ChunksTotal,
		Status:           raw.Status,
		ErrorMessage:     raw.ErrorMessage,
		CorrelationID:    correlationID,
	}

	// @cpt-begin:cpt-katapult-algo-observability-emit-progress:p1:inst-calc-percent
	if raw.BytesTotal > 0 {
		ep.PercentComplete = float64(raw.BytesTransferred) / float64(raw.BytesTotal) * 100
	}
	// @cpt-end:cpt-katapult-algo-observability-emit-progress:p1:inst-calc-percent

	// @cpt-begin:cpt-katapult-algo-observability-emit-progress:p1:inst-calc-eta
	// @cpt-begin:cpt-katapult-algo-observability-emit-progress:p1:inst-zero-speed
	if raw.SpeedBytesPerSec > 0 && raw.BytesTotal > 0 {
		remaining := float64(raw.BytesTotal-raw.BytesTransferred) / raw.SpeedBytesPerSec
		d := time.Duration(remaining * float64(time.Second))
		ep.EstimatedTimeRemaining = d.Truncate(time.Second).String()
	} else {
		ep.EstimatedTimeRemaining = "unknown"
	}
	// @cpt-end:cpt-katapult-algo-observability-emit-progress:p1:inst-zero-speed
	// @cpt-end:cpt-katapult-algo-observability-emit-progress:p1:inst-calc-eta

	// @cpt-begin:cpt-katapult-algo-observability-emit-progress:p1:inst-format-speed
	ep.FormattedSpeed = FormatSpeed(raw.SpeedBytesPerSec)
	// @cpt-end:cpt-katapult-algo-observability-emit-progress:p1:inst-format-speed

	// @cpt-begin:cpt-katapult-algo-observability-emit-progress:p1:inst-attach-correlation
	// correlation ID already set above
	// @cpt-end:cpt-katapult-algo-observability-emit-progress:p1:inst-attach-correlation

	// @cpt-begin:cpt-katapult-algo-observability-emit-progress:p1:inst-return-enriched
	return ep
	// @cpt-end:cpt-katapult-algo-observability-emit-progress:p1:inst-return-enriched
}

// FormatSpeed converts bytes per second into a human-readable string.
func FormatSpeed(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1<<30:
		return fmt.Sprintf("%.2f GB/s", bytesPerSec/float64(1<<30))
	case bytesPerSec >= 1<<20:
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/float64(1<<20))
	case bytesPerSec >= 1<<10:
		return fmt.Sprintf("%.2f KB/s", bytesPerSec/float64(1<<10))
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
}
