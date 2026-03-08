package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	if m.TransfersTotal == nil {
		t.Error("TransfersTotal should not be nil")
	}
	if m.TransfersActive == nil {
		t.Error("TransfersActive should not be nil")
	}
	if m.TransferDurationSeconds == nil {
		t.Error("TransferDurationSeconds should not be nil")
	}

	// Verify metrics are registered by gathering.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	if len(families) == 0 {
		t.Error("expected registered metric families, got none")
	}
}

func TestMetrics_OnTransferStateChange(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.OnTransferStateChange("pending", "transferring", "stream")
	m.OnTransferStateChange("transferring", "completed", "stream")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	found := false
	for _, f := range families {
		if f.GetName() == "katapult_transfers_total" {
			found = true
			if len(f.GetMetric()) != 2 {
				t.Errorf("expected 2 metric series, got %d", len(f.GetMetric()))
			}
		}
	}
	if !found {
		t.Error("katapult_transfers_total metric not found")
	}
}

func TestMetrics_OnProgressReport(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.OnProgressReport(1024, 100.5)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	for _, f := range families {
		if f.GetName() == "katapult_mover_speed_bytes_per_sec" {
			metrics := f.GetMetric()
			if len(metrics) != 1 {
				t.Fatalf("expected 1 metric, got %d", len(metrics))
			}
			if metrics[0].GetGauge().GetValue() != 100.5 {
				t.Errorf("speed = %f, want 100.5", metrics[0].GetGauge().GetValue())
			}
		}
	}
}

func TestMetrics_OnTransferError(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.OnTransferError("disk_full")
	m.OnTransferError("disk_full")
	m.OnTransferError("timeout")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	for _, f := range families {
		if f.GetName() == "katapult_mover_errors_total" {
			if len(f.GetMetric()) != 2 {
				t.Errorf("expected 2 error categories, got %d", len(f.GetMetric()))
			}
		}
	}
}
