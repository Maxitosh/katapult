---
cypilot: true
type: project-rule
topic: conventions
generated-by: auto-config
version: 1.0
---

# Conventions


<!-- toc -->

- [Naming](#naming)
  - [File Naming](#file-naming)
  - [Type Naming](#type-naming)
  - [Receiver Naming](#receiver-naming)
  - [Constructor Pattern](#constructor-pattern)
  - [Proto Import Alias](#proto-import-alias)
- [Import Organization](#import-organization)
- [Constants and Configuration](#constants-and-configuration)
  - [Environment Variables](#environment-variables)
  - [Operational Constants](#operational-constants)
- [Function Signatures](#function-signatures)
  - [Context First](#context-first)
  - [Structured Logger Injection](#structured-logger-injection)

<!-- /toc -->

Naming, import ordering, and code style conventions observed in the Katapult codebase. Apply when writing or reviewing any Go code.

## Naming

### File Naming

Use snake_case for all Go source files. Multi-word names use underscores.

Evidence: `internal/agent/discovery.go`, `internal/store/postgres/agent.go`, `internal/testutil/memrepo.go`

### Type Naming

Use PascalCase nouns for types. Prefix with package context only when exported and ambiguous.

Evidence: `Agent`, `Service`, `JWTValidator`, `HealthEvaluator`, `PVCDiscoverer`, `ToolVersions`

### Receiver Naming

Use single-letter or short abbreviations for method receivers. Match the first letter of the type.

Evidence: `s *Service` (`registry/service.go`), `r *AgentRepo` (`store/postgres/agent.go`), `d *PVCDiscoverer` (`agent/discovery.go`), `h *HealthEvaluator` (`registry/health.go`), `v *JWTValidator` (`grpc/auth.go`)

### Constructor Pattern

Name constructors `New{Type}`. Accept dependencies as parameters, return pointer to struct.

Evidence: `NewService(repo, logger)` (`registry/service.go`), `NewClient(addr, opts)` (`agent/client.go`), `NewAgentRepo(pool)` (`store/postgres/agent.go`)

### Proto Import Alias

Alias generated proto packages as `pb`.

Evidence: `pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"` (`grpc/server.go`, `agent/client.go`)

## Import Organization

Group imports in three blocks separated by blank lines: standard library, third-party, internal packages.

Evidence: all Go files in `internal/`

## Constants and Configuration

### Environment Variables

Define env var keys as string literals at point of use, not as package-level constants. Use `config.EnvOrDefault(key, fallback)` for reading.

Evidence: `cmd/agent/main.go`, `cmd/controlplane/main.go`, `internal/config/env.go`

### Operational Constants

Define retry counts, timeouts, and operational limits as package-level `const` blocks near the top of the file where they are used.

Evidence: `registrationRetries`, `retryBaseDelay`, `jwtTokenPath` in `cmd/agent/main.go`

## Function Signatures

### Context First

Pass `context.Context` as the first parameter of any function that performs I/O or may be cancelled.

Evidence: all repository methods, service methods, gRPC handlers

### Structured Logger Injection

Inject `*slog.Logger` via constructor, not as a global. Use `slog.JSONHandler` for JSON output to stdout.

Evidence: `cmd/agent/main.go:30`, `cmd/controlplane/main.go:47`
