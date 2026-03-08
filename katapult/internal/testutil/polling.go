package testutil

import "time"

// Standard polling constants for Katapult test infrastructure.
// Use these instead of hardcoded durations across test files.

// DefaultTimeout is the standard timeout for envtest controller reconciliation assertions.
const DefaultTimeout = 30 * time.Second

// DefaultPollingInterval is the standard polling tick for async assertions.
const DefaultPollingInterval = 250 * time.Millisecond

// ShortTimeout is for quick assertions like finalizer or condition checks.
const ShortTimeout = 10 * time.Second

// E2ETimeout is for e2e transfer completion waits.
const E2ETimeout = 5 * time.Minute

// E2EPollingInterval is for e2e status check polling.
const E2EPollingInterval = 3 * time.Second

// PortForwardTimeout is for port-forward or nodeport readiness checks.
const PortForwardTimeout = 15 * time.Second
