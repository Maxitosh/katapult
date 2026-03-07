package transfer

import (
	"testing"
	"time"
)

func TestApplyRetryBackoff_Exhausted(t *testing.T) {
	decision := ApplyRetryBackoff(3, 3, 5*time.Second, 5*time.Minute, 0.3)
	if decision.Action != "exhausted" {
		t.Fatalf("expected exhausted, got %s", decision.Action)
	}

	decision = ApplyRetryBackoff(5, 3, 5*time.Second, 5*time.Minute, 0.3)
	if decision.Action != "exhausted" {
		t.Fatalf("expected exhausted when retryCount > retryMax, got %s", decision.Action)
	}
}

func TestApplyRetryBackoff_Retry(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		retryMax   int
		baseDelay  time.Duration
		maxDelay   time.Duration
		jitter     float64
	}{
		{"first retry", 0, 3, 5 * time.Second, 5 * time.Minute, 0.3},
		{"second retry", 1, 3, 5 * time.Second, 5 * time.Minute, 0.3},
		{"third retry", 2, 3, 5 * time.Second, 5 * time.Minute, 0.3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := ApplyRetryBackoff(tt.retryCount, tt.retryMax, tt.baseDelay, tt.maxDelay, tt.jitter)
			if decision.Action != "retry" {
				t.Fatalf("expected retry, got %s", decision.Action)
			}
			if decision.Delay <= 0 {
				t.Fatalf("expected positive delay, got %v", decision.Delay)
			}
		})
	}
}

func TestApplyRetryBackoff_DelayCap(t *testing.T) {
	maxDelay := 30 * time.Second
	// With retryCount=10, base=5s: 5*2^10 = 5120s >> 30s, should be capped.
	// Run multiple times to account for jitter.
	for range 20 {
		decision := ApplyRetryBackoff(10, 20, 5*time.Second, maxDelay, 0.3)
		if decision.Action != "retry" {
			t.Fatal("expected retry")
		}
		// With jitter factor 0.3, max possible = 30s * 1.3 = 39s.
		if decision.Delay > time.Duration(float64(maxDelay)*1.31) {
			t.Fatalf("delay %v exceeds expected cap with jitter", decision.Delay)
		}
	}
}

func TestApplyRetryBackoff_ExponentialGrowth(t *testing.T) {
	// Without jitter, delays should grow exponentially.
	d0 := ApplyRetryBackoff(0, 5, 5*time.Second, 5*time.Minute, 0)
	d1 := ApplyRetryBackoff(1, 5, 5*time.Second, 5*time.Minute, 0)
	d2 := ApplyRetryBackoff(2, 5, 5*time.Second, 5*time.Minute, 0)

	if d0.Delay != 5*time.Second {
		t.Fatalf("expected 5s for retry 0, got %v", d0.Delay)
	}
	if d1.Delay != 10*time.Second {
		t.Fatalf("expected 10s for retry 1, got %v", d1.Delay)
	}
	if d2.Delay != 20*time.Second {
		t.Fatalf("expected 20s for retry 2, got %v", d2.Delay)
	}
}
