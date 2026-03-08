# Feature: Web UI

- [ ] `p1` - **ID**: `cpt-katapult-featstatus-web-ui`

<!-- toc -->

- [1. Feature Context](#1-feature-context)
  - [1.1 Overview](#11-overview)
  - [1.2 Purpose](#12-purpose)
  - [1.3 Actors](#13-actors)
  - [1.4 References](#14-references)
- [2. Actor Flows (CDSL)](#2-actor-flows-cdsl)
  - [Create Transfer via UI](#create-transfer-via-ui)
  - [Monitor Transfer](#monitor-transfer)
  - [Cancel Transfer via UI](#cancel-transfer-via-ui)
  - [Browse Agents and PVCs](#browse-agents-and-pvcs)
- [3. Processes / Business Logic (CDSL)](#3-processes-business-logic-cdsl)
  - [Validate Transfer Parameters](#validate-transfer-parameters)
  - [Render Transfer Progress](#render-transfer-progress)
- [4. States (CDSL)](#4-states-cdsl)
- [5. Definitions of Done](#5-definitions-of-done)
  - [Transfer Management Dashboard](#transfer-management-dashboard)
  - [Agent Overview and PVC Browser](#agent-overview-and-pvc-browser)
  - [Client-Side Validation and Mistake Prevention](#client-side-validation-and-mistake-prevention)
  - [Inline Help and Documentation](#inline-help-and-documentation)
- [6. Acceptance Criteria](#6-acceptance-criteria)
- [7. Non-Applicable Domains](#7-non-applicable-domains)

<!-- /toc -->

## 1. Feature Context

- [ ] `p2` - `cpt-katapult-feature-web-ui`

### 1.1 Overview

Web dashboard for initiating volume transfers, monitoring agent status, browsing PVC inventory, and validating transfer parameters through a guided visual interface. The Web UI is a React+TypeScript single-page application that consumes the REST API exclusively — it has no direct access to Kubernetes API, S3, or PostgreSQL.

Problem: Support engineers need to perform volume transfers but lack Kubernetes CLI expertise; a guided browser-based interface reduces misconfiguration risk and accelerates onboarding.
Primary value: Self-service transfer operations without CLI or kubectl knowledge, with built-in mistake prevention.
Key assumptions: The REST API and SSE progress endpoint are available; users authenticate via the API Server's OIDC/local auth mechanism.

### 1.2 Purpose

Provide a browser-based interface for managing volume transfers end-to-end, monitoring agent health, and browsing PVC inventory with client-side validation to prevent common mistakes.

This feature addresses:
- `cpt-katapult-fr-web-ui-transfers` — transfer creation, monitoring, and cancellation via guided UI
- `cpt-katapult-fr-web-ui-agents` — agent and PVC browsing grouped by cluster
- `cpt-katapult-fr-web-ui-validation` — client-side mistake prevention (same source/dest, size mismatch, confirmation dialogs)
- `cpt-katapult-fr-documentation` — inline help and onboarding guide integration

Design principles applied:
- `cpt-katapult-principle-api-first` — Web UI is a pure API client with no privileged access

### 1.3 Actors

| Actor | Role in Feature |
|-------|-----------------|
| `cpt-katapult-actor-support-engineer` | Primary user; uses guided transfer creation workflow, monitors transfers, browses agents |
| `cpt-katapult-actor-infra-engineer` | Power user; uses agent diagnostics view, reviews transfer event timelines, validates agent capabilities |

### 1.4 References

- **PRD**: [PRD.md](../PRD.md)
- **Design**: [DESIGN.md](../DESIGN.md) — `cpt-katapult-component-web-ui`
- **Dependencies**: `cpt-katapult-feature-api-cli` (REST API), `cpt-katapult-feature-observability` (SSE progress, transfer events)

## 2. Actor Flows (CDSL)

### Create Transfer via UI

- [ ] `p1` - **ID**: `cpt-katapult-flow-web-ui-create-transfer`

**Actors**:
- `cpt-katapult-actor-support-engineer`
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- User creates a transfer using the guided cluster → node → PVC workflow and monitors progress until completion

**Error Scenarios**:
- Source and destination PVC are the same — UI blocks submission
- Destination PVC is smaller than source — UI shows warning, requires confirmation
- API returns validation error — UI displays actionable error message
- SSE connection drops — UI reconnects and resumes progress display

**Steps**:
1. [ ] - `p1` - User navigates to transfer creation page - `inst-navigate-create`
2. [ ] - `p1` - API: GET /api/v1alpha1/clusters (fetch available clusters) - `inst-fetch-clusters`
3. [ ] - `p1` - User selects source cluster from dropdown - `inst-select-source-cluster`
4. [ ] - `p1` - API: GET /api/v1alpha1/agents?cluster={sourceCluster} (fetch agents for source cluster) - `inst-fetch-source-agents`
5. [ ] - `p1` - User selects source agent (node) from dropdown - `inst-select-source-agent`
6. [ ] - `p1` - API: GET /api/v1alpha1/agents/{agentId}/pvcs (fetch PVCs on source agent) - `inst-fetch-source-pvcs`
7. [ ] - `p1` - User selects source PVC from dropdown - `inst-select-source-pvc`
8. [ ] - `p1` - User selects destination cluster, agent, and PVC using the same cascading dropdown pattern - `inst-select-destination`
9. [ ] - `p1` - Algorithm: validate transfer parameters using `cpt-katapult-algo-web-ui-validate-transfer-params` - `inst-run-validation`
10. [ ] - `p1` - **IF** validation errors exist - `inst-check-validation`
    1. [ ] - `p1` - Display validation errors inline and block submission - `inst-show-validation-errors`
    2. [ ] - `p1` - **RETURN** user corrects inputs - `inst-return-correct`
11. [ ] - `p1` - **IF** validation warnings exist (size mismatch) - `inst-check-warnings`
    1. [ ] - `p1` - Display warning with explanation and require explicit confirmation - `inst-show-warning`
12. [ ] - `p1` - UI displays confirmation dialog with transfer summary (source, destination, auto-selected strategy with explanation) - `inst-show-confirmation`
13. [ ] - `p1` - User confirms transfer creation - `inst-confirm-create`
14. [ ] - `p1` - API: POST /api/v1alpha1/transfers (body: source PVC ref, destination PVC ref) - `inst-api-create-transfer`
15. [ ] - `p1` - **IF** API returns error - `inst-check-api-error`
    1. [ ] - `p1` - Display actionable error message from API response - `inst-show-api-error`
    2. [ ] - `p1` - **RETURN** error state - `inst-return-api-error`
16. [ ] - `p1` - UI redirects to transfer detail view with real-time progress - `inst-redirect-detail`
17. [ ] - `p1` - **RETURN** transfer created successfully - `inst-return-success`

### Monitor Transfer

- [ ] `p1` - **ID**: `cpt-katapult-flow-web-ui-monitor-transfer`

**Actors**:
- `cpt-katapult-actor-support-engineer`
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- User views active transfers dashboard with live progress and drills into a specific transfer for detailed event timeline

**Error Scenarios**:
- SSE connection drops — UI reconnects automatically and resumes progress updates
- Transfer enters Failed state — UI displays enriched error with remediation hints

**Steps**:
1. [ ] - `p1` - User navigates to transfers dashboard - `inst-navigate-dashboard`
2. [ ] - `p1` - API: GET /api/v1alpha1/transfers (fetch transfer list with status filters) - `inst-fetch-transfers`
3. [ ] - `p1` - UI renders transfer list with status badges, progress bars, and speed indicators - `inst-render-list`
4. [ ] - `p1` - User selects a transfer to view details - `inst-select-transfer`
5. [ ] - `p1` - API: GET /api/v1alpha1/transfers/{id} (fetch transfer detail) - `inst-fetch-detail`
6. [ ] - `p1` - Algorithm: render transfer progress using `cpt-katapult-algo-web-ui-render-progress` - `inst-render-progress`
7. [ ] - `p1` - API: GET /api/v1alpha1/transfers/{id}/progress (subscribe to SSE stream) - `inst-subscribe-sse`
8. [ ] - `p1` - UI updates progress bar, speed, ETA, and chunk progress in real time from SSE events - `inst-update-realtime`
9. [ ] - `p2` - API: GET /api/v1alpha1/transfers/{id}/events (fetch event timeline) - `inst-fetch-events`
10. [ ] - `p2` - UI renders chronological event timeline with timestamps and event types - `inst-render-timeline`
11. [ ] - `p1` - **IF** SSE connection drops - `inst-check-sse-drop`
    1. [ ] - `p1` - UI reconnects to SSE endpoint with exponential backoff - `inst-reconnect-sse`
12. [ ] - `p1` - **RETURN** transfer detail displayed with live progress - `inst-return-monitoring`

### Cancel Transfer via UI

- [ ] `p1` - **ID**: `cpt-katapult-flow-web-ui-cancel-transfer`

**Actors**:
- `cpt-katapult-actor-support-engineer`
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- User cancels an active transfer after confirmation, and the UI reflects the Cancelled state

**Error Scenarios**:
- Transfer already in terminal state — UI displays message that cancellation is not possible
- API returns error on cancel — UI displays error message

**Steps**:
1. [ ] - `p1` - User clicks cancel button on an active transfer - `inst-click-cancel`
2. [ ] - `p1` - UI displays confirmation dialog with transfer summary and warning about partial data on destination - `inst-show-cancel-confirm`
3. [ ] - `p1` - User confirms cancellation - `inst-confirm-cancel`
4. [ ] - `p1` - API: DELETE /api/v1alpha1/transfers/{id} (cancel transfer) - `inst-api-cancel`
5. [ ] - `p1` - **IF** API returns error - `inst-check-cancel-error`
    1. [ ] - `p1` - Display error message (transfer already completed/failed/cancelled) - `inst-show-cancel-error`
    2. [ ] - `p1` - **RETURN** error state - `inst-return-cancel-error`
6. [ ] - `p1` - UI updates transfer status to Cancelled - `inst-update-cancelled`
7. [ ] - `p1` - **RETURN** transfer cancelled - `inst-return-cancelled`

### Browse Agents and PVCs

- [ ] `p2` - **ID**: `cpt-katapult-flow-web-ui-browse-agents`

**Actors**:
- `cpt-katapult-actor-support-engineer`
- `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- User views agent inventory grouped by cluster and drills into PVC details per agent

**Error Scenarios**:
- No agents registered — UI displays empty state with guidance on deploying agents
- Agent marked unhealthy — UI highlights with visual indicator and last heartbeat timestamp

**Steps**:
1. [ ] - `p2` - User navigates to agents page - `inst-navigate-agents`
2. [ ] - `p2` - API: GET /api/v1alpha1/agents (fetch all agents) - `inst-fetch-agents`
3. [ ] - `p2` - API: GET /api/v1alpha1/clusters (fetch cluster list for grouping) - `inst-fetch-clusters-agents`
4. [ ] - `p2` - UI renders agents grouped by cluster with health status, node name, last heartbeat, and available movers - `inst-render-agents`
5. [ ] - `p2` - User selects an agent to view PVC details - `inst-select-agent`
6. [ ] - `p2` - API: GET /api/v1alpha1/agents/{agentId}/pvcs (fetch PVCs) - `inst-fetch-agent-pvcs`
7. [ ] - `p2` - UI renders PVC list with name, namespace, size, storage class, and node affinity - `inst-render-pvcs`
8. [ ] - `p2` - **RETURN** agent and PVC details displayed - `inst-return-agents`

## 3. Processes / Business Logic (CDSL)

### Validate Transfer Parameters

- [ ] `p1` - **ID**: `cpt-katapult-algo-web-ui-validate-transfer-params`

**Input**: Source PVC reference (cluster, agent, PVC), destination PVC reference (cluster, agent, PVC)

**Output**: Validation result (errors list, warnings list, strategy explanation)

**Steps**:
1. [ ] - `p1` - **IF** source cluster + agent + PVC equals destination cluster + agent + PVC **RETURN** error "Source and destination must be different" - `inst-check-same-pvc`
2. [ ] - `p1` - **IF** source PVC and destination PVC are on the same agent **RETURN** error "Source and destination must be on different nodes" - `inst-check-same-node`
3. [ ] - `p1` - **IF** destination PVC size < source PVC size - `inst-check-size-mismatch`
   1. [ ] - `p1` - Add warning "Destination PVC ({destSize}) is smaller than source PVC ({sourceSize}); transfer may fail if data exceeds destination capacity" - `inst-add-size-warning`
4. [ ] - `p1` - **IF** source cluster equals destination cluster - `inst-check-same-cluster`
   1. [ ] - `p1` - Set strategy explanation to "Intra-cluster streaming transfer selected: source and destination are on the same cluster, enabling direct tar+zstd pipe via stunnel for fastest throughput" - `inst-explain-intra`
5. [ ] - `p1` - **ELSE** - `inst-else-cross-cluster`
   1. [ ] - `p1` - Set strategy explanation to "Cross-cluster S3-staged transfer selected: source and destination are on different clusters, requiring chunked upload/download via S3 for resumable transfer" - `inst-explain-cross`
6. [ ] - `p1` - **RETURN** validation result (errors, warnings, strategy explanation) - `inst-return-validation`

### Render Transfer Progress

- [ ] `p2` - **ID**: `cpt-katapult-algo-web-ui-render-progress`

**Input**: SSE event stream (bytes_transferred, total_bytes, speed_bps, chunks_completed, chunks_total, phase)

**Output**: Rendered progress state (percentage, progress bar width, human-readable speed, ETA)

**Steps**:
1. [ ] - `p2` - Parse SSE event payload into progress fields - `inst-parse-sse`
2. [ ] - `p2` - Compute percentage as (bytes_transferred / total_bytes) * 100 - `inst-compute-percentage`
3. [ ] - `p2` - **IF** speed_bps > 0 - `inst-check-speed`
   1. [ ] - `p2` - Compute ETA as (total_bytes - bytes_transferred) / speed_bps - `inst-compute-eta`
   2. [ ] - `p2` - Format speed as human-readable (B/s, KB/s, MB/s, GB/s) - `inst-format-speed`
4. [ ] - `p2` - **ELSE** - `inst-else-no-speed`
   1. [ ] - `p2` - Set ETA to "Calculating..." - `inst-eta-calculating`
5. [ ] - `p2` - **IF** chunks_total > 0 (S3-staged transfer) - `inst-check-chunks`
   1. [ ] - `p2` - Render chunk progress indicator ({chunks_completed}/{chunks_total}) - `inst-render-chunks`
6. [ ] - `p2` - **RETURN** rendered progress state - `inst-return-progress`

## 4. States (CDSL)

Not applicable. The Web UI is a stateless client-side application. All entity lifecycle state (Transfer, Agent) is managed by the API Server and Transfer Orchestrator. The UI renders state from API responses but does not own or manage state transitions.

## 5. Definitions of Done

### Transfer Management Dashboard

- [ ] `p1` - **ID**: `cpt-katapult-dod-web-ui-transfer-dashboard`

The system **MUST** provide a web-based transfer management dashboard that supports guided transfer creation (cluster → node → PVC cascading selection), transfer list with status filtering and progress indicators, transfer detail view with real-time SSE progress updates, and transfer cancellation with confirmation dialog.

**Implements**:
- `cpt-katapult-flow-web-ui-create-transfer`
- `cpt-katapult-flow-web-ui-monitor-transfer`
- `cpt-katapult-flow-web-ui-cancel-transfer`
- `cpt-katapult-algo-web-ui-render-progress`

**Covers (PRD)**:
- `cpt-katapult-fr-web-ui-transfers`

**Covers (DESIGN)**:
- `cpt-katapult-component-web-ui`
- `cpt-katapult-component-api-server`

### Agent Overview and PVC Browser

- [ ] `p2` - **ID**: `cpt-katapult-dod-web-ui-agent-browser`

The system **MUST** provide a web-based agent overview page that displays registered agents grouped by cluster with health status, node name, last heartbeat timestamp, and available movers, and allows drilling into PVC details per agent showing PVC name, namespace, size, storage class, and node affinity.

**Implements**:
- `cpt-katapult-flow-web-ui-browse-agents`

**Covers (PRD)**:
- `cpt-katapult-fr-web-ui-agents`

**Covers (DESIGN)**:
- `cpt-katapult-component-web-ui`

### Client-Side Validation and Mistake Prevention

- [ ] `p1` - **ID**: `cpt-katapult-dod-web-ui-validation`

The system **MUST** implement client-side validation that prevents selecting the same PVC as both source and destination, displays a warning when destination PVC capacity is smaller than source, requires confirmation dialogs for transfer creation and cancellation, and shows an explanation of why a particular transfer strategy was auto-selected.

**Implements**:
- `cpt-katapult-algo-web-ui-validate-transfer-params`

**Covers (PRD)**:
- `cpt-katapult-fr-web-ui-validation`

**Covers (DESIGN)**:
- `cpt-katapult-component-web-ui`

### Inline Help and Documentation

- [ ] `p2` - **ID**: `cpt-katapult-dod-web-ui-documentation`

The system **MUST** provide inline help within the Web UI including tooltips for transfer parameters, strategy explanation in the confirmation dialog, empty state guidance when no agents are registered, and links to the support engineer onboarding guide and API documentation.

**Implements**:
- `cpt-katapult-flow-web-ui-create-transfer` (strategy explanation in confirmation)
- `cpt-katapult-flow-web-ui-browse-agents` (empty state guidance)

**Covers (PRD)**:
- `cpt-katapult-fr-documentation`

**Covers (DESIGN)**:
- `cpt-katapult-component-web-ui`

## 6. Acceptance Criteria

- [ ] Support engineer can create a transfer using guided cluster → node → PVC dropdowns without CLI knowledge
- [ ] Transfer progress updates appear within 5 seconds of agent report via SSE streaming
- [ ] UI blocks submission when source and destination PVC are identical
- [ ] UI displays warning and requires confirmation when destination PVC is smaller than source
- [ ] Confirmation dialog shows auto-selected strategy with human-readable explanation
- [ ] Transfer cancellation requires explicit confirmation dialog
- [ ] Agent overview displays agents grouped by cluster with health status indicators
- [ ] PVC browser shows PVC name, namespace, size, storage class, and node affinity
- [ ] UI reconnects to SSE endpoint automatically on connection drop
- [ ] Empty agent list displays guidance on deploying the agent DaemonSet

## 7. Non-Applicable Domains

**States**: Not applicable because the Web UI is a stateless client-side application. All entity lifecycle state (Transfer, Agent) is owned by the API Server and Transfer Orchestrator. The UI renders state from API responses but does not manage state transitions.

**Performance (PERF)**: Not applicable because the Web UI has no performance-critical server-side paths. Rendering performance is standard for a React SPA. Data transfer throughput and API response times are governed by the API Server and Agent Runtime features, not the UI.

**Security — Authentication/Authorization (SEC)**: Not applicable at the UI level because the API Server enforces authentication (OIDC/local) and RBAC (operator/viewer roles). The UI delegates all auth to the API layer and does not implement its own access control. Input validation in the UI is for user convenience, not security — the API re-validates all inputs server-side.

**Reliability — Fault Tolerance (REL)**: Not applicable because the UI has no server-side components requiring circuit breakers, retries, or bulkheads. The only resilience behavior is SSE reconnection on connection drop, which is documented in the Monitor Transfer flow.

**Data Persistence (DATA)**: Not applicable because the UI does not persist data. All data is read from and written to the REST API. No local storage, caching layers, or database access exists in the UI.

**Database Operations (INT-DB)**: Not applicable because the UI has no direct database access. All data flows through the REST API.

**Operations (OPS)**: Not applicable because the Web UI is served as static assets by the API Server. No separate deployment, configuration management, or health checks are needed beyond the API Server's own operational concerns.

**Compliance (COMPL)**: Not applicable because the Web UI does not handle regulated data directly. Data handling compliance (PII, retention) is enforced at the API and database layers.
