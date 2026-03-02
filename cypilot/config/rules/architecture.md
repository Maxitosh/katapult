---
cypilot: true
type: project-rule
topic: architecture
generated-by: auto-config
version: 1.0
---

# Architecture


<!-- toc -->

- [Package Layers](#package-layers)
  - [Domain Layer Rules](#domain-layer-rules)
  - [Repository Interface Placement](#repository-interface-placement)
  - [Entry Point Responsibilities](#entry-point-responsibilities)
- [Source Layout](#source-layout)
- [Database Migrations](#database-migrations)
- [Critical Files](#critical-files)

<!-- /toc -->

Package structure, dependency rules, and module boundaries for the Katapult monolith. Apply when adding components, modifying package boundaries, or refactoring.

## Package Layers

Katapult follows Clean Architecture with four layers. Dependencies flow inward only.

| Layer | Packages | Depends on |
|-------|----------|------------|
| Domain | `internal/domain` | Nothing (no imports outside stdlib) |
| Service | `internal/registry` | Domain |
| Interface | `internal/grpc`, `internal/agent` | Service, Domain |
| Infrastructure | `internal/store/postgres`, `internal/config` | Domain |
| Entry | `cmd/agent`, `cmd/controlplane` | All layers (wiring only) |

Evidence: `internal/domain/agent.go` imports only stdlib + `github.com/google/uuid`

### Domain Layer Rules

Keep domain models free of infrastructure concerns. No database tags, no gRPC types, no external dependencies beyond stdlib and UUID.

Evidence: `internal/domain/agent.go` — pure domain model with state machine

### Repository Interface Placement

Define repository interfaces in the service package that uses them, not in the domain package.

Evidence: `AgentRepository` defined in `internal/registry/repository.go`, implemented in `internal/store/postgres/agent.go`

### Entry Point Responsibilities

`cmd/` packages wire dependencies and start servers. No business logic, no validation, no domain operations.

Evidence: `cmd/controlplane/main.go` — creates repo, service, evaluator, server, then serves

## Source Layout

```
katapult/
├── api/proto/         # gRPC definitions (generated — do not edit .pb.go files)
├── cmd/               # Binary entry points (wiring only)
│   ├── agent/         # Agent binary (runs on K8s nodes)
│   └── controlplane/  # Control plane binary (central server)
├── internal/          # All non-public packages
│   ├── domain/        # Domain models, state machine
│   ├── registry/      # Service layer, repository interface, validation, health
│   ├── grpc/          # gRPC server implementation, JWT auth
│   ├── agent/         # Agent-side client, PVC discovery, tools
│   ├── config/        # Environment configuration helpers
│   ├── store/postgres/ # PostgreSQL repository + migrations
│   └── testutil/      # In-memory repository for tests
```

## Database Migrations

Store SQL migrations in `internal/store/postgres/migrations/` with sequential numbering: `{NNN}_{description}.{up|down}.sql`.

Evidence: `001_create_agents.up.sql` through `004_add_jwt_namespace.up.sql`

## Critical Files

| File | Why it matters |
|------|---------------|
| `internal/domain/agent.go` | Core domain model and state machine — all business rules flow from here |
| `internal/registry/repository.go` | Repository interface contract — all persistence implementations must satisfy this |
| `internal/registry/service.go` | Primary business logic — registration, heartbeat, state transitions |
| `internal/grpc/server.go` | gRPC API surface — maps proto to domain, validates JWT namespace |
| `internal/grpc/auth.go` | JWT authentication interceptor — security boundary |
| `api/proto/agent/v1alpha1/agent_service.proto` | gRPC service definition — source of truth for API contract |
| `cmd/controlplane/main.go` | Control plane bootstrap — all dependency wiring |
| `cmd/agent/main.go` | Agent bootstrap — K8s integration, retry logic |
