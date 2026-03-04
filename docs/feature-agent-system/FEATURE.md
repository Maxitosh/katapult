# Feature: Agent System

- [ ] `p1` - **ID**: `cpt-katapult-featstatus-agent-system`

<!-- toc -->

- [1. Feature Context](#1-feature-context)
  - [1.1 Overview](#11-overview)
  - [1.2 Purpose](#12-purpose)
  - [1.3 Configuration Parameters](#13-configuration-parameters)
  - [1.4 Actors](#14-actors)
  - [1.5 References](#15-references)
- [2. Actor Flows (CDSL)](#2-actor-flows-cdsl)
  - [Agent Registration](#agent-registration)
  - [Agent Heartbeat](#agent-heartbeat)
  - [PVC Discovery](#pvc-discovery)
- [3. Processes / Business Logic (CDSL)](#3-processes-business-logic-cdsl)
  - [Validate Registration](#validate-registration)
  - [Discover PVCs](#discover-pvcs)
  - [Evaluate Agent Health](#evaluate-agent-health)
- [4. States (CDSL)](#4-states-cdsl)
  - [Agent Lifecycle State Machine](#agent-lifecycle-state-machine)
- [5. Definitions of Done](#5-definitions-of-done)
  - [Agent Registration](#agent-registration-1)
  - [Agent Heartbeat Monitoring](#agent-heartbeat-monitoring)
  - [PVC Discovery](#pvc-discovery-1)
  - [Agent Authentication](#agent-authentication)
  - [PVC Boundary Enforcement](#pvc-boundary-enforcement)
  - [Agent Inventory Persistence](#agent-inventory-persistence)
- [6. Acceptance Criteria](#6-acceptance-criteria)
- [7. Non-Applicable Domains](#7-non-applicable-domains)

<!-- /toc -->

## 1. Feature Context

- [ ] `p1` - `cpt-katapult-feature-agent-system`

### 1.1 Overview

Agent registration, health monitoring, and PVC discovery for the spoke side of the hub-and-spoke architecture. Agents deployed as Kubernetes DaemonSets on worker nodes initiate outbound gRPC connections to the control plane, report their cluster identity and tool capabilities, discover local PVCs, and maintain health via periodic heartbeats.

Problem: The control plane needs a reliable inventory of available agents and their PVC resources before it can validate and orchestrate volume transfers.
Primary value: Establishes the foundational agent-to-control-plane communication channel that all other features depend on.
Key assumptions: Each participating Kubernetes cluster has the agent DaemonSet deployed; worker nodes have required tools (GNU tar >= 1.28, zstd, stunnel).

### 1.2 Purpose

Enable agents to self-register with the control plane using Kubernetes ServiceAccount JWT authentication, continuously report health status and PVC inventory, and enforce PVC boundary rules — forming the agent registry that the Transfer Engine, Security, and API features depend on.

This feature addresses:
- `cpt-katapult-fr-agent-registration` — agent auto-registration with cluster identity
- `cpt-katapult-fr-agent-health` — periodic heartbeat and health monitoring
- `cpt-katapult-fr-pvc-discovery` — PVC discovery with PV binding resolution
- `cpt-katapult-fr-agent-auth` — agent authentication via ServiceAccount JWT
- `cpt-katapult-fr-pvc-boundary` — namespace and label filter enforcement

Design principles applied:
- `cpt-katapult-principle-agent-autonomy` — agents operate independently once registered
- `cpt-katapult-principle-outbound-only` — agents initiate all connections; control plane never connects into clusters

Design constraints satisfied:
- `cpt-katapult-constraint-k8s-only` — agents run exclusively as Kubernetes DaemonSets
- `cpt-katapult-constraint-agent-tools` — agents verify required tool dependencies (GNU tar >= 1.28, zstd, stunnel) at startup

### 1.3 Configuration Parameters

| Parameter | Default | Description | Referenced In |
|-----------|---------|-------------|---------------|
| Heartbeat interval | 30s | Time between agent heartbeat messages | `cpt-katapult-flow-agent-system-heartbeat` |
| Heartbeat timeout (unhealthy) | 90s | Time after last heartbeat before agent is marked unhealthy | `cpt-katapult-algo-agent-system-evaluate-health`, `cpt-katapult-state-agent-system-agent-lifecycle` |
| Extended timeout (disconnected) | 5 min | Time after unhealthy before agent is marked disconnected | `cpt-katapult-state-agent-system-agent-lifecycle` |
| PVC namespace allowlist | (none) | Namespaces from which PVCs are discovered | `cpt-katapult-algo-agent-system-discover-pvcs` |
| PVC label selectors | (none) | Label filters applied during PVC discovery | `cpt-katapult-algo-agent-system-discover-pvcs` |
| K8s API retry max attempts | 3 | Maximum retries with exponential backoff for Kubernetes API calls | `cpt-katapult-algo-agent-system-discover-pvcs` |

### 1.4 Actors

| Actor | Role in Feature |
|-------|-----------------|
| `cpt-katapult-actor-agent` | Initiates registration, sends heartbeats, discovers and reports PVCs |
| `cpt-katapult-actor-infra-engineer` | Configures PVC boundary filters (namespace/label), monitors agent status via API |

### 1.5 References

- **PRD**: [PRD.md](../PRD.md)
- **Design**: [DESIGN.md](../DESIGN.md)
- **Dependencies**: None (foundation feature)

## 2. Actor Flows (CDSL)

### Agent Registration

- [x] `p1` - **ID**: `cpt-katapult-flow-agent-system-register`

**Actor**: `cpt-katapult-actor-agent`

**Success Scenarios**:
- Agent starts on a new node, registers successfully, and receives an agent ID
- Agent reconnects after control plane restart, re-registers, and agent registry auto-rebuilds

**Error Scenarios**:
- Agent fails tool verification (missing tar/zstd/stunnel) and aborts startup
- Agent JWT validation fails and registration is rejected
- Control plane is unreachable and agent retries with backoff

**Steps**:
1. [x] - `p1` - Agent pod starts via DaemonSet on Kubernetes worker node - `inst-agent-start`
2. [x] - `p1` - Agent verifies required tools are available (GNU tar >= 1.28, zstd, stunnel) - `inst-verify-tools`
3. [x] - `p1` - **IF** any required tool is missing or below minimum version **RETURN** error and abort startup - `inst-check-tools-fail`
4. [x] - `p1` - Agent reads Kubernetes ServiceAccount JWT token from mounted volume - `inst-read-jwt`
5. [x] - `p1` - Agent discovers local PVCs using algorithm `cpt-katapult-algo-agent-system-discover-pvcs` - `inst-initial-discover`
6. [x] - `p1` - Agent initiates outbound gRPC connection to control plane over mTLS (transport-level authentication and encryption) - `inst-grpc-connect`
7. [x] - `p1` - API: gRPC AgentService.Register (cluster ID, node name, tool versions, PVC inventory, JWT token) - `inst-grpc-register`
8. [x] - `p1` - Control plane validates agent identity using algorithm `cpt-katapult-algo-agent-system-validate-registration` - `inst-validate-reg`
9. [x] - `p1` - **IF** validation fails **RETURN** error with rejection reason - `inst-reject-reg`
10. [x] - `p1` - DB: Within a single transaction: UPSERT agents(cluster_id, node_name, healthy=true, tools, last_heartbeat=now), DELETE agent_pvcs WHERE agent_id=?, INSERT agent_pvcs for each discovered PVC - `inst-db-persist-registration`
11. [x] - `p1` - **RETURN** Registered (agent_id) - `inst-return-registered`

### Agent Heartbeat

- [x] `p1` - **ID**: `cpt-katapult-flow-agent-system-heartbeat`

**Actor**: `cpt-katapult-actor-agent`

**Success Scenarios**:
- Agent sends periodic heartbeat and control plane updates health status and PVC inventory

**Error Scenarios**:
- Heartbeat not received within configurable timeout and agent marked unhealthy
- Agent reconnects after temporary network partition and health restores
- Heartbeat received on unauthenticated connection and control plane rejects with error

**Steps**:
1. [x] - `p1` - Agent waits for heartbeat interval (configurable, default 30s) - `inst-wait-interval`
2. [x] - `p1` - Agent re-discovers local PVCs using algorithm `cpt-katapult-algo-agent-system-discover-pvcs` - `inst-heartbeat-discover`
3. [x] - `p1` - Agent sends heartbeat over the authenticated gRPC connection established during registration (mTLS channel with agent identity) - `inst-grpc-heartbeat`
4. [x] - `p1` - Control plane verifies the heartbeat originates from an authenticated connection and the agent_id matches the connection identity - `inst-verify-heartbeat-auth`
5. [x] - `p1` - **IF** authentication fails or agent_id does not match connection identity **RETURN** error "Unauthenticated heartbeat" - `inst-reject-heartbeat`
6. [x] - `p1` - DB: UPDATE agents SET healthy=true, last_heartbeat=now WHERE id=? - `inst-db-update-heartbeat`
7. [x] - `p1` - DB: Within a single transaction: DELETE agent_pvcs WHERE agent_id=? THEN INSERT agent_pvcs for each discovered PVC - `inst-db-replace-pvcs-heartbeat`
8. [x] - `p1` - **RETURN** Acknowledged - `inst-return-ack`

### PVC Discovery

- [x] `p1` - **ID**: `cpt-katapult-flow-agent-system-discover-pvcs`

**Actor**: `cpt-katapult-actor-agent`

**Success Scenarios**:
- Agent discovers all PVCs on its node, resolves PV bindings, and filters by configured boundaries

**Error Scenarios**:
- Kubernetes API is temporarily unavailable and agent retries with backoff
- PVC has no bound PV and is excluded from inventory

**Steps**:
1. [x] - `p1` - Agent queries Kubernetes API for PVCs in permitted namespaces - `inst-query-pvcs`
2. [x] - `p1` - Agent applies namespace and label filters using algorithm `cpt-katapult-algo-agent-system-discover-pvcs` - `inst-apply-filters`
3. [x] - `p1` - **FOR EACH** PVC in filtered results - `inst-iterate-pvcs`
   1. [x] - `p1` - Resolve PV binding from PVC spec.volumeName - `inst-resolve-pv`
   2. [x] - `p1` - **IF** PVC has no bound PV, skip PVC - `inst-skip-unbound`
   3. [x] - `p1` - Extract PV size from capacity, storage class from spec, and node affinity from PV topology constraints - `inst-extract-pv-attrs`
   4. [x] - `p1` - **IF** PV node affinity does not match current node, skip PVC - `inst-skip-wrong-node`
   5. [x] - `p1` - Add PVCInfo(pvc_name, size_bytes, storage_class, node_affinity) to inventory - `inst-add-to-inventory`
4. [x] - `p1` - **RETURN** PVC inventory list - `inst-return-inventory`

## 3. Processes / Business Logic (CDSL)

### Validate Registration

- [x] `p1` - **ID**: `cpt-katapult-algo-agent-system-validate-registration`

**Input**: Registration request (cluster ID, node name, tool versions, PVC inventory, JWT token)

**Output**: Validation result (success with agent ID, or rejection reason)

**Steps**:
1. [x] - `p1` - Verify JWT token signature against Kubernetes API server public key - `inst-verify-jwt`
2. [x] - `p1` - **IF** JWT invalid or expired **RETURN** error "Invalid agent identity token" - `inst-reject-jwt`
3. [x] - `p1` - Extract cluster identity and ServiceAccount from JWT claims - `inst-extract-claims`
4. [x] - `p1` - **IF** ServiceAccount does not match expected agent ServiceAccount **RETURN** error "Unauthorized ServiceAccount" - `inst-reject-sa`
5. [x] - `p1` - Verify required tool versions: tar >= 1.28, zstd present, stunnel present - `inst-check-tool-versions`
6. [x] - `p1` - **IF** any tool below minimum version **RETURN** error with missing tool details - `inst-reject-tools`
7. [x] - `p1` - DB: SELECT agents WHERE cluster_id=? AND node_name=? - `inst-db-check-existing`
8. [x] - `p1` - **IF** existing agent found, update existing record (re-registration) - `inst-handle-reregister`
9. [x] - `p1` - **ELSE** generate new agent UUID - `inst-generate-id`
10. [x] - `p1` - **RETURN** success (agent_id) - `inst-return-success`

### Discover PVCs

- [x] `p1` - **ID**: `cpt-katapult-algo-agent-system-discover-pvcs`

**Input**: Kubernetes API client, node name, namespace/label filter configuration

**Output**: List of PVCInfo entries for the current node

**Steps**:
1. [x] - `p1` - Read PVC boundary configuration (namespace allowlist, label selectors) - `inst-read-config`
2. [x] - `p1` - Query Kubernetes API: LIST PersistentVolumeClaims with namespace and label filters - `inst-k8s-list-pvcs`
3. [x] - `p1` - **IF** Kubernetes API returns error, retry with exponential backoff (max 3 attempts) - `inst-retry-k8s`
4. [x] - `p1` - **IF** all retries fail **RETURN** error "PVC discovery failed: Kubernetes API unavailable" - `inst-fail-k8s`
5. [x] - `p1` - **FOR EACH** PVC in API response - `inst-iterate-pvcs-algo`
   1. [x] - `p1` - **IF** PVC phase is not Bound, skip - `inst-skip-not-bound`
   2. [x] - `p1` - Resolve PersistentVolume from PVC spec.volumeName - `inst-resolve-pv-algo`
   3. [x] - `p1` - Extract size from PV spec.capacity.storage - `inst-extract-size`
   4. [x] - `p1` - Extract storage class from PV spec.storageClassName - `inst-extract-sc`
   5. [x] - `p1` - Extract node affinity from PV spec.nodeAffinity or topology constraints - `inst-extract-affinity`
   6. [x] - `p1` - **IF** node affinity does not include current node, skip - `inst-skip-non-local`
   7. [x] - `p1` - Build PVCInfo(pvc_name=namespace/name, size_bytes, storage_class, node_affinity) - `inst-build-pvcinfo`
6. [x] - `p1` - **RETURN** filtered PVCInfo list - `inst-return-pvcs`

### Evaluate Agent Health

- [x] `p1` - **ID**: `cpt-katapult-algo-agent-system-evaluate-health`

**Input**: Agent registry, configurable heartbeat timeout (default 90s)

**Output**: Updated agent health statuses

**Steps**:
1. [x] - `p1` - DB: SELECT agents WHERE healthy=true AND last_heartbeat < now() - timeout - `inst-query-stale`
2. [x] - `p1` - **FOR EACH** stale agent - `inst-iterate-stale`
   1. [x] - `p1` - DB: UPDATE agents SET healthy=false WHERE id=? - `inst-mark-unhealthy`
3. [x] - `p1` - **RETURN** count of agents marked unhealthy - `inst-return-count`

## 4. States (CDSL)

### Agent Lifecycle State Machine

- [x] `p1` - **ID**: `cpt-katapult-state-agent-system-agent-lifecycle`

**States**: Registering, Healthy, Unhealthy, Disconnected

**Initial State**: Registering

**Transitions**:
1. [x] - `p1` - **FROM** Registering **TO** Healthy **WHEN** registration succeeds and agent ID is assigned - `inst-reg-to-healthy`
2. [x] - `p1` - **FROM** Registering **TO** Disconnected **WHEN** registration fails (invalid JWT, missing tools) - `inst-reg-to-disconnected`
3. [x] - `p1` - **FROM** Healthy **TO** Unhealthy **WHEN** heartbeat timeout exceeded (configurable, default 90s) - `inst-healthy-to-unhealthy`
4. [x] - `p1` - **FROM** Unhealthy **TO** Healthy **WHEN** heartbeat received from previously unhealthy agent - `inst-unhealthy-to-healthy`
5. [x] - `p1` - **FROM** Unhealthy **TO** Disconnected **WHEN** no heartbeat received for extended timeout (configurable, default 5 min) - `inst-unhealthy-to-disconnected`
6. [x] - `p1` - **FROM** Disconnected **TO** Registering **WHEN** agent reconnects and initiates re-registration - `inst-disconnected-to-registering`
7. [x] - `p1` - **FROM** Healthy **TO** Registering **WHEN** agent reconnects after control plane restart - `inst-healthy-to-registering`

**Invariants**: All transitions not listed above are invalid. In particular:
- Disconnected → Healthy is prohibited (agent must re-register first)
- Healthy → Disconnected is prohibited (agent must pass through Unhealthy via heartbeat timeout)
- Registering → Unhealthy is prohibited (agent has no heartbeat tracking until registered)

## 5. Definitions of Done

### Agent Registration

- [x] `p1` - **ID**: `cpt-katapult-dod-agent-system-registration`

The system **MUST** accept agent registrations via gRPC AgentService.Register, validating cluster identity via Kubernetes ServiceAccount JWT, storing agent metadata (cluster ID, node name, tool versions), and returning a unique agent ID. Re-registration from the same cluster+node pair updates the existing record.

**Implements**:
- `cpt-katapult-flow-agent-system-register`
- `cpt-katapult-algo-agent-system-validate-registration`

**Covers (PRD)**:
- `cpt-katapult-fr-agent-registration`

**Covers (DESIGN)**:
- `cpt-katapult-principle-outbound-only`
- `cpt-katapult-constraint-k8s-only`
- `cpt-katapult-constraint-agent-tools`
- `cpt-katapult-component-agent-registry`
- `cpt-katapult-component-agent-runtime`
- `cpt-katapult-seq-agent-registration`
- `cpt-katapult-dbtable-agents`

**Touches**:
- API: `gRPC AgentService.Register`
- DB: `agents`
- Entities: `Agent`

### Agent Heartbeat Monitoring

- [x] `p1` - **ID**: `cpt-katapult-dod-agent-system-heartbeat`

The system **MUST** accept periodic heartbeats via gRPC AgentService.Heartbeat, update the agent's last_heartbeat timestamp and health status, refresh PVC inventory, and mark agents unhealthy when heartbeat timeout is exceeded (configurable, default 90s).

**Implements**:
- `cpt-katapult-flow-agent-system-heartbeat`
- `cpt-katapult-algo-agent-system-evaluate-health`

**Covers (PRD)**:
- `cpt-katapult-fr-agent-health`

**Covers (DESIGN)**:
- `cpt-katapult-principle-agent-autonomy`
- `cpt-katapult-component-agent-registry`
- `cpt-katapult-dbtable-agents`

**Touches**:
- API: `gRPC AgentService.Heartbeat`
- DB: `agents`
- Entities: `Agent`

### PVC Discovery

- [x] `p1` - **ID**: `cpt-katapult-dod-agent-system-pvc-discovery`

The system **MUST** discover PVCs on the agent's node by querying the Kubernetes API, resolving PV bindings (size, storage class, node affinity), filtering by configured namespace and label boundaries, and reporting the inventory to the control plane during registration and heartbeats.

**Implements**:
- `cpt-katapult-flow-agent-system-discover-pvcs`
- `cpt-katapult-algo-agent-system-discover-pvcs`

**Covers (PRD)**:
- `cpt-katapult-fr-pvc-discovery`

**Covers (DESIGN)**:
- `cpt-katapult-component-agent-runtime`
- `cpt-katapult-dbtable-agent-pvcs`

**Touches**:
- API: `Kubernetes API (client-go)`
- DB: `agent_pvcs`
- Entities: `PVCInfo`

### Agent Authentication

- [x] `p1` - **ID**: `cpt-katapult-dod-agent-system-auth`

The system **MUST** authenticate agents using Kubernetes ServiceAccount JWT tokens during gRPC registration. The control plane validates the JWT signature against the Kubernetes API server and verifies the ServiceAccount matches the expected agent identity.

**Implements**:
- `cpt-katapult-algo-agent-system-validate-registration`

**Covers (PRD)**:
- `cpt-katapult-fr-agent-auth`

**Covers (DESIGN)**:
- `cpt-katapult-principle-outbound-only`
- `cpt-katapult-constraint-k8s-only`
- `cpt-katapult-constraint-agent-tools`
- `cpt-katapult-component-agent-registry`

**Touches**:
- API: `gRPC AgentService.Register`
- Entities: `Agent`

### PVC Boundary Enforcement

- [x] `p1` - **ID**: `cpt-katapult-dod-agent-system-pvc-boundary`

The system **MUST** enforce PVC boundaries by applying configurable namespace allowlists and label selectors during PVC discovery. Only PVCs matching the configured filters are included in the agent's inventory. The system reads source and writes destination but never provisions or deletes PVCs.

**Implements**:
- `cpt-katapult-algo-agent-system-discover-pvcs`

**Covers (PRD)**:
- `cpt-katapult-fr-pvc-boundary`

**Covers (DESIGN)**:
- `cpt-katapult-component-agent-runtime`

**Touches**:
- API: `Kubernetes API (client-go)`
- Entities: `PVCInfo`

### Agent Inventory Persistence

- [x] `p1` - **ID**: `cpt-katapult-dod-agent-system-persistence`

The system **MUST** persist agent registrations and PVC inventories to PostgreSQL. The agents table stores cluster identity, node name, health status, tool versions, and heartbeat timestamps. The agent_pvcs table stores discovered PVC metadata with foreign key to the agents table and CASCADE delete.

**Implements**:
- `cpt-katapult-flow-agent-system-register`
- `cpt-katapult-flow-agent-system-heartbeat`

**Covers (PRD)**:
- `cpt-katapult-fr-agent-registration`
- `cpt-katapult-fr-agent-health`

**Covers (DESIGN)**:
- `cpt-katapult-component-agent-registry`
- `cpt-katapult-dbtable-agents`
- `cpt-katapult-dbtable-agent-pvcs`

**Touches**:
- DB: `agents`, `agent_pvcs`
- Entities: `Agent`, `PVCInfo`

## 6. Acceptance Criteria

- [ ] An agent deployed as a DaemonSet registers with the control plane via gRPC and receives a unique agent ID
- [ ] Agent registration validates the Kubernetes ServiceAccount JWT and rejects invalid tokens
- [ ] Agent re-registration from the same cluster+node updates the existing record instead of creating a duplicate
- [ ] Agents send heartbeats at a configurable interval (default 30s) and the control plane updates health status
- [ ] Agents that miss heartbeats beyond the configurable timeout (default 90s) are marked unhealthy
- [ ] Agents discover PVCs on their node with resolved PV binding, size, storage class, and node affinity
- [ ] PVC discovery respects namespace allowlist and label selector filters
- [ ] PVCs without bound PVs or with node affinity not matching the current node are excluded
- [ ] Agent and PVC data is persisted to PostgreSQL and survives control plane restarts
- [ ] Agents reconnect and re-register after control plane restart, rebuilding the agent registry

## 7. Non-Applicable Domains

**COMPL** (Compliance): Not applicable because this feature processes Kubernetes cluster metadata and blockchain volume data only. No personal data (PII) is handled. GDPR, HIPAA, PCI DSS do not apply per PRD Section 6.2 NFR Exclusions.

**UX** (Usability): Not applicable because this feature has no user-facing interface. Agent registration and heartbeat are automated system processes. Agent status is exposed to operators via the API feature (`cpt-katapult-feature-api-cli`), not this feature.

**PERF** (Performance): Not applicable as a dedicated concern because agent registration and heartbeat are lightweight gRPC calls (one per agent per 30s interval). PVC discovery queries the local Kubernetes API. No hot paths, caching strategies, or throughput targets apply. Performance NFRs for transfer operations are addressed in the Transfer Engine feature.

**OPS** (Operations): Not applicable as a standalone section because agent deployment is a standard Kubernetes DaemonSet. Observability (metrics, logging) is addressed in the Observability feature (`cpt-katapult-feature-observability`). No feature-specific configuration management or rollout strategy beyond standard Kubernetes rolling updates.
