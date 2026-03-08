package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// @cpt-algo:cpt-katapult-algo-observability-collect-metrics:p2
// @cpt-dod:cpt-katapult-dod-observability-metrics-logging:p2

// MetricsRecorder is the interface used by the orchestrator to record metric events.
type MetricsRecorder interface {
	OnTransferStateChange(fromState, toState, strategy string)
	OnProgressReport(bytesTransferred int64, speedBytesPerSec float64)
	OnAgentHealthChange(state string, delta int)
	OnTransferError(category string)
}

// Metrics holds all Prometheus collectors for the Katapult control plane.
type Metrics struct {
	// @cpt-begin:cpt-katapult-algo-observability-collect-metrics:p2:inst-register-cp-metrics
	TransfersTotal          *prometheus.CounterVec
	TransfersActive         prometheus.Gauge
	TransferDurationSeconds prometheus.Histogram
	TransferBytesTotal      prometheus.Counter
	AgentHealthGauge        *prometheus.GaugeVec
	// @cpt-end:cpt-katapult-algo-observability-collect-metrics:p2:inst-register-cp-metrics

	// @cpt-begin:cpt-katapult-algo-observability-collect-metrics:p2:inst-register-agent-metrics
	MoverBytesTransferred prometheus.Counter
	MoverSpeedGauge       prometheus.Gauge
	MoverErrorsTotal      *prometheus.CounterVec
	ChunksTransferredTotal prometheus.Counter
	// @cpt-end:cpt-katapult-algo-observability-collect-metrics:p2:inst-register-agent-metrics
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		TransfersTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "katapult_transfers_total",
			Help: "Total number of transfers by status and strategy",
		}, []string{"status", "strategy"}),

		TransfersActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "katapult_transfers_active",
			Help: "Number of currently active transfers",
		}),

		TransferDurationSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "katapult_transfer_duration_seconds",
			Help:    "Transfer duration in seconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 12),
		}),

		TransferBytesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "katapult_transfer_bytes_total",
			Help: "Total bytes transferred across all transfers",
		}),

		AgentHealthGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "katapult_agent_health",
			Help: "Number of agents by health state",
		}, []string{"state"}),

		MoverBytesTransferred: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "katapult_mover_bytes_transferred_total",
			Help: "Total bytes transferred by movers",
		}),

		MoverSpeedGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "katapult_mover_speed_bytes_per_sec",
			Help: "Current mover transfer speed in bytes per second",
		}),

		MoverErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "katapult_mover_errors_total",
			Help: "Total mover errors by category",
		}, []string{"category"}),

		ChunksTransferredTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "katapult_chunks_transferred_total",
			Help: "Total number of chunks transferred",
		}),
	}

	reg.MustRegister(
		m.TransfersTotal,
		m.TransfersActive,
		m.TransferDurationSeconds,
		m.TransferBytesTotal,
		m.AgentHealthGauge,
		m.MoverBytesTransferred,
		m.MoverSpeedGauge,
		m.MoverErrorsTotal,
		m.ChunksTransferredTotal,
	)

	return m
}

// @cpt-begin:cpt-katapult-algo-observability-collect-metrics:p2:inst-update-metrics

// OnTransferStateChange records a state change on the transfer metrics.
func (m *Metrics) OnTransferStateChange(fromState, toState, strategy string) {
	m.TransfersTotal.WithLabelValues(toState, strategy).Inc()
	switch toState {
	case "transferring":
		m.TransfersActive.Inc()
	case "completed", "failed", "cancelled":
		m.TransfersActive.Dec()
	}
}

// OnProgressReport records bytes and speed from a progress report.
func (m *Metrics) OnProgressReport(bytesTransferred int64, speedBytesPerSec float64) {
	m.MoverBytesTransferred.Add(float64(bytesTransferred))
	m.MoverSpeedGauge.Set(speedBytesPerSec)
}

// OnAgentHealthChange adjusts the agent health gauge.
func (m *Metrics) OnAgentHealthChange(state string, delta int) {
	m.AgentHealthGauge.WithLabelValues(state).Add(float64(delta))
}

// OnTransferError records an error by category.
func (m *Metrics) OnTransferError(category string) {
	m.MoverErrorsTotal.WithLabelValues(category).Inc()
}

// @cpt-end:cpt-katapult-algo-observability-collect-metrics:p2:inst-update-metrics
