# Feature: Control Plane API & CLI

- [ ] `p1` - **ID**: `cpt-katapult-featstatus-api-cli`

<!-- toc -->

- [1. Feature Context](#1-feature-context)
  - [1.1 Overview](#11-overview)
  - [1.2 Purpose](#12-purpose)
  - [1.3 Actors](#13-actors)
  - [1.4 References](#14-references)
- [2. Actor Flows (CDSL)](#2-actor-flows-cdsl)
  - [Create Transfer via API](#create-transfer-via-api)
  - [Create Transfer via CLI](#create-transfer-via-cli)
  - [List Transfers via API](#list-transfers-via-api)
  - [Get Transfer Detail](#get-transfer-detail)
  - [Cancel Transfer via API](#cancel-transfer-via-api)
  - [List Agents and PVCs](#list-agents-and-pvcs)
  - [Create Transfer via CRD](#create-transfer-via-crd)
- [3. Processes / Business Logic (CDSL)](#3-processes-business-logic-cdsl)
  - [Validate Transfer Request](#validate-transfer-request)
  - [Resolve CLI Command](#resolve-cli-command)
  - [Reconcile VolumeTransfer CRD](#reconcile-volumetransfer-crd)
- [4. States (CDSL)](#4-states-cdsl)
  - [Not Applicable](#not-applicable)
- [5. Definitions of Done](#5-definitions-of-done)
  - [REST API Transfer Endpoints](#rest-api-transfer-endpoints)
  - [REST API Agent Endpoints](#rest-api-agent-endpoints)
  - [CLI Tool](#cli-tool)
  - [CRD Controller](#crd-controller)
  - [Input Validation and Error Responses](#input-validation-and-error-responses)
  - [Authentication and Authorization](#authentication-and-authorization)
- [6. Acceptance Criteria](#6-acceptance-criteria)
- [7. Additional Context](#7-additional-context)
  - [Performance Considerations](#performance-considerations)
  - [Security Considerations](#security-considerations)
  - [Reliability Considerations](#reliability-considerations)
  - [Data Considerations](#data-considerations)
  - [Integration Considerations](#integration-considerations)
  - [Observability Considerations](#observability-considerations)
  - [Usability Considerations](#usability-considerations)
  - [Compliance Considerations](#compliance-considerations)
  - [Maintainability Considerations](#maintainability-considerations)
  - [Known Limitations](#known-limitations)

<!-- /toc -->

## 1. Feature Context

- [ ] `p1` - `cpt-katapult-feature-api-cli`

### 1.1 Overview

Expose all control-plane functionality through a REST/gRPC API, a CLI tool (`katapult`), and Kubernetes VolumeTransfer CRDs for declarative operations. This feature serves as the primary interface layer for operators, ensuring every operation flows through the API per the API-First Design principle.

Problem: Operators need programmatic and terminal-based access to transfer operations, agent status, and PVC inventory without direct database or Kubernetes API interaction.
Primary value: Single consistent API surface consumed by CLI, Web UI, CRDs, and future integrations.
Key assumptions: Transfer Engine and Security features are available; agents are registered and discoverable.

### 1.2 Purpose

Satisfy the API, CLI, and CRD functional requirements by providing a unified interface layer that delegates to domain components for all operations.

- `cpt-katapult-fr-api`
- `cpt-katapult-fr-cli`
- `cpt-katapult-fr-crd`
- `cpt-katapult-principle-api-first`
- `cpt-katapult-constraint-k8s-only`
- `cpt-katapult-constraint-single-cp`

### 1.3 Actors

| Actor | Role in Feature |
|-------|-----------------|
| `cpt-katapult-actor-infra-engineer` | Primary user of API, CLI, and CRDs for transfer and agent operations |
| `cpt-katapult-actor-support-engineer` | Uses API and CLI for troubleshooting transfers and viewing agent status |

### 1.4 References

- **PRD**: [PRD.md](../PRD.md)
- **Design**: [DESIGN.md](../DESIGN.md)
- **Dependencies**: `cpt-katapult-feature-transfer-engine`, `cpt-katapult-feature-security`

## 2. Actor Flows (CDSL)

### Create Transfer via API

- [x] `p1` - **ID**: `cpt-katapult-flow-api-cli-create-transfer-api`

**Actors**:
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator submits valid transfer request; receives 201 with transfer ID and Pending status

**Error Scenarios**:
- Missing or invalid fields return 400 with descriptive validation errors
- Unauthenticated request returns 401
- Unauthorized role returns 403
- Source or destination PVC not found returns 404
- Source and destination are the same PVC returns 400

**Steps**:
1. [x] - `p1` - Operator sends POST /api/v1alpha1/transfers with source, destination, optional strategy override, allowOverwrite, and retry config - `inst-submit-request`
2. [x] - `p1` - API Server authenticates request via auth middleware (local credentials or OIDC token) - `inst-authenticate`
3. [x] - `p1` - API Server authorizes request via RBAC middleware (require operator role) - `inst-authorize`
4. [x] - `p1` - API Server validates request body using `cpt-katapult-algo-api-cli-validate-transfer-request` - `inst-validate-input`
5. [x] - `p1` - **IF** validation fails **RETURN** 400 with field-level error details - `inst-return-validation-error`
6. [x] - `p1` - API Server delegates to Transfer Orchestrator via `cpt-katapult-component-transfer-orchestrator` CreateTransfer - `inst-delegate-create`
7. [x] - `p1` - **RETURN** 201 Created with transfer ID, status=Pending, and resource URL - `inst-return-created`

### Create Transfer via CLI

- [x] `p1` - **ID**: `cpt-katapult-flow-api-cli-create-transfer-cli`

**Actors**:
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator runs `katapult transfer create`; CLI displays transfer ID and status

**Error Scenarios**:
- Missing required flags display usage help
- API error responses displayed with actionable messages

**Steps**:
1. [x] - `p1` - Operator runs `katapult transfer create --source-cluster <cluster> --source-pvc <ns/name> --dest-cluster <cluster> --dest-pvc <ns/name>` with optional flags --strategy, --allow-overwrite, --output - `inst-cli-invoke`
2. [x] - `p1` - CLI resolves API server address from kubeconfig context or --server flag - `inst-resolve-server`
3. [x] - `p1` - CLI constructs POST /api/v1alpha1/transfers request using `cpt-katapult-algo-api-cli-resolve-cli-command` - `inst-construct-request`
4. [x] - `p1` - CLI sends authenticated request to API Server (bearer token from kubeconfig or --token flag) - `inst-send-request`
5. [x] - `p1` - **IF** API returns error **RETURN** formatted error message with remediation hint - `inst-handle-error`
6. [x] - `p1` - CLI formats response per --output flag (table default, json, yaml) and displays transfer ID, status, source, destination - `inst-format-output`

### List Transfers via API

- [x] `p1` - **ID**: `cpt-katapult-flow-api-cli-list-transfers`

**Actors**:
- `cpt-katapult-actor-infra-engineer`
- `cpt-katapult-actor-support-engineer`

**Success Scenarios**:
- Operator queries transfers; receives paginated list with status summary

**Error Scenarios**:
- Invalid filter parameters return 400
- Unauthenticated request returns 401

**Steps**:
1. [x] - `p1` - Operator sends GET /api/v1alpha1/transfers with optional query parameters (status, cluster, limit, offset) or runs `katapult transfer list` - `inst-query-transfers`
2. [x] - `p1` - API Server authenticates and authorizes request (viewer or operator role) - `inst-auth-list`
3. [x] - `p1` - API Server delegates to Transfer Orchestrator ListTransfers with filter criteria - `inst-delegate-list`
4. [x] - `p1` - **RETURN** 200 with transfer list including id, status, source, destination, created_at, and pagination metadata - `inst-return-list`

### Get Transfer Detail

- [x] `p1` - **ID**: `cpt-katapult-flow-api-cli-get-transfer`

**Actors**:
- `cpt-katapult-actor-infra-engineer`
- `cpt-katapult-actor-support-engineer`

**Success Scenarios**:
- Operator queries single transfer; receives full detail with progress

**Error Scenarios**:
- Transfer not found returns 404

**Steps**:
1. [x] - `p1` - Operator sends GET /api/v1alpha1/transfers/{id} or runs `katapult transfer get <id>` - `inst-query-detail`
2. [x] - `p1` - API Server authenticates and authorizes request - `inst-auth-detail`
3. [x] - `p1` - API Server delegates to Transfer Orchestrator GetTransfer - `inst-delegate-get`
4. [x] - `p1` - **IF** transfer not found **RETURN** 404 with descriptive message - `inst-not-found`
5. [x] - `p1` - **RETURN** 200 with transfer detail including progress (bytes_transferred, total_bytes, speed, eta, chunks_completed, chunks_total), strategy, events - `inst-return-detail`

### Cancel Transfer via API

- [x] `p1` - **ID**: `cpt-katapult-flow-api-cli-cancel-transfer`

**Actors**:
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator cancels active transfer; receives confirmation with Cancelled status

**Error Scenarios**:
- Transfer not found returns 404
- Transfer already in terminal state returns 409
- Unauthorized returns 403

**Steps**:
1. [x] - `p1` - Operator sends DELETE /api/v1alpha1/transfers/{id} or runs `katapult transfer cancel <id>` - `inst-cancel-request`
2. [x] - `p1` - API Server authenticates and authorizes request (require operator role) - `inst-auth-cancel`
3. [x] - `p1` - API Server delegates to Transfer Orchestrator CancelTransfer - `inst-delegate-cancel`
4. [x] - `p1` - **IF** transfer not found **RETURN** 404 - `inst-cancel-not-found`
5. [x] - `p1` - **IF** transfer already in terminal state (Completed, Failed, Cancelled) **RETURN** 409 Conflict - `inst-cancel-conflict`
6. [x] - `p1` - **RETURN** 200 with updated transfer status=Cancelled - `inst-return-cancelled`

### List Agents and PVCs

- [x] `p1` - **ID**: `cpt-katapult-flow-api-cli-list-agents`

**Actors**:
- `cpt-katapult-actor-infra-engineer`
- `cpt-katapult-actor-support-engineer`

**Success Scenarios**:
- Operator queries agents; receives list grouped by cluster with health and PVC counts

**Error Scenarios**:
- Unauthenticated request returns 401

**Steps**:
1. [x] - `p1` - Operator sends GET /api/v1alpha1/agents or runs `katapult agent list` - `inst-query-agents`
2. [x] - `p1` - API Server authenticates and authorizes request (viewer or operator role) - `inst-auth-agents`
3. [x] - `p1` - API Server delegates to Agent Registry via `cpt-katapult-component-agent-registry` ListAgents - `inst-delegate-agents`
4. [x] - `p1` - **RETURN** 200 with agent list including id, cluster, node, status, last_heartbeat, pvc_count - `inst-return-agents`
5. [x] - `p1` - Operator optionally sends GET /api/v1alpha1/agents/{id}/pvcs or runs `katapult agent get <id>` for PVC detail - `inst-query-pvcs`
6. [x] - `p1` - **RETURN** 200 with PVC list including name, namespace, size, storage_class, node_affinity, bound_pv - `inst-return-pvcs`

### Create Transfer via CRD

- [x] `p1` - **ID**: `cpt-katapult-flow-api-cli-create-transfer-crd`

**Actors**:
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator applies VolumeTransfer CR; CRD Controller creates transfer and updates status

**Error Scenarios**:
- Invalid CRD spec triggers reconciliation error with status condition
- Transfer Orchestrator rejects request; status condition reflects the error

**Steps**:
1. [x] - `p1` - Operator applies VolumeTransfer CR (katapult.io/v1alpha1) via kubectl - `inst-apply-crd`
2. [x] - `p1` - CRD Controller via `cpt-katapult-component-crd-controller` receives reconciliation event - `inst-reconcile-event`
3. [x] - `p1` - CRD Controller extracts spec (source, destination, strategy, allowOverwrite, retry) - `inst-extract-spec`
4. [x] - `p1` - CRD Controller calls Transfer Orchestrator CreateTransfer using `cpt-katapult-algo-api-cli-reconcile-crd` - `inst-crd-create`
5. [x] - `p1` - **IF** Orchestrator returns error, CRD Controller sets status condition type=Ready, status=False, reason=CreateFailed - `inst-crd-error`
6. [x] - `p1` - CRD Controller updates status subresource with phase, progress, and conditions from transfer state - `inst-update-status`
7. [x] - `p1` - CRD Controller re-enqueues reconciliation to poll transfer state until terminal - `inst-requeue`

## 3. Processes / Business Logic (CDSL)

### Validate Transfer Request

- [x] `p1` - **ID**: `cpt-katapult-algo-api-cli-validate-transfer-request`

**Input**: Transfer creation request (source cluster, source PVC reference, destination cluster, destination PVC reference, optional strategy, allowOverwrite flag, retry config)

**Output**: Validation result (valid or list of field-level errors)

**Steps**:
1. [x] - `p1` - **IF** source cluster or source PVC reference is empty **RETURN** error "source cluster and PVC are required" - `inst-check-source`
2. [x] - `p1` - **IF** destination cluster or destination PVC reference is empty **RETURN** error "destination cluster and PVC are required" - `inst-check-dest`
3. [x] - `p1` - **IF** source and destination refer to the same cluster and PVC **RETURN** error "source and destination must differ" - `inst-check-same`
4. [x] - `p1` - **IF** strategy is provided AND strategy not in [stream, s3, direct] **RETURN** error "invalid strategy" - `inst-check-strategy`
5. [x] - `p1` - **IF** retry.maxAttempts is provided AND retry.maxAttempts < 1 **RETURN** error "maxAttempts must be >= 1" - `inst-check-retry`
6. [x] - `p1` - Query Agent Registry for source agent that owns the source PVC - `inst-lookup-source-agent`
7. [x] - `p1` - **IF** source agent not found or unhealthy **RETURN** error "source agent unavailable" - `inst-check-source-agent`
8. [x] - `p1` - Query Agent Registry for destination agent that owns the destination PVC - `inst-lookup-dest-agent`
9. [x] - `p1` - **IF** destination agent not found or unhealthy **RETURN** error "destination agent unavailable" - `inst-check-dest-agent`
10. [x] - `p1` - **IF** allowOverwrite is false, query destination agent to verify destination PVC is empty - `inst-check-dest-empty`
11. [x] - `p1` - **IF** destination PVC is not empty AND allowOverwrite is false **RETURN** error "destination PVC is not empty; set allowOverwrite to proceed" - `inst-dest-not-empty`
12. [x] - `p1` - **RETURN** valid - `inst-return-valid`

### Resolve CLI Command

- [x] `p2` - **ID**: `cpt-katapult-algo-api-cli-resolve-cli-command`

**Input**: CLI arguments (command, subcommand, flags, positional args)

**Output**: Constructed HTTP request or formatted error message

**Steps**:
1. [x] - `p1` - Parse command and subcommand from argv (transfer create|list|get|cancel, agent list|get) - `inst-parse-command`
2. [x] - `p1` - **IF** command is unknown **RETURN** usage help with available commands - `inst-unknown-command`
3. [x] - `p1` - Parse flags and positional arguments per command schema - `inst-parse-flags`
4. [x] - `p1` - **IF** required flags missing **RETURN** error with missing flag names and usage hint - `inst-missing-flags`
5. [x] - `p1` - Resolve API server address from --server flag, KATAPULT_SERVER env var, or kubeconfig current context - `inst-resolve-address`
6. [x] - `p1` - Resolve authentication token from --token flag, KATAPULT_TOKEN env var, or kubeconfig auth info - `inst-resolve-auth`
7. [x] - `p1` - Construct HTTP request (method, path, headers, body) for the resolved command - `inst-construct-http`
8. [x] - `p1` - **RETURN** constructed request - `inst-return-request`

### Reconcile VolumeTransfer CRD

- [x] `p1` - **ID**: `cpt-katapult-algo-api-cli-reconcile-crd`

**Input**: VolumeTransfer CR spec and current status

**Output**: Updated status subresource or requeue signal

**Steps**:
1. [x] - `p1` - Read VolumeTransfer CR spec (source, destination, strategy, allowOverwrite, retry) - `inst-read-spec`
2. [x] - `p1` - **IF** status.transferID is empty (new CR), call Transfer Orchestrator CreateTransfer with spec fields - `inst-create-transfer`
3. [x] - `p1` - **IF** CreateTransfer fails, set status condition type=Ready, status=False, reason=CreateFailed, message=error detail - `inst-create-failed`
4. [x] - `p1` - **IF** CreateTransfer succeeds, store transfer ID in status.transferID - `inst-store-id`
5. [x] - `p1` - **IF** status.transferID is present (existing CR), query Transfer Orchestrator GetTransfer for current state - `inst-poll-state`
6. [x] - `p1` - Update status.phase from transfer state (Pending, Validating, Transferring, Completed, Failed, Cancelled) - `inst-update-phase`
7. [x] - `p1` - Update status.progress from transfer progress (bytes_transferred, total_bytes, speed, chunks) - `inst-update-progress`
8. [x] - `p1` - Update status.conditions with appropriate Kubernetes condition entries - `inst-update-conditions`
9. [x] - `p1` - **IF** transfer is in terminal state (Completed, Failed, Cancelled) **RETURN** do not requeue - `inst-terminal`
10. [x] - `p1` - **RETURN** requeue after interval (default 5s) - `inst-requeue`

## 4. States (CDSL)

### Not Applicable

Not applicable — The transfer state machine is defined in DESIGN (`cpt-katapult-component-transfer-orchestrator`) and detailed in `cpt-katapult-feature-transfer-engine`. The CRD Controller mirrors transfer state into the VolumeTransfer status subresource but does not own or define state transitions. The API and CLI layers are stateless request handlers.

## 5. Definitions of Done

### REST API Transfer Endpoints

- [x] `p1` - **ID**: `cpt-katapult-dod-api-cli-rest-transfer-endpoints`

The system **MUST** expose REST endpoints for transfer lifecycle operations: POST /api/v1alpha1/transfers (create), GET /api/v1alpha1/transfers (list with filtering), GET /api/v1alpha1/transfers/{id} (detail with progress), DELETE /api/v1alpha1/transfers/{id} (cancel), and GET /api/v1alpha1/transfers/{id}/events (event timeline).

**Implements**:
- `cpt-katapult-flow-api-cli-create-transfer-api`
- `cpt-katapult-flow-api-cli-list-transfers`
- `cpt-katapult-flow-api-cli-get-transfer`
- `cpt-katapult-flow-api-cli-cancel-transfer`

**Covers (PRD)**:
- `cpt-katapult-fr-api`

**Covers (DESIGN)**:
- `cpt-katapult-principle-api-first`
- `cpt-katapult-component-api-server`
- `cpt-katapult-interface-control-plane-api`

**Touches**:
- API: `POST /api/v1alpha1/transfers`, `GET /api/v1alpha1/transfers`, `GET /api/v1alpha1/transfers/{id}`, `DELETE /api/v1alpha1/transfers/{id}`, `GET /api/v1alpha1/transfers/{id}/events`
- Entities: Transfer

### REST API Agent Endpoints

- [x] `p1` - **ID**: `cpt-katapult-dod-api-cli-rest-agent-endpoints`

The system **MUST** expose REST endpoints for agent and PVC queries: GET /api/v1alpha1/agents (list agents), GET /api/v1alpha1/agents/{id} (agent detail with PVCs), GET /api/v1alpha1/agents/{id}/pvcs (PVC inventory), and GET /api/v1alpha1/clusters (cluster list).

**Implements**:
- `cpt-katapult-flow-api-cli-list-agents`

**Covers (PRD)**:
- `cpt-katapult-fr-api`

**Covers (DESIGN)**:
- `cpt-katapult-component-api-server`
- `cpt-katapult-component-agent-registry`
- `cpt-katapult-interface-control-plane-api`

**Touches**:
- API: `GET /api/v1alpha1/agents`, `GET /api/v1alpha1/agents/{id}`, `GET /api/v1alpha1/agents/{id}/pvcs`, `GET /api/v1alpha1/clusters`
- Entities: Agent, PVCInfo

### CLI Tool

- [x] `p1` - **ID**: `cpt-katapult-dod-api-cli-cli-tool`

The system **MUST** provide a Go CLI binary (`katapult`) that supports all transfer operations (create, list, get, cancel) and agent queries (list, get) by consuming the same REST API as the Web UI. The CLI supports table, JSON, and YAML output formats.

**Implements**:
- `cpt-katapult-flow-api-cli-create-transfer-cli`
- `cpt-katapult-flow-api-cli-list-transfers`
- `cpt-katapult-flow-api-cli-get-transfer`
- `cpt-katapult-flow-api-cli-cancel-transfer`
- `cpt-katapult-flow-api-cli-list-agents`
- `cpt-katapult-algo-api-cli-resolve-cli-command`

**Covers (PRD)**:
- `cpt-katapult-fr-cli`

**Covers (DESIGN)**:
- `cpt-katapult-principle-api-first`
- `cpt-katapult-component-api-server`

**Touches**:
- Entities: Transfer, Agent, PVCInfo

### CRD Controller

- [x] `p1` - **ID**: `cpt-katapult-dod-api-cli-crd-controller`

The system **MUST** provide a Kubebuilder-based CRD Controller that reconciles VolumeTransfer custom resources (katapult.io/v1alpha1). The controller creates transfers via the Transfer Orchestrator, mirrors transfer state into the CRD status subresource, and triggers cancellation on CR deletion.

**Implements**:
- `cpt-katapult-flow-api-cli-create-transfer-crd`
- `cpt-katapult-algo-api-cli-reconcile-crd`

**Covers (PRD)**:
- `cpt-katapult-fr-crd`

**Covers (DESIGN)**:
- `cpt-katapult-constraint-k8s-only`
- `cpt-katapult-component-crd-controller`

**Touches**:
- API: CRD VolumeTransfer (katapult.io/v1alpha1)
- Entities: Transfer

### Input Validation and Error Responses

- [x] `p1` - **ID**: `cpt-katapult-dod-api-cli-input-validation`

The system **MUST** validate all API inputs before processing and return descriptive 400 responses with field-level error details. Validation covers required fields, value constraints, PVC existence via Agent Registry, destination safety checks, and strategy validity.

**Implements**:
- `cpt-katapult-algo-api-cli-validate-transfer-request`

**Covers (PRD)**:
- `cpt-katapult-fr-api`

**Covers (DESIGN)**:
- `cpt-katapult-component-api-server`

**Touches**:
- Entities: Transfer, Agent, PVCInfo

### Authentication and Authorization

- [x] `p1` - **ID**: `cpt-katapult-dod-api-cli-auth`

The system **MUST** enforce authentication (local credentials or OIDC) and role-based authorization (operator and viewer roles) on all API endpoints. Operator role is required for create and cancel operations. Viewer role is sufficient for list and get operations. Unauthenticated requests receive 401; unauthorized requests receive 403.

**Implements**:
- `cpt-katapult-flow-api-cli-create-transfer-api`
- `cpt-katapult-flow-api-cli-list-transfers`
- `cpt-katapult-flow-api-cli-get-transfer`
- `cpt-katapult-flow-api-cli-cancel-transfer`
- `cpt-katapult-flow-api-cli-list-agents`

**Covers (PRD)**:
- `cpt-katapult-fr-api`

**Covers (DESIGN)**:
- `cpt-katapult-component-api-server`
- `cpt-katapult-constraint-single-cp`

**Touches**:
- Entities: Transfer, Agent

## 6. Acceptance Criteria

- [ ] POST /api/v1alpha1/transfers with valid input returns 201 with transfer ID and Pending status
- [ ] POST /api/v1alpha1/transfers with missing required fields returns 400 with field-level errors
- [ ] GET /api/v1alpha1/transfers returns paginated list of transfers with correct status values
- [ ] GET /api/v1alpha1/transfers/{id} returns full transfer detail including progress metrics
- [ ] DELETE /api/v1alpha1/transfers/{id} on active transfer returns 200 with Cancelled status
- [ ] DELETE /api/v1alpha1/transfers/{id} on terminal transfer returns 409 Conflict
- [ ] GET /api/v1alpha1/agents returns agent list grouped by cluster with health status
- [ ] GET /api/v1alpha1/agents/{id}/pvcs returns PVC inventory for the specified agent
- [ ] `katapult transfer create` with valid flags creates transfer and displays result in table format
- [ ] `katapult transfer list --output json` returns JSON array of transfers
- [ ] `katapult agent list` displays agents with status and PVC counts
- [ ] Applying VolumeTransfer CR triggers transfer creation and status reflects Pending
- [ ] Deleting VolumeTransfer CR with active transfer triggers cancellation
- [ ] VolumeTransfer status.phase mirrors transfer state through terminal state
- [ ] Unauthenticated API requests receive 401 response
- [ ] Viewer-role user attempting DELETE /transfers/{id} receives 403 response
- [ ] All API error responses include machine-readable error code and human-readable message

## 7. Additional Context

### Performance Considerations

Not applicable in detail for this feature. The API layer is a thin delegation layer to domain components (`cpt-katapult-component-transfer-orchestrator`, `cpt-katapult-component-agent-registry`). Performance-critical paths (transfer execution, data streaming, progress reporting) are handled by `cpt-katapult-feature-transfer-engine` and `cpt-katapult-component-agent-runtime`. The API Server adds minimal overhead (request parsing, auth middleware, response serialization).

NFR applicability:
- `cpt-katapult-nfr-cp-availability` — API Server runs as single replica per `cpt-katapult-constraint-single-cp`; active transfers continue during API downtime per agent autonomy
- `cpt-katapult-nfr-cp-recovery` — API Server is stateless; recovers by reconnecting to PostgreSQL and accepting agent re-registrations

### Security Considerations

Authentication and authorization are integrated at the API layer but implementation details (OIDC provider configuration, RBAC rule storage, token validation) belong to `cpt-katapult-feature-security`. This feature ensures:
- All endpoints pass through auth middleware before reaching handlers
- Role checks are enforced per endpoint (operator for mutations, viewer for reads)
- CLI authenticates using kubeconfig credentials or explicit token
- CRD Controller operates with in-cluster ServiceAccount (no user auth needed)

### Reliability Considerations

- API Server returns descriptive errors for all failure paths (validation, not found, conflict, auth)
- CLI surfaces API errors with actionable remediation hints
- CRD Controller handles reconciliation errors gracefully via status conditions and requeue
- Timeout handling delegated to HTTP server configuration and gRPC-Gateway defaults

### Data Considerations

The API layer does not access the database directly. All data operations are delegated to domain components:
- Transfer CRUD via `cpt-katapult-component-transfer-orchestrator`
- Agent/PVC queries via `cpt-katapult-component-agent-registry`

Input validation at the API boundary ensures only well-formed requests reach domain components.

### Integration Considerations

- REST API uses gRPC-Gateway for REST-to-gRPC translation per `cpt-katapult-interface-control-plane-api`
- CLI consumes the same REST API as the Web UI (no privileged access)
- CRD Controller uses controller-runtime (Kubebuilder) for Kubernetes API integration
- API documented via OpenAPI spec auto-generated from gRPC service definitions

### Observability Considerations

- API Server exposes Prometheus `/metrics` endpoint (request counts, latency histograms, error rates)
- Structured JSON logging with per-request correlation IDs
- CLI logs errors to stderr with --verbose flag for debug output
- Detailed observability implementation in `cpt-katapult-feature-observability`

### Usability Considerations

Not applicable — this feature provides programmatic API and CLI interfaces. The Web UI is a separate feature (`cpt-katapult-feature-web-ui`) that consumes the API defined here.

### Compliance Considerations

Not applicable — no regulatory requirements at the API layer. Data handling compliance is addressed at the domain and infrastructure layers.

### Maintainability Considerations

- Clear separation between API handlers (thin delegation), domain logic (Transfer Orchestrator, Agent Registry), and infrastructure (PostgreSQL, Kubernetes API)
- CLI commands mirror API structure for consistency and discoverability
- CRD spec mirrors API request structure to minimize translation logic
- gRPC-Gateway auto-generates REST handlers from protobuf definitions, reducing maintenance burden

### Known Limitations

- API version is v1alpha1 (unstable); breaking changes expected before v1
- Single control plane replica limits API availability (no HA in v1)
- No GraphQL API or SDK/client library generation in scope
