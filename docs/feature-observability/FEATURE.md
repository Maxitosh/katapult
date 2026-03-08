# Feature: Observability & Monitoring

- [x] `p2` - **ID**: `cpt-katapult-featstatus-observability`

<!-- toc -->

- [1. Feature Context](#1-feature-context)
  - [1.1 Overview](#11-overview)
  - [1.2 Purpose](#12-purpose)
  - [1.3 Configuration Parameters](#13-configuration-parameters)
  - [1.4 Actors](#14-actors)
  - [1.5 References](#15-references)
- [2. Actor Flows (CDSL)](#2-actor-flows-cdsl)
  - [Stream Transfer Progress](#stream-transfer-progress)
  - [Query Transfer Events](#query-transfer-events)
  - [Scrape Prometheus Metrics](#scrape-prometheus-metrics)
- [3. Processes / Business Logic (CDSL)](#3-processes-business-logic-cdsl)
  - [Emit Transfer Progress](#emit-transfer-progress)
  - [Record Transfer Event](#record-transfer-event)
  - [Enrich Error Context](#enrich-error-context)
  - [Collect Prometheus Metrics](#collect-prometheus-metrics)
- [4. States (CDSL)](#4-states-cdsl)
  - [Not Applicable](#not-applicable)
- [5. Definitions of Done](#5-definitions-of-done)
  - [Real-Time Progress Streaming](#real-time-progress-streaming)
  - [Transfer Event History](#transfer-event-history)
  - [Metrics and Structured Logging](#metrics-and-structured-logging)
  - [Actionable Error Messages](#actionable-error-messages)
- [6. Acceptance Criteria](#6-acceptance-criteria)
- [7. Non-Applicable Domains](#7-non-applicable-domains)

<!-- /toc -->

## 1. Feature Context

- [x] `p2` - `cpt-katapult-feature-observability`

### 1.1 Overview

Real-time transfer progress streaming, structured logging, Prometheus metrics, transfer event history, and actionable error messages for the Katapult control plane and agents. This feature provides operators with full visibility into transfer operations — from live byte-level progress to historical audit trails and system health metrics.

Problem: The current manual transfer process has zero progress visibility during multi-hour transfers. Operators cannot detect stalls, estimate completion, or diagnose failures without SSH access.
Primary value: Enables operators to monitor all transfers in real time, troubleshoot failures with actionable error messages, and integrate with existing Prometheus/Grafana monitoring stacks.
Key assumptions: Operators have access to the control plane API (via CLI, Web UI, or direct API calls). Prometheus is available for metric scraping.

### 1.2 Purpose

Enable operators to observe transfer progress in real time, audit transfer history, collect system metrics, and diagnose failures through enriched error messages — providing the operational visibility layer that supports self-service transfers by support engineers.

This feature addresses:
- `cpt-katapult-fr-realtime-progress` — real-time transfer progress including bytes, speed, ETA, and chunk-level progress
- `cpt-katapult-fr-transfer-history` — transfer audit log with initiator, timing, strategy, outcome, and data volume
- `cpt-katapult-fr-metrics-logging` — Prometheus metrics and structured JSON logs with correlation IDs
- `cpt-katapult-fr-actionable-errors` — failure messages with contextual remediation hints

NFR coverage:
- `cpt-katapult-nfr-progress-latency` — progress updates visible in UI within 5 seconds of agent report
- `cpt-katapult-nfr-throughput-intra` — metrics must capture intra-cluster throughput for verification
- `cpt-katapult-nfr-throughput-cross` — metrics must capture cross-cluster throughput for verification
- `cpt-katapult-nfr-bounded-failure` — event timeline must record failure point for bounded-failure verification
- `cpt-katapult-nfr-initiation-time` — metrics must capture initiation-to-first-byte latency

### 1.3 Configuration Parameters

| Parameter | Default | Description | Referenced In |
|-----------|---------|-------------|---------------|
| SSE keepalive interval | 15s | Interval for SSE keepalive pings to prevent connection drops | `cpt-katapult-flow-observability-stream-progress` |
| Progress report interval | 1s | Minimum interval between agent progress reports via gRPC | `cpt-katapult-algo-observability-emit-progress` |
| Metrics endpoint path | /metrics | Prometheus scrape endpoint path | `cpt-katapult-algo-observability-collect-metrics` |
| Log format | json | Structured log output format (json) | `cpt-katapult-algo-observability-collect-metrics` |
| Event retention | 90d | Duration to retain transfer events in database | `cpt-katapult-algo-observability-record-event` |

### 1.4 Actors

| Actor | Role in Feature |
|-------|-----------------|
| `cpt-katapult-actor-infra-engineer` | Subscribes to progress streams, queries transfer events, reviews metrics and logs, diagnoses failures |
| `cpt-katapult-actor-agent` | Reports transfer progress via gRPC streaming, enriches error context with local system information |
| `cpt-katapult-actor-control-plane` | Processes agent progress, persists events, exposes SSE streams and Prometheus metrics |

### 1.5 References

- **PRD**: [PRD.md](../PRD.md)
- **Design**: [DESIGN.md](../DESIGN.md)
- **Dependencies**: `cpt-katapult-feature-transfer-engine`

## 2. Actor Flows (CDSL)

### Stream Transfer Progress

- [x] `p1` - **ID**: `cpt-katapult-flow-observability-stream-progress`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator opens SSE connection and receives live progress updates (bytes, speed, ETA) for an active transfer
- Operator sees chunk-level progress for cross-cluster S3-staged transfers
- Progress stream completes cleanly when transfer reaches a terminal state

**Error Scenarios**:
- SSE connection drops and operator reconnects, receiving current state immediately
- Transfer ID does not exist and API returns 404
- Transfer is already in a terminal state and API returns final state snapshot then closes stream

**Steps**:
1. [x] - `p1` - Operator requests progress stream for a transfer via API or UI - `inst-request-stream`
2. [x] - `p1` - API: GET /api/v1alpha1/transfers/{id}/progress (SSE) - `inst-sse-connect`
3. [x] - `p1` - **IF** transfer ID does not exist **RETURN** 404 Not Found - `inst-check-transfer-exists`
4. [x] - `p1` - **IF** transfer is in a terminal state (Completed, Failed, Cancelled) **RETURN** final state snapshot and close stream - `inst-check-terminal`
5. [x] - `p1` - Control plane registers SSE subscriber for the transfer ID - `inst-register-subscriber`
6. [x] - `p1` - Agent reports progress via gRPC AgentService.ReportProgress (bytes_transferred, total_bytes, speed_bytes_sec, chunks_completed, chunks_total) - `inst-agent-reports`
7. [x] - `p1` - Control plane calculates derived fields (percent_complete, estimated_time_remaining) using algorithm `cpt-katapult-algo-observability-emit-progress` - `inst-calculate-derived`
8. [x] - `p1` - Control plane pushes SSE event to all registered subscribers for the transfer - `inst-push-sse`
9. [x] - `p1` - **IF** transfer reaches a terminal state, send final event and close SSE stream - `inst-stream-complete`
10. [x] - `p1` - **RETURN** SSE stream with progress events - `inst-return-stream`

### Query Transfer Events

- [x] `p2` - **ID**: `cpt-katapult-flow-observability-query-events`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator retrieves the full event timeline for a completed transfer including state transitions, milestones, and errors

**Error Scenarios**:
- Transfer ID does not exist and API returns 404
- No events recorded yet and API returns empty list

**Steps**:
1. [x] - `p2` - Operator requests transfer event timeline via API or UI - `inst-request-events`
2. [x] - `p2` - API: GET /api/v1alpha1/transfers/{id}/events - `inst-api-get-events`
3. [x] - `p2` - **IF** transfer ID does not exist **RETURN** 404 Not Found - `inst-events-check-exists`
4. [x] - `p2` - DB: SELECT transfer_events WHERE transfer_id=? ORDER BY created_at ASC - `inst-db-query-events`
5. [x] - `p2` - **RETURN** event timeline (list of event_type, message, metadata, timestamp) - `inst-return-events`

### Scrape Prometheus Metrics

- [x] `p2` - **ID**: `cpt-katapult-flow-observability-scrape-metrics`

**Actor**: `cpt-katapult-actor-control-plane`

**Success Scenarios**:
- Prometheus scrapes /metrics endpoint and receives all transfer and agent health metrics in OpenMetrics format

**Error Scenarios**:
- Metrics endpoint is temporarily unavailable during control plane restart

**Steps**:
1. [x] - `p2` - Prometheus sends HTTP GET to /metrics endpoint on control plane or agent - `inst-prom-scrape`
2. [x] - `p2` - Control plane or agent collects current metric values using algorithm `cpt-katapult-algo-observability-collect-metrics` - `inst-collect-metrics`
3. [x] - `p2` - **RETURN** metrics in OpenMetrics/Prometheus exposition format - `inst-return-metrics`

## 3. Processes / Business Logic (CDSL)

### Emit Transfer Progress

- [x] `p1` - **ID**: `cpt-katapult-algo-observability-emit-progress`

**Input**: Raw progress report from agent (bytes_transferred, total_bytes, speed_bytes_sec, chunks_completed, chunks_total, transfer_id)

**Output**: Enriched progress event (adds percent_complete, estimated_time_remaining, formatted_speed)

**Steps**:
1. [x] - `p1` - Receive raw progress from agent via gRPC AgentService.ReportProgress - `inst-receive-raw`
2. [x] - `p1` - Calculate percent_complete as (bytes_transferred / total_bytes) * 100 - `inst-calc-percent`
3. [x] - `p1` - Calculate estimated_time_remaining as (total_bytes - bytes_transferred) / speed_bytes_sec - `inst-calc-eta`
4. [x] - `p1` - **IF** speed_bytes_sec is zero, set estimated_time_remaining to unknown - `inst-zero-speed`
5. [x] - `p1` - Format speed into human-readable units (B/s, MB/s, GB/s) - `inst-format-speed`
6. [x] - `p1` - Attach transfer correlation ID to the progress event for log correlation - `inst-attach-correlation`
7. [x] - `p1` - **RETURN** enriched progress event - `inst-return-enriched`

### Record Transfer Event

- [x] `p2` - **ID**: `cpt-katapult-algo-observability-record-event`

**Input**: Transfer ID, event type, message, optional metadata (key-value pairs)

**Output**: Persisted event record

**Steps**:
1. [x] - `p2` - Receive event from transfer orchestrator (state transition, error, milestone) - `inst-receive-event`
2. [x] - `p2` - Generate event timestamp (server-side UTC) - `inst-gen-timestamp`
3. [x] - `p2` - DB: INSERT transfer_events(transfer_id, event_type, message, metadata, created_at) - `inst-db-insert-event`
4. [x] - `p2` - Emit structured log entry with transfer correlation ID, event type, and message - `inst-log-event`
5. [x] - `p2` - **RETURN** persisted event ID - `inst-return-event-id`

### Enrich Error Context

- [x] `p1` - **ID**: `cpt-katapult-algo-observability-enrich-error`

**Input**: Raw error from transfer operation, agent local context

**Output**: Actionable error message with remediation hint

**Steps**:
1. [x] - `p1` - Receive raw error from mover plugin or system call - `inst-receive-error`
2. [x] - `p1` - Classify error category (disk_full, permission_denied, network_unreachable, timeout, s3_error, unknown) - `inst-classify-error`
3. [x] - `p1` - **IF** category is disk_full, query available disk space and include in message (e.g., "Destination disk full: 2.1 TB available, 5.3 TB required") - `inst-enrich-disk`
4. [x] - `p1` - **IF** category is permission_denied, include file path and current permissions - `inst-enrich-permissions`
5. [x] - `p1` - **IF** category is network_unreachable, include target address and last successful connection time - `inst-enrich-network`
6. [x] - `p1` - **IF** category is timeout, include operation name, configured timeout, and elapsed time - `inst-enrich-timeout`
7. [x] - `p1` - **IF** category is s3_error, include S3 operation, bucket, key prefix, and HTTP status - `inst-enrich-s3`
8. [x] - `p1` - **IF** category is unknown, include raw error message and stack context - `inst-enrich-unknown`
9. [x] - `p1` - Compose actionable message: "{error summary}. Suggested action: {remediation}" - `inst-compose-message`
10. [x] - `p1` - **RETURN** actionable error message - `inst-return-actionable`

### Collect Prometheus Metrics

- [x] `p2` - **ID**: `cpt-katapult-algo-observability-collect-metrics`

**Input**: Metric registry (in-process)

**Output**: Prometheus exposition format text

**Steps**:
1. [x] - `p2` - Register control plane metrics on startup: transfer counters (total, active, by_status, by_strategy), transfer duration histogram, transfer bytes total, agent health gauges (healthy, unhealthy, disconnected) - `inst-register-cp-metrics`
2. [x] - `p2` - Register agent metrics on startup: mover bytes transferred counter, mover speed gauge, mover errors counter (by category), chunks transferred counter - `inst-register-agent-metrics`
3. [x] - `p2` - Increment/update metric values on each relevant event (transfer state change, progress report, agent health change) - `inst-update-metrics`
4. [x] - `p2` - On scrape request, serialize all registered metrics into Prometheus exposition format - `inst-serialize-metrics`
5. [x] - `p2` - **RETURN** metrics text - `inst-return-metrics-text`

## 4. States (CDSL)

### Not Applicable

Not applicable because observability does not introduce new entity lifecycle states. Transfer states (Pending, Validating, Transferring, Completed, Failed, Cancelled) are defined in the Transfer Engine feature (`cpt-katapult-feature-transfer-engine`). This feature observes and reports on those state transitions but does not add or modify them.

## 5. Definitions of Done

### Real-Time Progress Streaming

- [x] `p1` - **ID**: `cpt-katapult-dod-observability-realtime-progress`

The system **MUST** provide real-time transfer progress via SSE at GET /api/v1alpha1/transfers/{id}/progress, including bytes_transferred, total_bytes, percent_complete, speed_bytes_sec, estimated_time_remaining, and chunk-level progress (chunks_completed, chunks_total) for S3-staged transfers. Progress updates **MUST** be visible within 5 seconds of the agent reporting them.

**Implements**:
- `cpt-katapult-flow-observability-stream-progress`
- `cpt-katapult-algo-observability-emit-progress`

**Covers (PRD)**:
- `cpt-katapult-fr-realtime-progress`
- `cpt-katapult-nfr-progress-latency`

**Covers (DESIGN)**:
- `cpt-katapult-component-api-server`
- `cpt-katapult-component-agent-runtime`

**Touches**:
- API: `GET /api/v1alpha1/transfers/{id}/progress` (SSE)
- API: `gRPC AgentService.ReportProgress`
- Entities: `Transfer`

### Transfer Event History

- [x] `p2` - **ID**: `cpt-katapult-dod-observability-event-history`

The system **MUST** maintain a transfer audit log recording who initiated each transfer, when, source, destination, transfer strategy used, outcome, duration, and bytes transferred. Events **MUST** be queryable via GET /api/v1alpha1/transfers/{id}/events and ordered chronologically.

**Implements**:
- `cpt-katapult-flow-observability-query-events`
- `cpt-katapult-algo-observability-record-event`

**Covers (PRD)**:
- `cpt-katapult-fr-transfer-history`

**Covers (DESIGN)**:
- `cpt-katapult-component-api-server`
- `cpt-katapult-component-transfer-orchestrator`
- `cpt-katapult-dbtable-transfer-events`

**Touches**:
- API: `GET /api/v1alpha1/transfers/{id}/events`
- DB: `transfer_events`
- Entities: `Transfer`

### Metrics and Structured Logging

- [x] `p2` - **ID**: `cpt-katapult-dod-observability-metrics-logging`

The system **MUST** expose Prometheus metrics at /metrics for transfer throughput (bytes/sec), transfer duration (histogram), transfer counts (by status and strategy), and agent health (healthy/unhealthy/disconnected counts). The system **MUST** produce structured JSON logs with per-transfer correlation IDs from both agents and control plane.

**Implements**:
- `cpt-katapult-flow-observability-scrape-metrics`
- `cpt-katapult-algo-observability-collect-metrics`

**Covers (PRD)**:
- `cpt-katapult-fr-metrics-logging`
- `cpt-katapult-nfr-throughput-intra`
- `cpt-katapult-nfr-throughput-cross`
- `cpt-katapult-nfr-bounded-failure`
- `cpt-katapult-nfr-initiation-time`

**Covers (DESIGN)**:
- `cpt-katapult-component-api-server`
- `cpt-katapult-component-agent-runtime`

**Touches**:
- API: `GET /metrics`
- Entities: `Transfer`, `Agent`

### Actionable Error Messages

- [x] `p1` - **ID**: `cpt-katapult-dod-observability-actionable-errors`

All transfer failures **MUST** surface actionable error messages that include the failure reason and a suggested remediation. Agents enrich errors with local context (disk space, permissions, network reachability) and the control plane formats and surfaces them through the API, SSE progress stream, and transfer event timeline.

**Implements**:
- `cpt-katapult-algo-observability-enrich-error`

**Covers (PRD)**:
- `cpt-katapult-fr-actionable-errors`

**Covers (DESIGN)**:
- `cpt-katapult-component-agent-runtime`
- `cpt-katapult-component-transfer-orchestrator`

**Touches**:
- API: `GET /api/v1alpha1/transfers/{id}/progress` (SSE), `GET /api/v1alpha1/transfers/{id}/events`
- Entities: `Transfer`

## 6. Acceptance Criteria

- [ ] SSE progress stream delivers bytes_transferred, speed, ETA, and percent_complete for an active intra-cluster transfer
- [ ] SSE progress stream delivers chunk-level progress (chunks_completed/chunks_total) for an active S3-staged transfer
- [ ] Progress updates are visible via SSE within 5 seconds of the agent reporting them
- [ ] SSE stream returns a final state snapshot and closes cleanly when a transfer reaches a terminal state
- [ ] SSE connection reconnects and receives current state immediately without data loss
- [ ] Transfer event timeline records state transitions (Pending, Validating, Transferring, Completed/Failed/Cancelled) with timestamps
- [ ] Transfer event timeline records who initiated the transfer, source, destination, strategy, outcome, duration, and bytes transferred
- [ ] Prometheus /metrics endpoint exposes transfer counters (total, active, by status, by strategy)
- [ ] Prometheus /metrics endpoint exposes transfer duration histogram and transfer bytes total
- [ ] Prometheus /metrics endpoint exposes agent health gauges (healthy, unhealthy, disconnected counts)
- [ ] Agent /metrics endpoint exposes mover-level metrics (bytes transferred, speed, errors by category)
- [ ] Structured JSON logs include a per-transfer correlation ID on both control plane and agent
- [ ] Failed transfers surface actionable error messages with failure reason and remediation hint
- [ ] Disk-full errors include available and required space in the error message
- [ ] Permission errors include the file path and current permissions in the error message

## 7. Non-Applicable Domains

**COMPL** (Compliance): Not applicable because this feature processes Kubernetes cluster metadata and transfer operational data only. No personal data (PII) is handled. GDPR, HIPAA, PCI DSS do not apply per PRD Section 6.2 NFR Exclusions.

**UX** (Usability): Not applicable because this feature has no direct user interface. Observability data (progress streams, events, metrics) is consumed via the API by other features — Web UI (`cpt-katapult-feature-web-ui`) and CLI (`cpt-katapult-feature-api-cli`). UI/UX concerns for how progress is displayed belong to those consuming features.

**SEC** (Security): Not applicable as a standalone concern for this feature because all observability endpoints (SSE, events, metrics) are served through the API Server which enforces authentication and RBAC middleware from the Security feature (`cpt-katapult-feature-security`). No new authentication or authorization boundaries are introduced. The Prometheus /metrics endpoint follows standard practice of being accessible within the cluster network.
