package registry

import (
	"context"
	"log/slog"
	"time"
)

// HealthEvaluator periodically checks agent heartbeat timestamps and
// marks stale agents as unhealthy or disconnected.
type HealthEvaluator struct {
	repo                AgentRepository
	unhealthyTimeout    time.Duration
	disconnectedTimeout time.Duration
	logger              *slog.Logger
}

// NewHealthEvaluator creates a new health evaluator.
// unhealthyTimeout: duration after last heartbeat before marking unhealthy (default 90s).
// disconnectedTimeout: duration after last heartbeat before marking disconnected (default 5m).
func NewHealthEvaluator(repo AgentRepository, unhealthyTimeout, disconnectedTimeout time.Duration, logger *slog.Logger) *HealthEvaluator {
	return &HealthEvaluator{
		repo:                repo,
		unhealthyTimeout:    unhealthyTimeout,
		disconnectedTimeout: disconnectedTimeout,
		logger:              logger,
	}
}

// Evaluate runs a single health evaluation cycle.
// Returns the number of agents marked unhealthy and disconnected.
func (h *HealthEvaluator) Evaluate(ctx context.Context) (unhealthy int, disconnected int, err error) {
	now := time.Now()

	unhealthy, err = h.repo.MarkUnhealthy(ctx, now.Add(-h.unhealthyTimeout))
	if err != nil {
		return 0, 0, err
	}
	if unhealthy > 0 {
		h.logger.Info("marked agents unhealthy", "count", unhealthy)
	}

	disconnected, err = h.repo.MarkDisconnected(ctx, now.Add(-h.disconnectedTimeout))
	if err != nil {
		return unhealthy, 0, err
	}
	if disconnected > 0 {
		h.logger.Info("marked agents disconnected", "count", disconnected)
	}

	return unhealthy, disconnected, nil
}

// RunLoop starts the health evaluation loop at the given interval.
// Blocks until the context is cancelled.
func (h *HealthEvaluator) RunLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, _, err := h.Evaluate(ctx); err != nil {
				h.logger.Error("health evaluation failed", "error", err)
			}
		}
	}
}
