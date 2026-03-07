package transfer

import (
	"math"
	"math/rand/v2"
	"time"
)

// RetryDecision holds the result of the retry backoff calculation.
type RetryDecision struct {
	Action string        // "retry" or "exhausted"
	Delay  time.Duration // backoff delay (only meaningful when Action == "retry")
}

// ApplyRetryBackoff computes the retry decision based on current attempt count.
// @cpt-algo:cpt-katapult-algo-transfer-engine-retry-backoff:p1
// @cpt-dod:cpt-katapult-dod-transfer-engine-retry:p2
func ApplyRetryBackoff(retryCount, retryMax int, baseDelay, maxDelay time.Duration, jitterFactor float64) RetryDecision {
	// @cpt-begin:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-check-exhausted
	if retryCount >= retryMax {
		return RetryDecision{Action: "exhausted"}
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-check-exhausted

	// @cpt-begin:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-compute-delay
	delay := float64(baseDelay) * math.Pow(2, float64(retryCount))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-compute-delay

	// @cpt-begin:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-apply-jitter
	jitter := delay * jitterFactor * (2*rand.Float64() - 1) // random in [-jitterFactor, +jitterFactor]
	delay += jitter
	if delay < 0 {
		delay = 0
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-apply-jitter

	// @cpt-begin:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-return-retry
	return RetryDecision{
		Action: "retry",
		Delay:  time.Duration(delay),
	}
	// @cpt-end:cpt-katapult-algo-transfer-engine-retry-backoff:p1:inst-return-retry
}
