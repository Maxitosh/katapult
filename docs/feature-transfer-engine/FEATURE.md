# Feature: Transfer Engine

- [ ] `p1` - **ID**: `cpt-katapult-featstatus-transfer-engine`

<!-- toc -->

- [1. Feature Context](#1-feature-context)
  - [1.1 Overview](#11-overview)
  - [1.2 Purpose](#12-purpose)
  - [1.3 Configuration Parameters](#13-configuration-parameters)
  - [1.4 Actors](#14-actors)
  - [1.5 References](#15-references)
- [2. Actor Flows (CDSL)](#2-actor-flows-cdsl)
  - [Initiate Transfer](#initiate-transfer)
  - [Cancel Transfer](#cancel-transfer)
  - [Report Progress](#report-progress)
- [3. Processes / Business Logic (CDSL)](#3-processes-business-logic-cdsl)
  - [Validate Transfer Request](#validate-transfer-request)
  - [Select Transfer Strategy](#select-transfer-strategy)
  - [Apply Retry with Backoff](#apply-retry-with-backoff)
  - [Execute Resource Cleanup](#execute-resource-cleanup)
- [4. States (CDSL)](#4-states-cdsl)
  - [Transfer Lifecycle State Machine](#transfer-lifecycle-state-machine)
- [5. Definitions of Done](#5-definitions-of-done)
  - [Transfer Initiation](#transfer-initiation)
  - [Transfer Cancellation](#transfer-cancellation)
  - [Strategy Selection](#strategy-selection)
  - [Resumable Transfers](#resumable-transfers)
  - [Destination Safety](#destination-safety)
  - [Retry with Backoff](#retry-with-backoff)
  - [Transfer Autonomy](#transfer-autonomy)
  - [Resource Cleanup](#resource-cleanup)
  - [Transfer State Persistence](#transfer-state-persistence)
- [6. Acceptance Criteria](#6-acceptance-criteria)
- [7. Non-Applicable Domains](#7-non-applicable-domains)

<!-- /toc -->

## 1. Feature Context

- [ ] `p1` - `cpt-katapult-feature-transfer-engine`

### 1.1 Overview

End-to-end volume transfer orchestration for the hub side of the hub-and-spoke architecture. The Transfer Orchestrator validates source and destination PVCs, selects a transfer strategy based on cluster topology, coordinates source and destination agents, manages the transfer state machine from creation through terminal state, retries failed phases with exponential backoff and jitter, and executes resource cleanup on completion, failure, or cancellation.

Problem: Moving Persistent Volume data between Kubernetes nodes and clusters requires coordinating multiple agents, selecting an appropriate transfer strategy, handling partial failures with resume capability, and cleaning up resources reliably.
Primary value: Enables operators to transfer volume data across nodes and clusters with automatic strategy selection, retry logic, and guaranteed resource cleanup.
Key assumptions: Agent System feature is operational — agents are registered, healthy, and reporting PVC inventory. S3-compatible object store is accessible from all clusters for cross-cluster transfers with resume capability.

### 1.2 Purpose

Orchestrate volume transfers end-to-end — selecting strategies, coordinating source and destination agents, managing the transfer state machine, and handling failures with retry and resumption.

This feature addresses:
- `cpt-katapult-fr-initiate-transfer` — transfer creation with source/destination PVC validation
- `cpt-katapult-fr-cancel-transfer` — operator-initiated cancellation with safe destination cleanup
- `cpt-katapult-fr-strategy-selection` — automatic strategy selection based on cluster topology
- `cpt-katapult-fr-resumable-transfer` — resumable cross-cluster transfers via chunked S3 staging
- `cpt-katapult-fr-destination-safety` — destination pre-flight checks (empty or overwrite-permitted)
- `cpt-katapult-fr-retry-backoff` — retry with exponential backoff and jitter
- `cpt-katapult-fr-transfer-autonomy` — agents continue transfers if control plane is temporarily unavailable
- `cpt-katapult-fr-resource-cleanup` — resource cleanup on completion, failure, or cancellation

Design principles applied:
- `cpt-katapult-principle-agent-autonomy` — once agents receive transfer commands and the data path is established, transfers complete independently of further control plane communication

Design constraints satisfied:
- `cpt-katapult-constraint-s3-required` — cross-cluster transfers with resume capability require an S3-compatible object store; fallback to direct mover when S3 is unavailable
- `cpt-katapult-constraint-single-cp` — single control plane replica in v1; active transfers survive CP downtime per agent autonomy principle

### 1.3 Configuration Parameters

| Parameter | Default | Description | Referenced In |
|-----------|---------|-------------|---------------|
| Retry max attempts | 3 | Maximum retries per transfer phase | `cpt-katapult-algo-transfer-engine-retry-backoff` |
| Retry base delay | 5s | Base delay for exponential backoff | `cpt-katapult-algo-transfer-engine-retry-backoff` |
| Retry max delay | 5min | Maximum backoff delay cap | `cpt-katapult-algo-transfer-engine-retry-backoff` |
| Jitter factor | 0.3 | Random jitter multiplier applied to backoff delay | `cpt-katapult-algo-transfer-engine-retry-backoff` |
| Chunk size | 4 GiB | Fixed-size segment for S3-staged cross-cluster transfers | `cpt-katapult-flow-transfer-engine-initiate` |
| Validation timeout | 30s | Timeout for PVC and agent validation phase | `cpt-katapult-algo-transfer-engine-validate-request` |
| Transfer command timeout | 60s | Timeout waiting for agent to acknowledge transfer command | `cpt-katapult-flow-transfer-engine-initiate` |

### 1.4 Actors

| Actor | Role in Feature |
|-------|-----------------|
| `cpt-katapult-actor-infra-engineer` | Initiates transfers via API, cancels active transfers, monitors transfer status |
| `cpt-katapult-actor-agent` | Executes data transfer on worker nodes, reports progress via gRPC streaming |
| `cpt-katapult-actor-control-plane` | Orchestrates transfer lifecycle, selects strategy, coordinates agents, manages state |

### 1.5 References

- **PRD**: [PRD.md](../PRD.md)
- **Design**: [DESIGN.md](../DESIGN.md)
- **Dependencies**: `cpt-katapult-feature-agent-system` (agents must be registered and reporting PVC inventory)

## 2. Actor Flows (CDSL)

### Initiate Transfer

- [ ] `p1` - **ID**: `cpt-katapult-flow-transfer-engine-initiate`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator creates a transfer, orchestrator validates PVCs and agents, selects streaming strategy for intra-cluster transfer, and agents complete the data transfer
- Operator creates a cross-cluster transfer, orchestrator selects S3 strategy, source agent uploads chunks, destination agent downloads and reassembles

**Error Scenarios**:
- Source PVC does not exist in agent registry and transfer creation fails with actionable error
- Source or destination agent is unhealthy and transfer creation fails
- Destination PVC is non-empty and overwrite is not permitted, transfer fails with safety error
- Agent fails to acknowledge transfer command within timeout

**Steps**:
1. [ ] - `p1` - Operator submits transfer request via API (source cluster/PVC, destination cluster/PVC, optional strategy override, allow_overwrite flag, retry config) - `inst-submit-request`
2. [ ] - `p1` - DB: INSERT transfers(source_cluster, source_pvc, destination_cluster, destination_pvc, strategy=null, state=pending, allow_overwrite, retry_max, created_by, created_at=now) RETURNING id - `inst-db-create-transfer`
3. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=created, message="Transfer created", created_at=now) - `inst-db-event-created`
4. [ ] - `p1` - Transition transfer state to Validating using state machine `cpt-katapult-state-transfer-engine-transfer-lifecycle` - `inst-transition-validating`
5. [ ] - `p1` - Validate transfer request using algorithm `cpt-katapult-algo-transfer-engine-validate-request` - `inst-run-validate`
6. [ ] - `p1` - **IF** validation fails **RETURN** error with actionable message and transition to Failed - `inst-validation-fail`
7. [ ] - `p1` - Select transfer strategy using algorithm `cpt-katapult-algo-transfer-engine-select-strategy` - `inst-run-strategy`
8. [ ] - `p1` - DB: UPDATE transfers SET strategy=? WHERE id=? - `inst-db-set-strategy`
9. [ ] - `p1` - Request credentials from Credential Manager for selected strategy (TLS certs for stream, scoped S3 session for s3, TLS certs for direct) - `inst-request-credentials`
10. [ ] - `p1` - Transition transfer state to Transferring - `inst-transition-transferring`
11. [ ] - `p1` - DB: UPDATE transfers SET started_at=now WHERE id=? - `inst-db-set-started`
12. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=started, message="Data transfer started with {strategy} strategy") - `inst-db-event-started`
13. [ ] - `p1` - Send transfer command to source agent via Agent Registry with credentials, strategy config, and chunk size (if S3) - `inst-command-source`
14. [ ] - `p1` - Send transfer command to destination agent via Agent Registry with credentials and strategy config - `inst-command-dest`
15. [ ] - `p1` - **IF** agent does not acknowledge command within transfer command timeout **RETURN** error and transition to Failed - `inst-command-timeout`
16. [ ] - `p1` - **RETURN** transfer_id, state=transferring - `inst-return-transfer`

### Cancel Transfer

- [ ] `p1` - **ID**: `cpt-katapult-flow-transfer-engine-cancel`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Operator cancels an active transfer, orchestrator signals agents, cleanup executes, destination PVC left in safe state

**Error Scenarios**:
- Transfer is already in a terminal state (Completed, Failed, Cancelled) and cancellation is rejected
- Agent is unreachable during cancellation; orchestrator proceeds with server-side cleanup and marks transfer cancelled

**Steps**:
1. [ ] - `p1` - Operator submits cancel request via API (transfer_id) - `inst-submit-cancel`
2. [ ] - `p1` - DB: SELECT transfers WHERE id=? - `inst-db-load-transfer`
3. [ ] - `p1` - **IF** transfer state is Completed, Failed, or Cancelled **RETURN** error "Transfer already in terminal state" - `inst-reject-terminal`
4. [ ] - `p1` - Transition transfer state to Cancelled using state machine `cpt-katapult-state-transfer-engine-transfer-lifecycle` - `inst-transition-cancelled`
5. [ ] - `p1` - DB: UPDATE transfers SET state=cancelled, completed_at=now WHERE id=? - `inst-db-set-cancelled`
6. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=cancelled, message="Transfer cancelled by operator") - `inst-db-event-cancelled`
7. [ ] - `p1` - Send cancel command to source agent via Agent Registry - `inst-cancel-source`
8. [ ] - `p1` - Send cancel command to destination agent via Agent Registry - `inst-cancel-dest`
9. [ ] - `p1` - Execute resource cleanup using algorithm `cpt-katapult-algo-transfer-engine-cleanup` - `inst-run-cleanup`
10. [ ] - `p1` - **RETURN** transfer_id, state=cancelled - `inst-return-cancelled`

### Report Progress

- [ ] `p1` - **ID**: `cpt-katapult-flow-transfer-engine-report-progress`

**Actor**: `cpt-katapult-actor-agent`

**Success Scenarios**:
- Agent reports progress (bytes transferred, speed, chunks completed) and orchestrator updates transfer state and persists event
- Agent reports transfer complete and orchestrator transitions to Completed with cleanup

**Error Scenarios**:
- Agent reports transfer failure and orchestrator applies retry logic
- Agent reports failure and retries are exhausted, orchestrator transitions to Failed

**Steps**:
1. [ ] - `p1` - Agent sends progress report via gRPC AgentService.ReportProgress (transfer_id, bytes_transferred, bytes_total, speed, chunks_completed, chunks_total, status) - `inst-receive-progress`
2. [ ] - `p1` - DB: UPDATE transfers SET bytes_transferred=?, bytes_total=?, chunks_completed=?, chunks_total=? WHERE id=? - `inst-db-update-progress`
3. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=progress, metadata={bytes, speed, chunks}) - `inst-db-event-progress`
4. [ ] - `p1` - **IF** status is "completed" - `inst-check-completed`
   1. [ ] - `p1` - Transition transfer state to Completed - `inst-transition-completed`
   2. [ ] - `p1` - DB: UPDATE transfers SET state=completed, completed_at=now WHERE id=? - `inst-db-set-completed`
   3. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=completed, message="Transfer completed successfully") - `inst-db-event-completed`
   4. [ ] - `p1` - Execute resource cleanup using algorithm `cpt-katapult-algo-transfer-engine-cleanup` - `inst-cleanup-on-complete`
5. [ ] - `p1` - **IF** status is "failed" - `inst-check-failed`
   1. [ ] - `p1` - Apply retry logic using algorithm `cpt-katapult-algo-transfer-engine-retry-backoff` - `inst-run-retry`
   2. [ ] - `p1` - **IF** retry returns "retry" - `inst-check-retry`
      1. [ ] - `p1` - DB: UPDATE transfers SET retry_count=retry_count+1 WHERE id=? - `inst-db-increment-retry`
      2. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=retried, message="Retrying transfer, attempt {N}") - `inst-db-event-retried`
      3. [ ] - `p1` - Re-send transfer command to agents after backoff delay - `inst-resend-command`
   3. [ ] - `p1` - **IF** retry returns "exhausted" - `inst-check-exhausted`
      1. [ ] - `p1` - Transition transfer state to Failed - `inst-transition-failed`
      2. [ ] - `p1` - DB: UPDATE transfers SET state=failed, error_message=?, completed_at=now WHERE id=? - `inst-db-set-failed`
      3. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=failed, message="Transfer failed after {N} retries: {error}") - `inst-db-event-failed`
      4. [ ] - `p1` - Execute resource cleanup using algorithm `cpt-katapult-algo-transfer-engine-cleanup` - `inst-cleanup-on-fail`
6. [ ] - `p1` - **RETURN** Acknowledged - `inst-return-ack`

## 3. Processes / Business Logic (CDSL)

### Validate Transfer Request

- [ ] `p1` - **ID**: `cpt-katapult-algo-transfer-engine-validate-request`

**Input**: Transfer request (source_cluster, source_pvc, destination_cluster, destination_pvc, allow_overwrite)

**Output**: Validation result (success, or rejection with actionable error message)

**Steps**:
1. [ ] - `p1` - DB: SELECT agent_pvcs JOIN agents WHERE pvc_name=source_pvc AND cluster_id=source_cluster AND agents.healthy=true - `inst-lookup-source-pvc`
2. [ ] - `p1` - **IF** source PVC not found in agent registry **RETURN** error "Source PVC {source_pvc} not found in cluster {source_cluster}. Verify the PVC exists and the agent on the owning node is healthy." - `inst-reject-no-source`
3. [ ] - `p1` - DB: SELECT agent_pvcs JOIN agents WHERE pvc_name=destination_pvc AND cluster_id=destination_cluster AND agents.healthy=true - `inst-lookup-dest-pvc`
4. [ ] - `p1` - **IF** destination PVC not found in agent registry **RETURN** error "Destination PVC {destination_pvc} not found in cluster {destination_cluster}. Verify the PVC exists and the agent on the owning node is healthy." - `inst-reject-no-dest`
5. [ ] - `p1` - **IF** source_cluster=destination_cluster AND source_pvc=destination_pvc **RETURN** error "Source and destination PVC cannot be the same" - `inst-reject-same-pvc`
6. [ ] - `p1` - Query destination agent for destination PVC empty status via Agent Registry - `inst-check-dest-empty`
7. [ ] - `p1` - **IF** destination PVC is non-empty AND allow_overwrite is false **RETURN** error "Destination PVC {destination_pvc} is not empty. Set allow_overwrite=true to overwrite existing data." - `inst-reject-non-empty`
8. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=validated, message="Transfer request validated successfully") - `inst-db-event-validated`
9. [ ] - `p1` - **RETURN** success - `inst-return-valid`

### Select Transfer Strategy

- [ ] `p1` - **ID**: `cpt-katapult-algo-transfer-engine-select-strategy`

**Input**: Transfer request (source_cluster, destination_cluster, optional strategy_override), S3 configuration availability

**Output**: Selected strategy (stream, s3, or direct)

**Steps**:
1. [ ] - `p1` - **IF** strategy_override is provided, validate override is one of (stream, s3, direct) - `inst-check-override`
2. [ ] - `p1` - **IF** strategy_override is valid **RETURN** strategy_override - `inst-return-override`
3. [ ] - `p1` - **IF** strategy_override is invalid **RETURN** error "Invalid strategy: {override}. Valid options: stream, s3, direct" - `inst-reject-invalid-strategy`
4. [ ] - `p1` - **IF** source_cluster equals destination_cluster **RETURN** stream (intra-cluster streaming via tar+zstd+stunnel) - `inst-select-stream`
5. [ ] - `p1` - **IF** source_cluster differs from destination_cluster AND S3 is configured **RETURN** s3 (cross-cluster via chunked S3 staging) - `inst-select-s3`
6. [ ] - `p1` - **IF** source_cluster differs from destination_cluster AND S3 is not configured **RETURN** direct (cross-cluster fallback via rsync/tar+stunnel without chunk-level resume) - `inst-select-direct`

### Apply Retry with Backoff

- [ ] `p2` - **ID**: `cpt-katapult-algo-transfer-engine-retry-backoff`

**Input**: Current retry_count, retry_max, base_delay, max_delay, jitter_factor, failure reason

**Output**: Decision (retry with computed delay, or exhausted)

**Steps**:
1. [ ] - `p1` - **IF** retry_count >= retry_max **RETURN** exhausted - `inst-check-exhausted`
2. [ ] - `p1` - Compute delay = min(base_delay * 2^retry_count, max_delay) - `inst-compute-delay`
3. [ ] - `p1` - Apply jitter: delay = delay * (1 + random(-jitter_factor, +jitter_factor)) - `inst-apply-jitter`
4. [ ] - `p1` - **RETURN** retry with computed delay - `inst-return-retry`

### Execute Resource Cleanup

- [ ] `p1` - **ID**: `cpt-katapult-algo-transfer-engine-cleanup`

**Input**: Transfer record (transfer_id, strategy, state), agent connection status

**Output**: Cleanup result (success or partial with warnings)

**Steps**:
1. [ ] - `p1` - Request Credential Manager to revoke transfer credentials - `inst-revoke-credentials`
2. [ ] - `p1` - **IF** source agent is reachable, signal source agent to clean up temporary Kubernetes resources (Services, Secrets, staging directories) - `inst-cleanup-source`
3. [ ] - `p1` - **IF** destination agent is reachable, signal destination agent to remove staging directory and temporary resources - `inst-cleanup-dest`
4. [ ] - `p1` - **IF** strategy is s3, delete all S3 objects under the transfer prefix via Credential Manager - `inst-cleanup-s3`
5. [ ] - `p1` - **IF** any cleanup step fails, log warning but do not fail the overall cleanup (best-effort) - `inst-cleanup-warn`
6. [ ] - `p1` - DB: INSERT transfer_events(transfer_id, event_type=cleanup, message="Resource cleanup completed") - `inst-db-event-cleanup`
7. [ ] - `p1` - **RETURN** cleanup result - `inst-return-cleanup`

## 4. States (CDSL)

### Transfer Lifecycle State Machine

- [ ] `p1` - **ID**: `cpt-katapult-state-transfer-engine-transfer-lifecycle`

**States**: Pending, Validating, Transferring, Completed, Failed, Cancelled

**Initial State**: Pending

**Transitions**:
1. [ ] - `p1` - **FROM** Pending **TO** Validating **WHEN** orchestrator begins PVC and agent validation - `inst-pending-to-validating`
2. [ ] - `p1` - **FROM** Validating **TO** Transferring **WHEN** validation succeeds, strategy selected, credentials issued, and agent commands sent - `inst-validating-to-transferring`
3. [ ] - `p1` - **FROM** Validating **TO** Failed **WHEN** validation fails (PVC not found, agent unhealthy, destination non-empty) - `inst-validating-to-failed`
4. [ ] - `p1` - **FROM** Transferring **TO** Completed **WHEN** agent reports transfer complete and cleanup succeeds - `inst-transferring-to-completed`
5. [ ] - `p1` - **FROM** Transferring **TO** Failed **WHEN** agent reports failure and retry attempts exhausted - `inst-transferring-to-failed`
6. [ ] - `p1` - **FROM** Pending **TO** Cancelled **WHEN** operator cancels before validation starts - `inst-pending-to-cancelled`
7. [ ] - `p1` - **FROM** Validating **TO** Cancelled **WHEN** operator cancels during validation - `inst-validating-to-cancelled`
8. [ ] - `p1` - **FROM** Transferring **TO** Cancelled **WHEN** operator cancels active transfer - `inst-transferring-to-cancelled`

**Invariants**: All transitions not listed above are invalid. In particular:
- Completed → any state is prohibited (terminal state)
- Failed → any state is prohibited (terminal state)
- Cancelled → any state is prohibited (terminal state)
- Transferring → Validating is prohibited (no backward transitions)
- Transferring → Pending is prohibited (no backward transitions)

## 5. Definitions of Done

### Transfer Initiation

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-initiation`

The system **MUST** accept transfer requests specifying source and destination cluster/PVC pairs, validate that both PVCs exist in the agent registry with healthy agents, select an appropriate transfer strategy, issue credentials, send commands to source and destination agents, and persist the transfer record with audit events.

**Implements**:
- `cpt-katapult-flow-transfer-engine-initiate`
- `cpt-katapult-algo-transfer-engine-validate-request`
- `cpt-katapult-algo-transfer-engine-select-strategy`

**Covers (PRD)**:
- `cpt-katapult-fr-initiate-transfer`

**Covers (DESIGN)**:
- `cpt-katapult-principle-agent-autonomy`
- `cpt-katapult-component-transfer-orchestrator`
- `cpt-katapult-seq-intra-transfer`
- `cpt-katapult-seq-cross-transfer`
- `cpt-katapult-dbtable-transfers`
- `cpt-katapult-dbtable-transfer-events`

**Touches**:
- API: `gRPC AgentService.StreamCommands`
- DB: `transfers`, `transfer_events`
- Entities: `Transfer`, `Chunk`, `Credential`

### Transfer Cancellation

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-cancellation`

The system **MUST** accept cancellation requests for active transfers (Pending, Validating, or Transferring states), transition the transfer to Cancelled, signal both source and destination agents to stop, execute resource cleanup (credentials, temp resources, S3 objects), and leave the destination PVC in a safe state.

**Implements**:
- `cpt-katapult-flow-transfer-engine-cancel`
- `cpt-katapult-algo-transfer-engine-cleanup`

**Covers (PRD)**:
- `cpt-katapult-fr-cancel-transfer`

**Covers (DESIGN)**:
- `cpt-katapult-component-transfer-orchestrator`
- `cpt-katapult-seq-cancel-transfer`

**Touches**:
- API: `gRPC AgentService.StreamCommands`
- DB: `transfers`, `transfer_events`
- Entities: `Transfer`

### Strategy Selection

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-strategy`

The system **MUST** automatically select the optimal transfer strategy based on source/destination topology: streaming (tar+zstd via stunnel) for intra-cluster transfers, S3-staged (chunked upload/download) for cross-cluster transfers when S3 is configured, and direct (rsync/tar via stunnel) as cross-cluster fallback without S3. Manual strategy override must be supported.

**Implements**:
- `cpt-katapult-algo-transfer-engine-select-strategy`

**Covers (PRD)**:
- `cpt-katapult-fr-strategy-selection`

**Covers (DESIGN)**:
- `cpt-katapult-constraint-s3-required`
- `cpt-katapult-component-transfer-orchestrator`

**Touches**:
- DB: `transfers`
- Entities: `Transfer`

### Resumable Transfers

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-resumable`

The system **MUST** support resumable cross-cluster transfers via chunked S3 staging. Data is compressed and split into fixed-size chunks (configurable, default 4 GiB) uploaded to S3. On failure, transfer resumes from the first missing chunk rather than restarting. Destination agent downloads chunks in parallel, verifies checksums, reassembles, and extracts.

**Implements**:
- `cpt-katapult-flow-transfer-engine-initiate`
- `cpt-katapult-flow-transfer-engine-report-progress`

**Covers (PRD)**:
- `cpt-katapult-fr-resumable-transfer`

**Covers (DESIGN)**:
- `cpt-katapult-constraint-s3-required`
- `cpt-katapult-component-transfer-orchestrator`
- `cpt-katapult-seq-cross-transfer`

**Touches**:
- API: `gRPC AgentService.StreamCommands`, `gRPC AgentService.ReportProgress`
- DB: `transfers`
- Entities: `Transfer`, `Chunk`

### Destination Safety

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-dest-safety`

The system **MUST** perform pre-flight checks on the destination PVC before starting data transfer. If the destination is non-empty and `allow_overwrite` is false, the transfer fails with an actionable error message. If `allow_overwrite` is true, existing data on the destination is overwritten.

**Implements**:
- `cpt-katapult-algo-transfer-engine-validate-request`

**Covers (PRD)**:
- `cpt-katapult-fr-destination-safety`

**Covers (DESIGN)**:
- `cpt-katapult-component-transfer-orchestrator`

**Touches**:
- API: `gRPC AgentService.StreamCommands`
- Entities: `Transfer`, `PVCInfo`

### Retry with Backoff

- [ ] `p2` - **ID**: `cpt-katapult-dod-transfer-engine-retry`

The system **MUST** retry failed transfer phases with configurable exponential backoff (base delay * 2^attempt) capped at a maximum delay, with random jitter applied to prevent thundering herd. After retry exhaustion, the transfer transitions to Failed with an actionable error message.

**Implements**:
- `cpt-katapult-algo-transfer-engine-retry-backoff`
- `cpt-katapult-flow-transfer-engine-report-progress`

**Covers (PRD)**:
- `cpt-katapult-fr-retry-backoff`

**Covers (DESIGN)**:
- `cpt-katapult-component-transfer-orchestrator`

**Touches**:
- DB: `transfers`, `transfer_events`
- Entities: `Transfer`

### Transfer Autonomy

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-autonomy`

The system **MUST** ensure that once agents receive transfer commands and the data path is established, transfers complete independently of further control plane communication. Agents continue data movement during control plane downtime. When the control plane recovers, agents report final status and the orchestrator reconciles transfer state.

**Implements**:
- `cpt-katapult-flow-transfer-engine-report-progress`

**Covers (PRD)**:
- `cpt-katapult-fr-transfer-autonomy`

**Covers (DESIGN)**:
- `cpt-katapult-principle-agent-autonomy`
- `cpt-katapult-constraint-single-cp`
- `cpt-katapult-component-transfer-orchestrator`

**Touches**:
- API: `gRPC AgentService.ReportProgress`
- DB: `transfers`, `transfer_events`
- Entities: `Transfer`

### Resource Cleanup

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-cleanup`

The system **MUST** execute resource cleanup on all terminal states (Completed, Failed, Cancelled). Cleanup includes revoking transfer credentials, signaling agents to remove temporary Kubernetes resources (Services, Secrets, staging directories), and deleting S3 objects for S3-staged transfers. Cleanup is best-effort — individual step failures are logged as warnings but do not prevent the transfer from reaching its terminal state.

**Implements**:
- `cpt-katapult-algo-transfer-engine-cleanup`

**Covers (PRD)**:
- `cpt-katapult-fr-resource-cleanup`

**Covers (DESIGN)**:
- `cpt-katapult-component-transfer-orchestrator`

**Touches**:
- API: `gRPC AgentService.StreamCommands`
- DB: `transfer_events`
- Entities: `Transfer`, `Credential`

### Transfer State Persistence

- [ ] `p1` - **ID**: `cpt-katapult-dod-transfer-engine-persistence`

The system **MUST** persist all transfer state and audit events to PostgreSQL. The transfers table stores source/destination, strategy, state, progress metrics, retry count, error messages, and timestamps. The transfer_events table stores a timeline of all state transitions and progress events with metadata. Both tables survive control plane restarts.

**Implements**:
- `cpt-katapult-flow-transfer-engine-initiate`
- `cpt-katapult-flow-transfer-engine-cancel`
- `cpt-katapult-flow-transfer-engine-report-progress`

**Covers (PRD)**:
- `cpt-katapult-fr-initiate-transfer`
- `cpt-katapult-fr-cancel-transfer`

**Covers (DESIGN)**:
- `cpt-katapult-component-transfer-orchestrator`
- `cpt-katapult-dbtable-transfers`
- `cpt-katapult-dbtable-transfer-events`

**Touches**:
- DB: `transfers`, `transfer_events`
- Entities: `Transfer`

## 6. Acceptance Criteria

- [ ] An operator can create a transfer specifying source and destination cluster/PVC pairs and receive a transfer ID
- [ ] Transfer creation validates that source and destination PVCs exist in the agent registry with healthy agents
- [ ] Transfer creation rejects requests where destination PVC is non-empty and overwrite is not permitted
- [ ] Strategy selection picks streaming for intra-cluster, S3 for cross-cluster with S3, and direct for cross-cluster without S3
- [ ] Manual strategy override is accepted when valid and rejected with error when invalid
- [ ] Cross-cluster S3 transfers split data into configurable chunks and resume from the first missing chunk on failure
- [ ] Agents report real-time progress (bytes, speed, chunks) and the orchestrator persists updates
- [ ] Failed transfer phases are retried with exponential backoff and jitter up to the configured maximum
- [ ] Transfers transition to Failed with actionable error messages after retry exhaustion
- [ ] Operators can cancel active transfers (Pending, Validating, Transferring) and the system executes cleanup
- [ ] Cancellation of terminal transfers (Completed, Failed, Cancelled) is rejected
- [ ] Resource cleanup executes on all terminal states: credentials revoked, temp resources removed, S3 objects deleted
- [ ] Agents continue active transfers during control plane downtime and report final status on reconnection
- [ ] All transfer state transitions and events are persisted to PostgreSQL and survive control plane restarts

## 7. Non-Applicable Domains

**COMPL** (Compliance): Not applicable because this feature transfers blockchain state data (block headers, transaction indexes, state tries) only. No personal data (PII) is handled. GDPR, HIPAA, PCI DSS do not apply per PRD Section 6.2 NFR Exclusions.

**UX** (Usability): Not applicable because this feature has no user-facing interface. Operators interact with transfers via the API/CLI feature (`cpt-katapult-feature-api-cli`) and monitor progress via the Observability feature (`cpt-katapult-feature-observability`), not this feature.

**OPS** (Operations): Not applicable as a standalone section because deployment is a standard Kubernetes Deployment for the control plane. Observability (metrics, logging, progress streaming) is addressed in the Observability feature (`cpt-katapult-feature-observability`). No feature-specific rollout strategy beyond standard Kubernetes rolling updates.
