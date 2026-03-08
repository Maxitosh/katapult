---
cypilot: true
type: project-rule
topic: testing
generated-by: auto-config
version: 1.0
---

# Testing


<!-- toc -->

- [Test Structure](#test-structure)
  - [Table-Driven Tests](#table-driven-tests)
  - [Test File Placement](#test-file-placement)
  - [Test Helper Functions](#test-helper-functions)
- [Unit Tests](#unit-tests)
  - [In-Memory Repository](#in-memory-repository)
  - [Construct Fixtures in Code](#construct-fixtures-in-code)
- [Integration Tests](#integration-tests)
  - [Build Tag Separation](#build-tag-separation)
  - [Testcontainers for Database](#testcontainers-for-database)
- [Assertions](#assertions)
  - [Standard Library Only](#standard-library-only)

<!-- /toc -->

Test conventions, fixture patterns, and integration test setup for the Katapult codebase. Apply when writing or running tests.

## Test Structure

### Table-Driven Tests

Use table-driven tests with `t.Run()` sub-tests. Define test cases as slices of anonymous structs with `name` and relevant fields.

Evidence: `domain/agent_test.go` — state transition tests, `registry/validation_test.go` — tar version tests, `grpc/auth_test.go` — JWT validation tests

### Test File Placement

Place test files in the same package as the code under test. Use `_test.go` suffix.

Evidence: `registry/service_test.go` tests `registry.Service`, `domain/agent_test.go` tests `domain.Agent`

### Test Helper Functions

Extract shared test setup into unexported helper functions: `setupTestServer()`, `contextWithClaims()`, `newTestAgent()`.

Evidence: `grpc/testutil_test.go` — `setupTestServer`, `contextWithClaims`; `registry/service_test.go` — `newTestAgent`

## Unit Tests

### In-Memory Repository

Use `testutil.MemRepo` for unit tests of service logic. It implements `AgentRepository` with thread-safe in-memory storage.

Evidence: `registry/service_test.go` — `testutil.NewMemRepo()` used in all service tests

### Construct Fixtures in Code

Build test fixtures directly in test functions. Do not use fixture files on disk.

Evidence: all test files construct `domain.Agent{}` structs inline

## Integration Tests

### Build Tag Separation

Tag integration tests with `//go:build integration` to exclude them from `go test ./...`. Run explicitly with `-tags integration`.

Evidence: `store/postgres/agent_test.go:1` — `//go:build integration`

### Testcontainers for Database

Use `testcontainers-go` with PostgreSQL image for database integration tests. Run migrations before tests.

Evidence: `store/postgres/agent_test.go` — `postgres.RunContainer()`, migration execution in `TestMain`

## Assertions

### Assertion Libraries

Use `testing.T` methods (`t.Errorf`, `t.Fatalf`, `t.Helper()`) as the primary assertion approach for unit tests and synchronous assertions. All test files use `if got != want { t.Errorf(...) }` pattern.

For async assertions in integration and e2e tests, use Gomega via `gomega.NewWithT(t)` — this bridges to `testing.T` without requiring Ginkgo. Use `Eventually` for polling assertions and `Consistently` for stability checks.

Evidence: controller integration tests use `gomega.NewWithT(t)` with `Eventually` for reconciliation polling; e2e tests use `Eventually` for transfer completion and pod readiness waits.

### Polling Constants

Use standardized polling constants from `internal/testutil/polling.go` instead of hardcoded durations:

- `DefaultTimeout` (30s) / `DefaultPollingInterval` (250ms) — envtest controller reconciliation
- `ShortTimeout` (10s) — quick assertions (finalizer, condition checks)
- `E2ETimeout` (5m) / `E2EPollingInterval` (3s) — e2e transfer completion
- `PortForwardTimeout` (15s) — port-forward/nodeport readiness
