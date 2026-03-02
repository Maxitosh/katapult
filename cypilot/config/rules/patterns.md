---
cypilot: true
type: project-rule
topic: patterns
generated-by: auto-config
version: 1.0
---

# Patterns


<!-- toc -->

- [Error Handling](#error-handling)
  - [Wrap with Context](#wrap-with-context)
  - [Not-Found Returns Nil](#not-found-returns-nil)
  - [gRPC Status Codes](#grpc-status-codes)
- [State Machine](#state-machine)
  - [Explicit Transition Validation](#explicit-transition-validation)
  - [Valid Transitions Table](#valid-transitions-table)
- [Retry and Resilience](#retry-and-resilience)
  - [Exponential Backoff](#exponential-backoff)
  - [Graceful Shutdown](#graceful-shutdown)
- [API Patterns](#api-patterns)
  - [JWT Bearer Token in Metadata](#jwt-bearer-token-in-metadata)
  - [Namespace Immutability](#namespace-immutability)
- [Data Access](#data-access)
  - [Transaction for Multi-Table Operations](#transaction-for-multi-table-operations)
  - [JSONB for Structured Fields](#jsonb-for-structured-fields)

<!-- /toc -->

Recurring implementation patterns in the Katapult codebase. Apply when implementing features or writing business logic.

## Error Handling

### Wrap with Context

Wrap errors using `fmt.Errorf("operation context: %w", err)`. Include the operation name and relevant identifiers.

Evidence: `registry/service.go` — `fmt.Errorf("upsert agent: %w", err)`, `fmt.Errorf("transition to healthy: %w", err)`

### Not-Found Returns Nil

Repository methods return `(nil, nil)` when an entity is not found — not an error. Callers check for nil result.

Evidence: `store/postgres/agent.go:GetAgentByID` returns `(nil, nil)` when `rows.Next()` is false

### gRPC Status Codes

Map domain errors to gRPC status codes at the server layer. Use `codes.InvalidArgument` for validation failures, `codes.NotFound` for missing entities, `codes.FailedPrecondition` for invalid state transitions.

Evidence: `grpc/server.go` — `status.Errorf(codes.InvalidArgument, ...)`, `status.Errorf(codes.NotFound, ...)`

## State Machine

### Explicit Transition Validation

Validate state transitions via a dedicated `TransitionTo()` method on the domain model before persisting. Never set state directly.

Evidence: `domain/agent.go:TransitionTo()` — validates allowed transitions, returns error for invalid ones

### Valid Transitions Table

```
Registering  → Healthy, Disconnected
Healthy      → Unhealthy, Registering
Unhealthy    → Healthy, Disconnected
Disconnected → Registering (only)
```

Evidence: `domain/agent.go` — `validTransitions` map

## Retry and Resilience

### Exponential Backoff

Use exponential backoff for retries: `baseDelay * (1 << attempt)`. Define `maxRetries` and `baseDelay` as constants near usage.

Evidence: `cmd/agent/main.go` — `retryBaseDelay = 2 * time.Second`, `registrationRetries = 10`; `agent/discovery.go` — configurable `MaxRetries` and `RetryBaseDelay`

### Graceful Shutdown

Use `signal.NotifyContext` with `SIGINT`/`SIGTERM` for cancellation. Check `ctx.Done()` in loops.

Evidence: `cmd/agent/main.go:36`, `cmd/controlplane/main.go:90`

## API Patterns

### JWT Bearer Token in Metadata

Pass JWT tokens via gRPC metadata key `"authorization"` with `"Bearer "` prefix. Extract with unary interceptor.

Evidence: `grpc/auth.go:UnaryAuthInterceptor`, `agent/client.go:Register` — `metadata.AppendToOutgoingContext`

### Namespace Immutability

Bind agent identity to JWT namespace at registration. Reject subsequent requests if namespace changes.

Evidence: `grpc/server.go:Heartbeat` — compares `claims.Kubernetes.Namespace` to `agent.JWTNamespace`

## Data Access

### Transaction for Multi-Table Operations

Use database transactions when an operation touches multiple tables (e.g., upserting agent + replacing PVCs).

Evidence: `store/postgres/agent.go:UpsertAgent` — `pool.Begin(ctx)` wrapping agent upsert + PVC delete/insert

### JSONB for Structured Fields

Store structured data (tool versions) as JSONB columns. Marshal/unmarshal with `encoding/json`.

Evidence: `store/postgres/agent.go` — `json.Marshal(a.Tools)`, `json.Unmarshal` in scan
