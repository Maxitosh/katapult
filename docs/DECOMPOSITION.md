# Decomposition: Katapult

<!-- toc -->

- [1. Overview](#1-overview)
- [2. Entries](#2-entries)
  - [2.1 Agent System ⏳ HIGH](#21-agent-system-high)
  - [2.2 Transfer Engine ⏳ HIGH](#22-transfer-engine-high)
  - [2.3 Security & Credentials ⏳ HIGH](#23-security-credentials-high)
  - [2.4 Control Plane API & CLI ⏳ HIGH](#24-control-plane-api-cli-high)
  - [2.5 Observability & Monitoring ⏳ MEDIUM](#25-observability-monitoring-medium)
  - [2.6 Web UI ⏳ MEDIUM](#26-web-ui-medium)
  - [2.7 Integration Tests ⏳ MEDIUM](#27-integration-tests-medium)
- [3. Feature Dependencies](#3-feature-dependencies)

<!-- /toc -->

## 1. Overview

Katapult's DESIGN is decomposed into seven features organized around the hub-and-spoke architecture. The decomposition groups design elements by functional cohesion — the agent subsystem (spoke), transfer orchestration (hub core), security layer, API surface, observability, web UI, and test infrastructure — while minimizing cross-feature dependencies.

**Decomposition Strategy**:
- Features grouped by functional cohesion around architectural boundaries
- Foundation feature (Agent System) enables all others — must be implemented first
- Dependencies flow upward: agents → transfers → API/security → observability/UI
- Each feature covers specific components, sequences, and data entities from DESIGN
- 100% coverage of all DESIGN elements verified
- Mutual exclusivity enforced: each design element assigned to exactly one feature

## 2. Entries

**Overall implementation status:**

- [ ] `p1` - **ID**: `cpt-katapult-status-overall`

### 2.1 [Agent System](feature-agent-system/) ⏳ HIGH

- [ ] `p1` - **ID**: `cpt-katapult-feature-agent-system`

- **Purpose**: Enable agents to register with the control plane, report health, and discover PVCs on their Kubernetes clusters — forming the spoke side of the hub-and-spoke architecture.

- **Depends On**: None

- **Scope**:
  - Agent gRPC-based auto-registration with cluster identity and capabilities
  - Periodic heartbeat and health monitoring
  - PVC discovery with PV binding resolution, size, storage class, and node affinity
  - Agent-to-control-plane mTLS bootstrap authentication
  - PVC boundary enforcement via namespace and label filters
  - Agent inventory persistence

- **Out of scope**:
  - Multi-cluster agent federation
  - Agent auto-upgrade or self-update
  - Non-Kubernetes platforms

- **Requirements Covered**:

  - [ ] `p1` - `cpt-katapult-fr-agent-registration`
  - [ ] `p1` - `cpt-katapult-fr-agent-health`
  - [ ] `p1` - `cpt-katapult-fr-pvc-discovery`
  - [ ] `p1` - `cpt-katapult-fr-agent-auth`
  - [ ] `p1` - `cpt-katapult-fr-pvc-boundary`

- **Design Principles Covered**:

  - [ ] `p1` - `cpt-katapult-principle-agent-autonomy`
  - [ ] `p1` - `cpt-katapult-principle-outbound-only`

- **Design Constraints Covered**:

  - [ ] `p1` - `cpt-katapult-constraint-k8s-only`
  - [ ] `p1` - `cpt-katapult-constraint-agent-tools`

- **Domain Model Entities**:
  - Agent
  - PVCInfo

- **Design Components**:

  - [ ] `p1` - `cpt-katapult-component-agent-runtime`
  - [ ] `p1` - `cpt-katapult-component-agent-registry`

- **API**:
  - gRPC AgentService.Register
  - gRPC AgentService.Heartbeat

- **Sequences**:

  - `cpt-katapult-seq-agent-registration`

- **Data**:

  - `cpt-katapult-dbtable-agents`
  - `cpt-katapult-dbtable-agent-pvcs`

### 2.2 [Transfer Engine](feature-transfer-engine/) ⏳ HIGH

- [ ] `p1` - **ID**: `cpt-katapult-feature-transfer-engine`

- **Purpose**: Orchestrate volume transfers end-to-end — selecting strategies, coordinating source and destination agents, managing the transfer state machine, and handling failures with retry and resumption.

- **Depends On**: `cpt-katapult-feature-agent-system`

- **Scope**:
  - Transfer initiation with source/destination PVC validation
  - Transfer cancellation with safe destination cleanup
  - Automatic strategy selection (intra-cluster streaming vs cross-cluster S3-staged)
  - Resumable cross-cluster transfers via chunked S3 staging
  - Destination safety pre-flight checks (empty or overwrite-permitted)
  - Retry with exponential backoff and jitter
  - Transfer autonomy — agents continue if control plane is temporarily unavailable
  - Resource cleanup on completion, failure, or cancellation

- **Out of scope**:
  - Transfer scheduling or queueing
  - Bandwidth throttling
  - Snapshot-based transfer strategies

- **Requirements Covered**:

  - [ ] `p1` - `cpt-katapult-fr-initiate-transfer`
  - [ ] `p1` - `cpt-katapult-fr-cancel-transfer`
  - [ ] `p1` - `cpt-katapult-fr-strategy-selection`
  - [ ] `p1` - `cpt-katapult-fr-resumable-transfer`
  - [ ] `p1` - `cpt-katapult-fr-destination-safety`
  - [ ] `p2` - `cpt-katapult-fr-retry-backoff`
  - [ ] `p1` - `cpt-katapult-fr-transfer-autonomy`
  - [ ] `p1` - `cpt-katapult-fr-resource-cleanup`

- **Design Principles Covered**:

  - [ ] `p1` - `cpt-katapult-principle-agent-autonomy`

- **Design Constraints Covered**:

  - [ ] `p1` - `cpt-katapult-constraint-s3-required`
  - [ ] `p2` - `cpt-katapult-constraint-single-cp`

- **Domain Model Entities**:
  - Transfer
  - Chunk
  - Credential

- **Design Components**:

  - [ ] `p1` - `cpt-katapult-component-transfer-orchestrator`

- **API**:
  - gRPC AgentService.StreamCommands
  - gRPC AgentService.ReportProgress

- **Sequences**:

  - `cpt-katapult-seq-intra-transfer`
  - `cpt-katapult-seq-cross-transfer`
  - `cpt-katapult-seq-cancel-transfer`

- **Data**:

  - `cpt-katapult-dbtable-transfers`
  - `cpt-katapult-dbtable-transfer-events`

### 2.3 [Security & Credentials](feature-security/) ⏳ HIGH

- [ ] `p1` - **ID**: `cpt-katapult-feature-security`

- **Purpose**: Manage encryption, ephemeral credentials, and user authentication and authorization to ensure all data transfers and API access are secure by default.

- **Depends On**: `cpt-katapult-feature-agent-system`

- **Scope**:
  - In-transit encryption by default (mTLS for gRPC, S3 SSE for staged data)
  - Ephemeral credential generation and automatic rotation per transfer
  - User authentication via OIDC/SSO
  - Role-based authorization for API and CLI access

- **Out of scope**:
  - Multi-tenant isolation
  - External key management service (KMS) integration
  - Audit logging of authentication events (covered by Observability)

- **Requirements Covered**:

  - [ ] `p1` - `cpt-katapult-fr-encryption-default`
  - [ ] `p1` - `cpt-katapult-fr-ephemeral-credentials`
  - [ ] `p1` - `cpt-katapult-fr-user-auth`
  - [ ] `p1` - `cpt-katapult-fr-user-authz`

- **Design Principles Covered**:

  - [ ] `p1` - `cpt-katapult-principle-encryption-default`
  - [ ] `p1` - `cpt-katapult-principle-ephemeral-credentials`

- **Design Constraints Covered**:

  None specific to this feature.

- **Domain Model Entities**:
  - Credential

- **Design Components**:

  - [ ] `p1` - `cpt-katapult-component-credential-manager`

- **API**:
  - Internal: credential issuance (Go interface, not network-exposed)
  - Internal: auth middleware for API Server

- **Sequences**:

  None unique to this feature.

- **Data**:

  None unique; credentials are ephemeral and not persisted.

### 2.4 [Control Plane API & CLI](feature-api-cli/) ⏳ HIGH

- [ ] `p1` - **ID**: `cpt-katapult-feature-api-cli`

- **Purpose**: Expose all control-plane functionality through a REST/gRPC API, a CLI tool, and Kubernetes CRDs for declarative operations — serving as the primary interface layer for operators.

- **Depends On**: `cpt-katapult-feature-transfer-engine`, `cpt-katapult-feature-security`

- **Scope**:
  - REST API endpoints for transfers, agents, PVCs, and clusters
  - gRPC-Gateway for REST-to-gRPC translation
  - CLI tool (katapult) for operator workflows
  - CRD controller for declarative VolumeTransfer resources
  - Input validation and error responses

- **Out of scope**:
  - GraphQL API
  - SDK or client library generation
  - API versioning beyond v1alpha1

- **Requirements Covered**:

  - [ ] `p1` - `cpt-katapult-fr-api`
  - [ ] `p1` - `cpt-katapult-fr-cli`
  - [ ] `p1` - `cpt-katapult-fr-crd`

- **Design Principles Covered**:

  - [ ] `p1` - `cpt-katapult-principle-api-first`

- **Design Constraints Covered**:

  - [ ] `p1` - `cpt-katapult-constraint-k8s-only`
  - [ ] `p2` - `cpt-katapult-constraint-single-cp`

- **Domain Model Entities**:
  - Transfer
  - Agent
  - PVCInfo

- **Design Components**:

  - [ ] `p1` - `cpt-katapult-component-api-server`
  - [ ] `p1` - `cpt-katapult-component-crd-controller`

- **API**:
  - POST /api/v1alpha1/transfers
  - GET /api/v1alpha1/transfers
  - GET /api/v1alpha1/transfers/{id}
  - DELETE /api/v1alpha1/transfers/{id}
  - GET /api/v1alpha1/agents
  - GET /api/v1alpha1/agents/{id}
  - GET /api/v1alpha1/agents/{id}/pvcs
  - GET /api/v1alpha1/clusters
  - CRD: VolumeTransfer (katapult.io/v1alpha1)
  - CLI: katapult transfer create/list/get/cancel
  - CLI: katapult agent list/get

- **Sequences**:

  None unique to this feature.

- **Data**:

  None unique; uses transfers, agents, and agent-pvcs tables.

- **Interface**:

  - [ ] `p1` - `cpt-katapult-interface-control-plane-api`

- **NFR Covered**:

  - [ ] `p2` - `cpt-katapult-nfr-cp-availability`
  - [ ] `p2` - `cpt-katapult-nfr-cp-recovery`

### 2.5 [Observability & Monitoring](feature-observability/) ⏳ MEDIUM

- [ ] `p2` - **ID**: `cpt-katapult-feature-observability`

- **Purpose**: Provide real-time transfer progress, structured logging, Prometheus metrics, transfer history, and actionable error messages so operators can monitor and troubleshoot transfers.

- **Depends On**: `cpt-katapult-feature-transfer-engine`

- **Scope**:
  - Real-time progress streaming via SSE (bytes transferred, speed, ETA)
  - Transfer event timeline and audit history
  - Prometheus metrics for throughput, duration, success/failure rates, agent health
  - Structured logging across control plane and agents
  - Actionable error messages with remediation hints

- **Out of scope**:
  - Distributed tracing (OpenTelemetry)
  - Alerting rules or alert manager integration
  - Pre-built Grafana dashboards

- **Requirements Covered**:

  - [ ] `p1` - `cpt-katapult-fr-realtime-progress`
  - [ ] `p2` - `cpt-katapult-fr-transfer-history`
  - [ ] `p2` - `cpt-katapult-fr-metrics-logging`
  - [ ] `p1` - `cpt-katapult-fr-actionable-errors`

- **Design Principles Covered**:

  None unique to this feature.

- **Design Constraints Covered**:

  None unique to this feature.

- **Domain Model Entities**:
  - Transfer
  - Agent

- **Design Components**:

  Cross-cutting; implemented within `cpt-katapult-component-api-server`, `cpt-katapult-component-transfer-orchestrator`, and `cpt-katapult-component-agent-runtime`.

- **API**:
  - GET /api/v1alpha1/transfers/{id}/progress (SSE stream)
  - GET /api/v1alpha1/transfers/{id}/events
  - GET /metrics (Prometheus)

- **Sequences**:

  None unique to this feature.

- **Data**:

  Uses `cpt-katapult-dbtable-transfer-events` for event timeline.

- **NFR Covered**:

  - [ ] `p2` - `cpt-katapult-nfr-progress-latency`
  - [ ] `p1` - `cpt-katapult-nfr-throughput-intra`
  - [ ] `p1` - `cpt-katapult-nfr-throughput-cross`
  - [ ] `p1` - `cpt-katapult-nfr-bounded-failure`
  - [ ] `p1` - `cpt-katapult-nfr-initiation-time`

### 2.6 [Web UI](feature-web-ui/) ⏳ MEDIUM

- [ ] `p2` - **ID**: `cpt-katapult-feature-web-ui`

- **Purpose**: Provide a web dashboard for initiating transfers, monitoring agent status, browsing PVCs, and validating transfer parameters through a visual interface.

- **Depends On**: `cpt-katapult-feature-api-cli`, `cpt-katapult-feature-observability`

- **Scope**:
  - Transfer management dashboard (create, monitor, cancel)
  - Agent status and PVC browser
  - Transfer parameter validation UI
  - Documentation and inline help

- **Out of scope**:
  - Mobile-responsive design
  - Role-based UI customization
  - Real-time push notifications (uses polling/SSE from API)

- **Requirements Covered**:

  - [ ] `p1` - `cpt-katapult-fr-web-ui-transfers`
  - [ ] `p2` - `cpt-katapult-fr-web-ui-agents`
  - [ ] `p2` - `cpt-katapult-fr-web-ui-validation`
  - [ ] `p2` - `cpt-katapult-fr-documentation`

- **Design Principles Covered**:

  None unique to this feature.

- **Design Constraints Covered**:

  None unique to this feature.

- **Domain Model Entities**:
  - Transfer
  - Agent
  - PVCInfo

- **Design Components**:

  - [ ] `p2` - `cpt-katapult-component-web-ui`

- **API**:
  - Consumes REST API (no new endpoints)

- **Sequences**:

  None unique to this feature.

- **Data**:

  None unique; reads via REST API.

### 2.7 Integration Tests ⏳ MEDIUM

- [ ] `p2` - **ID**: `cpt-katapult-feature-integration-tests`

- **Purpose**: Provide three-tier test infrastructure (controller, component, E2E) that validates Katapult across CRD reconciliation, cross-component interactions, and full transfer workflows in ephemeral environments.

- **Depends On**: `cpt-katapult-feature-agent-system`, `cpt-katapult-feature-transfer-engine`, `cpt-katapult-feature-api-cli`

- **Scope**:
  - Envtest-based CRD Controller reconciliation tests (VolumeTransfer lifecycle, status updates, finalizers)
  - Testcontainers-based component integration tests (gRPC registration/heartbeat, API→orchestrator lifecycle, MinIO-backed S3 transfers)
  - Kind-based E2E tests (full stack deployment via NodePort service access, intra-cluster streaming transfer, cross-cluster S3-staged transfer, cancellation/cleanup, CLI execution)
  - Build tag separation (`//go:build integration`, `//go:build e2e`)
  - Shared test helpers (fixture builders, assertion utilities, container lifecycle)

- **Out of scope**:
  - Performance/load testing
  - Chaos engineering / fault injection framework
  - UI end-to-end testing (Cypress/Playwright)

- **Requirements Covered**:

  - [ ] `p1` - `cpt-katapult-fr-controller-integration-tests`
  - [ ] `p1` - `cpt-katapult-fr-component-integration-tests`
  - [ ] `p2` - `cpt-katapult-fr-e2e-tests`

- **Design Principles Covered**:

  None unique to this feature.

- **Design Constraints Covered**:

  None unique to this feature.

- **Domain Model Entities**:
  - Transfer (test fixtures)
  - Agent (test fixtures)

- **Design Components**:

  - [ ] `p2` - `cpt-katapult-component-test-harness`

- **API**:
  - Validates all existing API endpoints and gRPC services (no new endpoints)

- **Sequences**:

  None unique; validates existing sequences (`cpt-katapult-seq-intra-transfer`, `cpt-katapult-seq-cross-transfer`, `cpt-katapult-seq-agent-registration`).

- **Data**:

  None unique; tests create and verify data in existing tables via test fixtures.

- **NFR Covered**:

  - [ ] `p2` - `cpt-katapult-nfr-test-execution-time`

---

## 3. Feature Dependencies

```text
cpt-katapult-feature-agent-system
    ↓
    ├─→ cpt-katapult-feature-transfer-engine
    │       ↓
    │       ├─→ cpt-katapult-feature-observability ─┐
    │       │                                        ├─→ cpt-katapult-feature-web-ui
    │       └─→ cpt-katapult-feature-api-cli ───────┘
    │               ↓
    │               └─→ cpt-katapult-feature-integration-tests
    │                       ↑               ↑
    └─→ cpt-katapult-feature-security ─→ cpt-katapult-feature-api-cli
```

**Dependency Rationale**:

- `cpt-katapult-feature-transfer-engine` requires `cpt-katapult-feature-agent-system`: transfers depend on registered agents that report PVC inventory and execute data movement commands
- `cpt-katapult-feature-security` requires `cpt-katapult-feature-agent-system`: agent mTLS bootstrap is the foundation for credential management; user auth builds on the agent communication channel
- `cpt-katapult-feature-api-cli` requires `cpt-katapult-feature-transfer-engine`: API and CLI expose transfer operations that the orchestrator implements
- `cpt-katapult-feature-api-cli` requires `cpt-katapult-feature-security`: API endpoints require authentication and authorization middleware
- `cpt-katapult-feature-observability` requires `cpt-katapult-feature-transfer-engine`: progress streaming and metrics observe transfer state machine transitions
- `cpt-katapult-feature-web-ui` requires `cpt-katapult-feature-api-cli`: the UI is a client of the REST API
- `cpt-katapult-feature-web-ui` requires `cpt-katapult-feature-observability`: the UI displays real-time progress and transfer history
- `cpt-katapult-feature-transfer-engine` and `cpt-katapult-feature-security` are independent of each other and can be developed in parallel after Agent System
- `cpt-katapult-feature-observability` and `cpt-katapult-feature-api-cli` can be developed in parallel after Transfer Engine (API also needs Security)
- `cpt-katapult-feature-integration-tests` requires `cpt-katapult-feature-agent-system`, `cpt-katapult-feature-transfer-engine`, and `cpt-katapult-feature-api-cli`: tests validate agent registration, transfer lifecycle, CRD reconciliation, and API endpoints — all three features must exist to test against
