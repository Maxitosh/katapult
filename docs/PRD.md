# PRD — Katapult

<!-- toc -->

- [1. Overview](#1-overview)
  - [1.1 Purpose](#11-purpose)
  - [1.2 Background / Problem Statement](#12-background-problem-statement)
  - [1.3 Goals (Business Outcomes)](#13-goals-business-outcomes)
  - [1.4 Glossary](#14-glossary)
- [2. Actors](#2-actors)
  - [2.1 Human Actors](#21-human-actors)
    - [Infrastructure Engineer](#infrastructure-engineer)
    - [Support Engineer](#support-engineer)
  - [2.2 System Actors](#22-system-actors)
    - [Agent](#agent)
    - [Control Plane](#control-plane)
    - [S3-Compatible Object Store](#s3-compatible-object-store)
- [3. Operational Concept & Environment](#3-operational-concept-environment)
  - [3.1 Module-Specific Environment Constraints](#31-module-specific-environment-constraints)
- [4. Scope](#4-scope)
  - [4.1 In Scope](#41-in-scope)
  - [4.2 Out of Scope](#42-out-of-scope)
- [5. Functional Requirements](#5-functional-requirements)
  - [5.1 Transfer Lifecycle](#51-transfer-lifecycle)
    - [Initiate Volume Transfer](#initiate-volume-transfer)
    - [Cancel Active Transfer](#cancel-active-transfer)
    - [Transfer Strategy Selection](#transfer-strategy-selection)
    - [Resumable Cross-Cluster Transfers](#resumable-cross-cluster-transfers)
    - [Destination Safety](#destination-safety)
    - [Retry with Backoff](#retry-with-backoff)
  - [5.2 Agent Management](#52-agent-management)
    - [Agent Auto-Registration](#agent-auto-registration)
    - [Agent Health Monitoring](#agent-health-monitoring)
    - [PVC Discovery](#pvc-discovery)
  - [5.3 Progress & Observability](#53-progress-observability)
    - [Real-Time Progress](#real-time-progress)
    - [Transfer History and Audit](#transfer-history-and-audit)
    - [Metrics and Logging](#metrics-and-logging)
    - [Actionable Error Messages](#actionable-error-messages)
  - [5.4 Security & Credentials](#54-security-credentials)
    - [Encryption by Default](#encryption-by-default)
    - [Per-Transfer Ephemeral Credentials](#per-transfer-ephemeral-credentials)
    - [Agent Authentication](#agent-authentication)
    - [User Authentication](#user-authentication)
    - [User Authorization](#user-authorization)
  - [5.5 User Interfaces](#55-user-interfaces)
    - [Web UI — Transfer Management](#web-ui-transfer-management)
    - [Web UI — Agent Overview](#web-ui-agent-overview)
    - [Web UI — Mistake Prevention](#web-ui-mistake-prevention)
    - [CLI](#cli)
    - [Control Plane API](#control-plane-api)
  - [5.6 Resource Lifecycle](#56-resource-lifecycle)
    - [Automatic Resource Cleanup](#automatic-resource-cleanup)
    - [PVC Lifecycle Boundary](#pvc-lifecycle-boundary)
    - [Transfer Autonomy from Control Plane](#transfer-autonomy-from-control-plane)
  - [5.7 Kubernetes-Native Integration](#57-kubernetes-native-integration)
    - [VolumeTransfer CRD](#volumetransfer-crd)
  - [5.8 Documentation](#58-documentation)
    - [Documentation Suite](#documentation-suite)
- [6. Non-Functional Requirements](#6-non-functional-requirements)
  - [6.1 NFR Inclusions](#61-nfr-inclusions)
    - [Transfer Throughput — Intra-Cluster](#transfer-throughput-intra-cluster)
    - [Transfer Throughput — Cross-Cluster](#transfer-throughput-cross-cluster)
    - [Bounded Failure Cost](#bounded-failure-cost)
    - [Transfer Initiation Time](#transfer-initiation-time)
    - [Progress Reporting Latency](#progress-reporting-latency)
    - [Control Plane Availability](#control-plane-availability)
    - [Control Plane State Recovery](#control-plane-state-recovery)
  - [6.2 NFR Exclusions](#62-nfr-exclusions)
- [7. Public Library Interfaces](#7-public-library-interfaces)
  - [7.1 Public API Surface](#71-public-api-surface)
    - [Control Plane REST/gRPC API](#control-plane-restgrpc-api)
    - [CLI (`katapult`)](#cli-katapult)
    - [Web UI](#web-ui)
    - [VolumeTransfer CRD](#volumetransfer-crd-1)
  - [7.2 External Integration Contracts](#72-external-integration-contracts)
    - [S3-Compatible Object Store](#s3-compatible-object-store-1)
    - [Prometheus Metrics](#prometheus-metrics)
- [8. Use Cases](#8-use-cases)
  - [8.1 Transfer Operations](#81-transfer-operations)
    - [Intra-Cluster Volume Transfer](#intra-cluster-volume-transfer)
    - [Cross-Cluster Volume Transfer](#cross-cluster-volume-transfer)
    - [Support Engineer Self-Service Transfer](#support-engineer-self-service-transfer)
    - [Cancel Active Transfer](#cancel-active-transfer-1)
- [9. Acceptance Criteria](#9-acceptance-criteria)
- [10. Dependencies](#10-dependencies)
- [11. Assumptions](#11-assumptions)
- [12. Risks](#12-risks)

<!-- /toc -->

## 1. Overview

### 1.1 Purpose

Katapult is a Kubernetes-native tool for transferring persistent volumes between worker machines within and across clusters. It eliminates the manual, error-prone process of bootstrapping blockchain nodes from volume snapshots by providing a unified control plane, automated agent coordination, and multiple transfer strategies optimized for one-shot full copies.

### 1.2 Background / Problem Statement

Deploying blockchain nodes in Kubernetes requires bootstrapping from volume snapshots to avoid multi-day/week sync times that violate SLAs. Today, transferring volumes between worker machines (intra-cluster and cross-cluster) is a fully manual, error-prone process involving ad hoc installation of rclone, temporary network exposure, and manual coordination across 10 globally distributed clusters.

The current process requires 11 manual steps: identifying a source node, SSH into the source, installing rclone, downscaling the source blockchain node, exposing volume data via temporary NodePort/Ingress, SSH into the destination, provisioning a new volume, starting the transfer, waiting hours with no progress visibility, starting the blockchain node on transferred data to verify, and cleaning up temporary services.

This process is repeated up to ~10 times per week, with volumes ranging from 200 GiB to 15 TB. Errors during any step require restarting the entire flow. Support engineers cannot perform transfers independently due to the complexity, creating a bottleneck on infrastructure engineers.

**Target Users**:

- Infrastructure engineers managing blockchain nodes across clusters
- Support engineers handling node deployment tasks

**Key Problems Solved**:

- Manual SSH/rclone installation and temporary service creation per transfer (~30-60 minutes setup)
- No progress visibility during multi-hour transfers
- No resume capability — failures require full restart
- Support engineers cannot self-serve transfers
- No audit trail or centralized management across 10 clusters

**Critical Workload Characteristic**: Every transfer is a one-shot full copy. There are no incremental syncs — once data is transferred, the destination node runs independently.

### 1.3 Goals (Business Outcomes)

**Success Criteria**:

- Transfer initiation time reduced from ~30-60 minutes of manual work to under 2 minutes (Baseline: 30-60 min manual; Target: <2 min; Timeframe: v0.1)
- Zero manual SSH or rclone installation required for any transfer (Baseline: 11-step manual process; Target: 0 manual steps; Timeframe: v0.1)
- Support engineers independently perform transfers after a 10-minute onboarding (Baseline: only infra engineers can transfer; Target: support self-service; Timeframe: v0.2)
- Intra-cluster 5 TB transfer completes in under 30 minutes on 10 Gbps network (Baseline: not measured; Target: <30 min; Timeframe: v0.1)
- Cross-cluster 5 TB transfer completes within 12 hours with chunk-level resume on failure (Baseline: full restart on failure; Target: bounded failure cost; Timeframe: v0.1)
- Zero data corruption incidents attributable to the tool (Baseline: N/A; Target: 0; Timeframe: ongoing)

**Capabilities**:

- Initiate and manage volume transfers between any PVCs across clusters
- Monitor transfer progress in real time with speed, ETA, and failure details
- Resume interrupted cross-cluster transfers without restarting from zero
- Encrypt all transfers by default
- Provide web UI for non-specialist self-service and CLI for terminal users

### 1.4 Glossary

| Term | Definition |
|------|------------|
| PVC | Persistent Volume Claim — a Kubernetes resource representing a storage request |
| PV | Persistent Volume — the actual storage resource backing a PVC |
| Mover | An internal transfer strategy implementation (e.g., stream-based, object-store-based, rsync-based) |
| Agent | A lightweight process running on each worker node that executes transfers |
| Control Plane | The central coordination service managing agents, transfers, and credentials |
| Transfer | A single operation that copies all data from a source PVC to a destination PVC |
| Chunk | A fixed-size segment of compressed transfer data staged in object storage |
| Node Affinity | The binding of a local PV to a specific Kubernetes node |
| CRD | Custom Resource Definition — extends the Kubernetes API with custom resources |
| Stunnel | A TLS tunneling proxy used to encrypt direct agent-to-agent connections |

## 2. Actors

### 2.1 Human Actors

#### Infrastructure Engineer

**ID**: `cpt-katapult-actor-infra-engineer`

**Role**: Deploys and manages blockchain nodes across clusters. Understands Kubernetes concepts (PVCs, nodes, clusters). Currently performs manual rclone transfers.
**Needs**: Speed, reliability, clear error reporting, CLI access, ability to override transfer strategy.

#### Support Engineer

**ID**: `cpt-katapult-actor-support-engineer`

**Role**: Occasionally handles node deployment tasks. Technically capable but not a Kubernetes specialist.
**Needs**: Simple web UI, guided workflow, no room for misconfiguration, transfer progress visibility.

### 2.2 System Actors

#### Agent

**ID**: `cpt-katapult-actor-agent`

**Role**: Lightweight process deployed on every participating worker node. Initiates outbound connection to the control plane, executes transfers, manages temporary Kubernetes resources, and reports progress.

#### Control Plane

**ID**: `cpt-katapult-actor-control-plane`

**Role**: Central coordination service. Exposes API and Web UI, manages transfer state, maintains agent registry, issues per-transfer credentials, and aggregates progress.

#### S3-Compatible Object Store

**ID**: `cpt-katapult-actor-object-store`

**Role**: External storage service used as an intermediate staging area for cross-cluster transfers. Agents upload and download transfer data via S3-compatible API.

## 3. Operational Concept & Environment

### 3.1 Module-Specific Environment Constraints

- Requires Kubernetes clusters with node-local storage (TopoLVM, OpenEBS, or similar CSI drivers)
- Agents require `tar` (GNU tar ≥ 1.28 with `--sort` support), `zstd`, and `stunnel` on worker nodes
- Control plane requires outbound S3 API access for cross-cluster transfers
- Agents initiate outbound connections to the control plane — no inbound connectivity to clusters required
- Deployed as Kubernetes workloads: DaemonSet (agents) and Deployment (control plane)

## 4. Scope

### 4.1 In Scope

- Volume transfer initiation, monitoring, cancellation, and retry between PVCs
- Intra-cluster and cross-cluster transfers across globally distributed clusters
- Multiple transfer strategies with automatic selection based on topology
- Encrypted transfers by default for all transfer paths
- Web UI for guided transfer workflows and monitoring
- CLI for terminal-based transfer operations
- REST/gRPC API for programmatic access
- Real-time progress reporting with speed, ETA, and chunk-level detail
- Agent registration, health monitoring, and PVC discovery
- Per-transfer ephemeral credentials (no long-lived secrets)
- Automatic cleanup of all transfer-created resources
- PVC-first addressing with raw path fallback
- Resume capability for cross-cluster transfers

### 4.2 Out of Scope

- **Snapshot lifecycle orchestration**: The tool does NOT manage when to downscale nodes, take snapshots, or start nodes on new data. That remains the operator's or external automation's responsibility.
- **Source discovery / catalog**: The tool does NOT maintain an inventory of available snapshots or help find a suitable source. Engineers use existing backoffice or GitOps repos.
- **Bandwidth throttling / QoS**: No prioritization or rate-limiting between concurrent transfers in v1. Concurrent transfers share resources naturally.
- **Guard rails against misuse**: The tool does NOT prevent transfers from production-serving nodes. That judgment remains with the operator.
- **Automation triggers / webhooks**: v1 focuses on human-initiated transfers. The architecture must not preclude API-driven automation in the future.
- **Replacing the storage layer**: The tool works with whatever CSI drivers are already deployed. It does not require adopting a new storage system.
- **Incremental / delta sync**: The tool is optimized for one-shot full copies. It does not maintain sync state between transfers or optimize for "update an existing copy."
- **Control plane high availability**: v1 assumes a single control plane replica. HA (leader election, state replication) is deferred to post-v1.
- **Multi-tenancy**: Namespaced isolation of transfers and agents is deferred to post-v1.

## 5. Functional Requirements

> **Testing strategy**: All requirements verified via automated tests (unit, integration, e2e) targeting 90%+ code coverage unless otherwise specified. Document verification method only for non-test approaches (analysis, inspection, demonstration).

### 5.1 Transfer Lifecycle

#### Initiate Volume Transfer

- [ ] `p1` - **ID**: `cpt-katapult-fr-initiate-transfer`

The system **MUST** allow initiating a volume transfer by specifying a source PVC (cluster + PVC name) and a destination PVC (cluster + PVC name). The system **MUST** validate that both PVCs exist and are accessible by registered agents before starting the transfer.

**Rationale**: Core capability — replaces the 11-step manual process with a single operation.

**Actors**: `cpt-katapult-actor-infra-engineer`, `cpt-katapult-actor-support-engineer`

#### Cancel Active Transfer

- [ ] `p1` - **ID**: `cpt-katapult-fr-cancel-transfer`

The system **MUST** allow cancelling an active transfer. Cancellation **MUST** trigger cleanup of all resources created for that transfer (temporary services, credentials, staged data).

**Rationale**: Operators need to abort transfers that are no longer needed without leaving orphaned resources.

**Actors**: `cpt-katapult-actor-infra-engineer`, `cpt-katapult-actor-support-engineer`

#### Transfer Strategy Selection

- [ ] `p1` - **ID**: `cpt-katapult-fr-strategy-selection`

The system **MUST** automatically select a transfer strategy based on topology: intra-cluster transfers **MUST** use a fast streaming strategy; cross-cluster transfers **MUST** use an object-store-staged strategy when S3 is configured; cross-cluster transfers without S3 **MUST** fall back to a direct file-level transfer strategy. The system **MUST** allow operators to override the automatic selection.

**Rationale**: Different network topologies require different optimal strategies. Auto-selection reduces operator burden; manual override preserves flexibility.

**Actors**: `cpt-katapult-actor-infra-engineer`

#### Resumable Cross-Cluster Transfers

- [ ] `p1` - **ID**: `cpt-katapult-fr-resumable-transfer`

The system **MUST** support chunk-level resume for cross-cluster transfers staged through object storage. A failure during transfer **MUST NOT** require restarting the entire operation — only the incomplete portion **MUST** be retried.

**Rationale**: Cross-cluster transfers of multi-TB volumes over the public internet are prone to failures. Without resume, a failure at 80% wastes hours of work.

**Actors**: `cpt-katapult-actor-agent`

#### Destination Safety

- [ ] `p1` - **ID**: `cpt-katapult-fr-destination-safety`

A failed transfer **MUST** leave the destination PVC in a safe state — incomplete data **MUST NOT** contaminate the destination volume. The destination PVC **MUST** be empty by default; non-empty destinations **MUST** require explicit operator opt-in (`allowOverwrite`).

**Rationale**: Operators must be able to retry transfers without worrying about corruption. Requiring opt-in for overwrite prevents accidental data loss.

**Actors**: `cpt-katapult-actor-infra-engineer`

#### Retry with Backoff

- [ ] `p2` - **ID**: `cpt-katapult-fr-retry-backoff`

The system **MUST** support configurable retry with exponential backoff for failed transfer phases. The maximum number of retry attempts and backoff parameters **MUST** be configurable per transfer.

**Rationale**: Transient failures (network blips, temporary S3 unavailability) should be handled automatically without operator intervention.

**Actors**: `cpt-katapult-actor-control-plane`

### 5.2 Agent Management

#### Agent Auto-Registration

- [ ] `p1` - **ID**: `cpt-katapult-fr-agent-registration`

Agents **MUST** register with the control plane automatically on startup by initiating an outbound connection. The control plane **MUST NOT** initiate connections into clusters. Agent registration **MUST** report cluster identity, node name, and available PVCs.

**Rationale**: Outbound-only connections eliminate firewall changes across 10 clusters. Automatic registration removes manual agent setup.

**Actors**: `cpt-katapult-actor-agent`, `cpt-katapult-actor-control-plane`

#### Agent Health Monitoring

- [ ] `p1` - **ID**: `cpt-katapult-fr-agent-health`

The control plane **MUST** monitor agent health via heartbeats and mark agents as unhealthy after a configurable heartbeat timeout. The system **MUST** display agent health status, last heartbeat time, and available tool versions.

**Rationale**: Operators need visibility into which agents are available before initiating transfers.

**Actors**: `cpt-katapult-actor-control-plane`

#### PVC Discovery

- [ ] `p1` - **ID**: `cpt-katapult-fr-pvc-discovery`

Agents **MUST** report available PVCs (with size, storage class, and node affinity) to the control plane. The system **MUST** resolve PVC names to bound PVs and determine which node holds the data, handling node-local storage constraints transparently.

**Rationale**: PVC-first addressing aligns with Kubernetes ecosystem norms and is safer for non-specialist operators than raw filesystem paths.

**Actors**: `cpt-katapult-actor-agent`

### 5.3 Progress & Observability

#### Real-Time Progress

- [ ] `p1` - **ID**: `cpt-katapult-fr-realtime-progress`

The system **MUST** provide real-time transfer progress including bytes transferred, total bytes, transfer speed, and ETA. For object-store-staged transfers, the system **MUST** additionally report chunk-level progress (N/M chunks completed).

**Rationale**: The current process has zero progress visibility during multi-hour transfers. Real-time feedback lets operators detect stalls early and plan around completion times.

**Actors**: `cpt-katapult-actor-infra-engineer`, `cpt-katapult-actor-support-engineer`

#### Transfer History and Audit

- [ ] `p2` - **ID**: `cpt-katapult-fr-transfer-history`

The system **MUST** maintain a transfer audit log recording who initiated each transfer, when, source, destination, transfer strategy used, outcome, duration, and bytes transferred.

**Rationale**: Audit trail is needed for operational accountability across 10 clusters and multiple operators.

**Actors**: `cpt-katapult-actor-infra-engineer`

#### Metrics and Logging

- [ ] `p2` - **ID**: `cpt-katapult-fr-metrics-logging`

The system **MUST** expose Prometheus metrics for transfer throughput, duration, success/failure rates, and agent health. The system **MUST** produce structured JSON logs with per-transfer correlation IDs from both agents and control plane.

**Rationale**: Integration with existing Prometheus/Grafana monitoring stack for alerting and dashboards.

**Actors**: `cpt-katapult-actor-control-plane`, `cpt-katapult-actor-agent`

#### Actionable Error Messages

- [ ] `p1` - **ID**: `cpt-katapult-fr-actionable-errors`

All transfer failures **MUST** surface actionable error messages that include the failure reason and a suggested remediation (e.g., "Destination disk full: 2.1 TB available, 5.3 TB required").

**Rationale**: Support engineers need to diagnose and resolve transfer failures without escalating to infrastructure engineers.

**Actors**: `cpt-katapult-actor-support-engineer`

### 5.4 Security & Credentials

#### Encryption by Default

- [ ] `p1` - **ID**: `cpt-katapult-fr-encryption-default`

All transfers **MUST** be encrypted by default. Direct agent-to-agent transfers **MUST** use TLS. Object-store-staged transfers **MUST** use HTTPS (TLS in transit). Encryption **MUST** only be disabled via explicit operator opt-out.

**Rationale**: Cross-cluster traffic traverses the public internet. Encryption-by-default prevents accidental plaintext transfers of sensitive blockchain state.

**Actors**: `cpt-katapult-actor-infra-engineer`

#### Per-Transfer Ephemeral Credentials

- [ ] `p1` - **ID**: `cpt-katapult-fr-ephemeral-credentials`

The system **MUST** issue fresh, scoped, short-lived credentials for each transfer. Credentials **MUST** be useless after the transfer completes. The system **MUST NOT** use long-lived shared secrets for data transfer.

**Rationale**: Per-transfer credentials limit blast radius of credential compromise and eliminate credential rotation burden.

**Actors**: `cpt-katapult-actor-control-plane`

#### Agent Authentication

- [ ] `p1` - **ID**: `cpt-katapult-fr-agent-auth`

Agents **MUST** authenticate to the control plane using cluster identity credentials. The control plane **MUST** verify agent identity before accepting registration or issuing transfer commands.

**Rationale**: Prevents unauthorized agents from joining the cluster mesh or initiating transfers.

**Actors**: `cpt-katapult-actor-agent`, `cpt-katapult-actor-control-plane`

#### User Authentication

- [ ] `p1` - **ID**: `cpt-katapult-fr-user-auth`

The system **MUST** require authentication for all human access to the Web UI and API. The system **MUST** support local username/password authentication as a baseline. The system **MUST** support SSO integration via OIDC-compatible identity providers for enterprise environments. Authenticated sessions **MUST** expire after a configurable idle timeout (default: 30 minutes).

**Rationale**: Human operators initiate transfers of multi-TB blockchain data across production clusters. Unauthenticated access would allow unauthorized transfers and data exposure.

**Actors**: `cpt-katapult-actor-infra-engineer`, `cpt-katapult-actor-support-engineer`

#### User Authorization

- [ ] `p1` - **ID**: `cpt-katapult-fr-user-authz`

The system **MUST** enforce role-based access control with at least two roles: **operator** (full access: initiate, monitor, cancel transfers; override transfer strategy; manage agent configuration) and **viewer** (monitor transfers, view agent status; no transfer initiation or cancellation). Infrastructure engineers **MUST** be assignable to the operator role. Support engineers **MUST** be assignable to either role based on organizational policy. Role assignment **MUST** be configurable by administrators.

**Rationale**: Support engineers need guided, bounded access to prevent accidental misconfiguration. Infrastructure engineers need full operational control including strategy overrides and agent management.

**Actors**: `cpt-katapult-actor-infra-engineer`, `cpt-katapult-actor-support-engineer`

### 5.5 User Interfaces

#### Web UI — Transfer Management

- [ ] `p1` - **ID**: `cpt-katapult-fr-web-ui-transfers`

The system **MUST** provide a web UI for initiating transfers with guided PVC selection (cluster → node → PVC dropdowns), monitoring active and recent transfers with progress bars and speed indicators, viewing transfer detail with full event timeline, and cancelling active transfers.

**Rationale**: Support engineers need a simple, guided interface that prevents misconfiguration and requires no Kubernetes CLI knowledge.

**Actors**: `cpt-katapult-actor-support-engineer`, `cpt-katapult-actor-infra-engineer`

#### Web UI — Agent Overview

- [ ] `p2` - **ID**: `cpt-katapult-fr-web-ui-agents`

The system **MUST** provide a web UI view of registered agents grouped by cluster, showing node name, health status, last heartbeat, available movers, tool versions, and visible PVCs with size and storage class.

**Rationale**: Operators need to verify agent availability and capabilities before initiating transfers.

**Actors**: `cpt-katapult-actor-infra-engineer`

#### Web UI — Mistake Prevention

- [ ] `p2` - **ID**: `cpt-katapult-fr-web-ui-validation`

The Web UI **MUST** prevent obvious mistakes by validating that source and destination are different, displaying a warning when destination PVC is smaller than source, requiring confirmation for destructive actions, and explaining why a particular transfer strategy was auto-selected.

**Rationale**: Support engineers are not Kubernetes specialists — the UI must guard against misconfiguration.

**Actors**: `cpt-katapult-actor-support-engineer`

#### CLI

- [ ] `p1` - **ID**: `cpt-katapult-fr-cli`

The system **MUST** provide a CLI (`katapult`) or kubectl plugin for initiating transfers, checking transfer status, listing active transfers, cancelling transfers, and listing agents with health status. The CLI **MUST** support explicit transfer strategy override and S3 configuration options.

**Rationale**: Infrastructure engineers prefer terminal workflows integrated with their existing kubectl-based tooling.

**Actors**: `cpt-katapult-actor-infra-engineer`

#### Control Plane API

- [ ] `p1` - **ID**: `cpt-katapult-fr-api`

The control plane **MUST** expose a REST/gRPC API as the single source of truth for all operations. Both the Web UI and CLI **MUST** be clients of this API. The API **MUST** support transfer CRUD, agent registry queries, and progress streaming.

**Rationale**: API-first design ensures future automation (backoffice integration, CI/CD triggers) can drive transfers programmatically without changes to the core system.

**Actors**: `cpt-katapult-actor-control-plane`

### 5.6 Resource Lifecycle

#### Automatic Resource Cleanup

- [ ] `p1` - **ID**: `cpt-katapult-fr-resource-cleanup`

Every resource created for a transfer (Kubernetes Services, Secrets, S3 objects, temporary directories) **MUST** be cleaned up when the transfer reaches a terminal state (Completed, Failed, Cancelled). Cleanup **MUST** be enforced by the system, not by operator memory.

**Rationale**: Orphaned resources across 10 clusters create operational debt and security exposure (open LoadBalancers, lingering S3 data).

**Actors**: `cpt-katapult-actor-control-plane`, `cpt-katapult-actor-agent`

#### PVC Lifecycle Boundary

- [ ] `p1` - **ID**: `cpt-katapult-fr-pvc-boundary`

The system **MUST NOT** provision or delete PVCs. Volume lifecycle (creation, resizing, deletion) **MUST** remain the operator's or external orchestration's responsibility. The system **MUST** read from a source volume and write to a destination volume, both of which must exist before the transfer starts.

**Rationale**: Clear responsibility boundary — the tool transfers data, it does not manage storage lifecycle.

**Actors**: `cpt-katapult-actor-infra-engineer`

#### Transfer Autonomy from Control Plane

- [ ] `p1` - **ID**: `cpt-katapult-fr-transfer-autonomy`

Once an agent receives a transfer command and the data path is established, the transfer **MUST** complete successfully without further control plane communication. A control plane outage **MUST NOT** interrupt an in-progress transfer.

**Rationale**: The control plane coordinates, but must not be a single point of failure for active transfers.

**Actors**: `cpt-katapult-actor-agent`

### 5.7 Kubernetes-Native Integration

#### VolumeTransfer CRD

- [ ] `p1` - **ID**: `cpt-katapult-fr-crd`

The system **MUST** model each transfer as a Kubernetes Custom Resource (`VolumeTransfer`) with a status subresource. The CRD **MUST** support declarative specification of source, destination, transfer options, and retry configuration. Transfer status **MUST** be queryable via standard `kubectl get` commands.

**Rationale**: The CRD/controller pattern follows the Kubernetes operator model, enabling reconciliation, GitOps integration, and standard K8s tooling.

**Actors**: `cpt-katapult-actor-infra-engineer`

### 5.8 Documentation

#### Documentation Suite

- [ ] `p2` - **ID**: `cpt-katapult-fr-documentation`

The system **MUST** provide: CLI help text and usage examples built into the `katapult` binary, API reference documentation (OpenAPI specification for REST endpoints), a support engineer onboarding guide sufficient for completing a first transfer within 10 minutes, and a deployment/operations guide covering installation, agent setup, S3 configuration, and troubleshooting.

**Rationale**: The 10-minute onboarding target for support engineers requires documented guides. OSS adoption depends on clear documentation for deployment and operation.

**Actors**: `cpt-katapult-actor-infra-engineer`, `cpt-katapult-actor-support-engineer`

## 6. Non-Functional Requirements

### 6.1 NFR Inclusions

#### Transfer Throughput — Intra-Cluster

- [ ] `p1` - **ID**: `cpt-katapult-nfr-throughput-intra`

The system **MUST** transfer a 5 TB volume between nodes in the same cluster in under 30 minutes on a 10 Gbps network.

**Threshold**: 5 TB in <30 minutes (~2.8 GB/s effective throughput) on 10 Gbps network

**Rationale**: Intra-cluster transfers are the most frequent operation; fast transfers minimize blockchain node downtime.

#### Transfer Throughput — Cross-Cluster

- [ ] `p1` - **ID**: `cpt-katapult-nfr-throughput-cross`

The system **MUST** transfer a 5 TB volume between clusters within 12 hours including upload and download phases, with chunk-level resume on failure.

**Threshold**: 5 TB in <12 hours cross-cluster with resume capability

**Rationale**: Cross-cluster transfers traverse the public internet with higher failure risk; 12-hour SLA with resume ensures predictable completion.

#### Bounded Failure Cost

- [ ] `p1` - **ID**: `cpt-katapult-nfr-bounded-failure`

The worst-case wasted work on failure **MUST** be bounded: at most one chunk (default 4 GiB) for cross-cluster transfers, or one pipeline run for intra-cluster transfers. A failure **MUST NOT** waste the entire multi-hour transfer.

**Threshold**: Max wasted work ≤ 1 chunk (4 GiB) for cross-cluster, ≤ 1 pipeline run for intra-cluster

**Rationale**: Unbounded failure cost makes SLA compliance unpredictable for multi-TB transfers over unreliable networks.

#### Transfer Initiation Time

- [ ] `p1` - **ID**: `cpt-katapult-nfr-initiation-time`

A transfer **MUST** be initiatable (from UI or CLI interaction to transfer starting) in under 2 minutes.

**Threshold**: <2 minutes from user action to transfer data movement beginning

**Rationale**: Replaces the current 30-60 minute manual setup process.

#### Progress Reporting Latency

- [ ] `p2` - **ID**: `cpt-katapult-nfr-progress-latency`

Transfer progress updates **MUST** be visible in the Web UI within 5 seconds of the agent reporting them.

**Threshold**: ≤5 seconds end-to-end latency (agent → control plane → UI)

**Rationale**: Real-time progress is a core value proposition; stale progress undermines operator confidence.

#### Control Plane Availability

- [ ] `p2` - **ID**: `cpt-katapult-nfr-cp-availability`

The control plane targets best-effort availability with single-replica deployment in v1. Planned maintenance **MUST** be communicated to operators in advance. Active transfers **MUST** continue during control plane downtime (per FR `cpt-katapult-fr-transfer-autonomy`). New transfer initiation is unavailable during control plane downtime.

**Threshold**: Best-effort (single replica, no HA); active transfers unaffected by CP outage

**Rationale**: Explicit acknowledgment of v1 availability limitations. HA is deferred to post-v1 (see Out of Scope).

#### Control Plane State Recovery

- [ ] `p2` - **ID**: `cpt-katapult-nfr-cp-recovery`

The control plane state (transfer history, agent registry, configuration) **MUST** be recoverable after a full data loss. Agent registry **MUST** be rebuilt automatically when agents reconnect. Transfer history RPO is best-effort in v1 — operators accept potential loss of historical records. The control plane **MUST** be restorable to operational state (accepting new transfers) within 30 minutes of a clean redeployment.

**Threshold**: RTO <30 minutes (clean redeploy); RPO best-effort for transfer history; agent registry auto-rebuilds

**Rationale**: Single-replica deployment means control plane data is vulnerable to PV failure. Explicit recovery expectations enable operations planning.

### 6.2 NFR Exclusions

- **Accessibility** (UX-PRD-002): Not applicable — Katapult is a Kubernetes infrastructure tool used by infrastructure and support engineers via CLI and an internal web UI, not a public-facing web application. Users have standard desktop browsers and technical proficiency.
- **Internationalization** (UX-PRD-003): Not applicable — English-only for all interfaces. Chainstack infrastructure team operates in English; OSS adopters are Kubernetes infrastructure engineers who work in English.
- **Inclusivity** (UX-PRD-005): Not applicable — narrow technical audience (Kubernetes infrastructure engineers and support staff).
- **Regulatory Compliance** (COMPL-PRD-001/002/003): Not applicable — Katapult transfers blockchain state data (block headers, transaction indexes, state tries), not personal data. No PII processing, no financial data handling, no healthcare data. GDPR, HIPAA, PCI DSS, SOX do not apply.
- **Privacy by Design** (SEC-PRD-005): Not applicable — Katapult does not process personal data. Volume data is blockchain state owned by the operator.
- **Safety** (SAFE-PRD-001/002): Not applicable — Katapult is a data transfer tool with no physical interaction, no control of physical systems, and no potential to cause harm to people, property, or environment.
- **Offline Capability** (UX-PRD-004): Not applicable — Katapult requires Kubernetes cluster connectivity and network access by design. The control plane and agents are server-side components.
- **Device/Platform** (UX-PRD-004): Not applicable beyond desktop browser — Web UI is accessed from operator workstations only. No mobile or responsive design requirements.

## 7. Public Library Interfaces

### 7.1 Public API Surface

#### Control Plane REST/gRPC API

- [ ] `p1` - **ID**: `cpt-katapult-interface-api`

**Type**: REST/gRPC API

**Stability**: unstable (v1alpha1)

**Description**: Primary API for all transfer operations, agent management, and progress queries. Both the Web UI and CLI are clients of this API.

**Breaking Change Policy**: API is versioned via K8s API group (`katapult.io/v1alpha1`). Breaking changes require API version bump.

#### CLI (`katapult`)

- [ ] `p1` - **ID**: `cpt-katapult-interface-cli`

**Type**: CLI (standalone binary or kubectl plugin)

**Stability**: unstable

**Description**: Terminal interface for transfer operations (start, status, list, cancel, agents). Supports explicit transfer strategy override and S3 configuration.

**Breaking Change Policy**: Command syntax follows semver. Breaking CLI changes require major version bump.

#### Web UI

- [ ] `p1` - **ID**: `cpt-katapult-interface-web-ui`

**Type**: Web UI (served from control plane)

**Stability**: unstable

**Description**: Browser-based interface for guided transfer workflows, real-time dashboard, transfer detail, and agent overview. Primary interface for support engineers.

**Breaking Change Policy**: UI follows semver. Major UX changes documented in release notes.

#### VolumeTransfer CRD

- [ ] `p1` - **ID**: `cpt-katapult-interface-crd`

**Type**: Kubernetes CRD (katapult.io/v1alpha1)

**Stability**: unstable (v1alpha1)

**Description**: Declarative Kubernetes resource for transfer specification and status. Supports spec (source, destination, options, retry) and status subresource (phase, progress, errors).

**Breaking Change Policy**: CRD follows Kubernetes API versioning conventions. Breaking changes require API version bump (v1alpha1 → v1beta1 → v1).

### 7.2 External Integration Contracts

#### S3-Compatible Object Store

- [ ] `p2` - **ID**: `cpt-katapult-contract-s3`

**Direction**: required from external system

**Protocol/Format**: S3-compatible API (AWS S3, MinIO, or compatible)

**Compatibility**: Must support PutObject, GetObject, DeleteObject, ListObjectsV2, CreateMultipartUpload. STS AssumeRole required for default credential model; presigned URL fallback for non-STS environments.

#### Prometheus Metrics

- [ ] `p2` - **ID**: `cpt-katapult-contract-prometheus`

**Direction**: provided by Katapult

**Protocol/Format**: Prometheus exposition format (HTTP `/metrics` endpoint)

**Compatibility**: Standard Prometheus scrape interface.

## 8. Use Cases

### 8.1 Transfer Operations

#### Intra-Cluster Volume Transfer

- [ ] `p1` - **ID**: `cpt-katapult-usecase-intra-transfer`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Preconditions**:
- Source and destination PVCs exist in the same cluster
- Agents are registered and healthy on both source and destination nodes
- Source blockchain node has been downscaled (pod stopped)

**Main Flow**:
1. Engineer selects source PVC and destination PVC via CLI or Web UI
2. System validates PVCs exist and agents are available on the relevant nodes
3. System auto-selects streaming transfer strategy (same cluster)
4. System issues per-transfer credentials and initiates transfer
5. Engineer monitors progress in real time (bytes, speed, ETA)
6. Transfer completes; system cleans up temporary resources
7. Engineer starts blockchain node on destination data

**Postconditions**: Destination PVC contains a complete copy of source data. All transfer-created resources are cleaned up.

**Alternative Flows**:
- **Destination PVC not empty**: System rejects transfer with actionable error unless `allowOverwrite` is set
- **Transfer fails mid-stream**: System retries automatically; on retry exhaustion, reports actionable error with failure reason
- **Agent unavailable**: System reports which agent is unreachable and on which node

#### Cross-Cluster Volume Transfer

- [ ] `p1` - **ID**: `cpt-katapult-usecase-cross-transfer`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Preconditions**:
- Source and destination PVCs exist in different clusters
- S3 bucket is configured and accessible
- Agents are registered in both clusters

**Main Flow**:
1. Engineer specifies source PVC (cluster + PVC name) and destination PVC (cluster + PVC name)
2. System auto-selects object-store-staged transfer strategy (cross-cluster + S3 configured)
3. System issues scoped S3 credentials for this transfer
4. Source agent compresses and uploads data as chunks to S3
5. Engineer monitors upload progress (chunks uploaded, speed)
6. Destination agent downloads chunks in parallel, verifies checksums, and reassembles
7. Engineer monitors download progress (chunks downloaded, extraction progress)
8. Transfer completes; system cleans up S3 objects and temporary directories

**Postconditions**: Destination PVC contains a verified copy of source data. S3 objects deleted. Temporary directories removed.

**Alternative Flows**:
- **Source agent crashes at 80% upload**: On restart, agent resumes from first missing chunk — does not re-upload completed chunks
- **Destination agent crashes mid-download**: On restart, agent verifies already-downloaded chunks and downloads only missing ones
- **S3 temporarily unavailable**: S3 SDK retries with backoff; transfer stalls but does not fail immediately

#### Support Engineer Self-Service Transfer

- [ ] `p1` - **ID**: `cpt-katapult-usecase-support-transfer`

**Actor**: `cpt-katapult-actor-support-engineer`

**Preconditions**:
- Support engineer has Web UI access
- Source and destination clusters have healthy agents

**Main Flow**:
1. Support engineer opens Web UI
2. Selects source: cluster → node → PVC (dropdowns populated from agent data)
3. Selects destination: cluster → node → PVC
4. Reviews auto-selected transfer strategy and options (system explains selection)
5. Confirms and starts transfer
6. Monitors progress on dashboard (progress bar, speed, ETA)
7. Transfer completes; support engineer sees success confirmation

**Postconditions**: Transfer completed without requiring infrastructure engineer involvement.

**Alternative Flows**:
- **Source and destination are the same PVC**: UI prevents submission with inline validation error
- **Destination PVC too small**: UI displays warning with size comparison before confirmation
- **Transfer fails**: UI shows actionable error message with suggested remediation

#### Cancel Active Transfer

- [ ] `p2` - **ID**: `cpt-katapult-usecase-cancel-transfer`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Preconditions**:
- A transfer is in progress

**Main Flow**:
1. Engineer identifies transfer to cancel (via dashboard or `katapult list`)
2. Engineer requests cancellation (UI button or `katapult cancel <id>`)
3. System confirms cancellation request
4. System signals agents to stop transfer
5. System cleans up all resources (services, S3 objects, staging directories, credentials)
6. Transfer status updated to Cancelled

**Postconditions**: Transfer stopped. All associated resources cleaned up. Destination PVC in safe state (no partial data in main data directory).

**Alternative Flows**:
- **Agent unreachable during cancellation**: Controller cleans up resources it controls (services, CRD status); orphaned agent-side resources cleaned up when agent reconnects

## 9. Acceptance Criteria

- [ ] A transfer is initiatable from Web UI or CLI in under 2 minutes with no manual SSH, rclone installation, or temporary service creation
- [ ] Support engineers can independently initiate and monitor a transfer after a 10-minute onboarding session
- [ ] A 5 TB intra-cluster transfer completes in under 30 minutes on a 10 Gbps network
- [ ] A 5 TB cross-cluster transfer completes within 12 hours with chunk-level resume on failure
- [ ] All transfers are encrypted by default (TLS for direct transfers, HTTPS for S3)
- [ ] A failed transfer leaves the destination PVC in a safe state — no data corruption
- [ ] Worst-case wasted work on failure is bounded: one chunk (cross-cluster) or one pipeline run (intra-cluster)
- [ ] All transfer-created resources (services, secrets, S3 objects, temp directories) are cleaned up on terminal state
- [ ] Real-time progress (bytes, speed, ETA, chunk progress) is visible in Web UI and CLI during active transfers

## 10. Dependencies

| Dependency | Description | Criticality |
|------------|-------------|-------------|
| Kubernetes clusters | Target deployment environment with CSI drivers (TopoLVM, OpenEBS, etc.) | p1 |
| S3-compatible object store | Intermediate staging for cross-cluster transfers | p1 (cross-cluster) |
| GNU tar ≥ 1.28 | Required on agent nodes for deterministic streaming (`--sort` support) | p1 |
| zstd | Compression/decompression on agent nodes | p1 |
| stunnel | TLS tunneling for direct agent-to-agent transfers | p1 (direct movers) |
| pv-migrate | Immediate stopgap tool to reduce manual effort while Katapult is built | p2 |

## 11. Assumptions

- All participating Kubernetes clusters allow outbound connections from agent pods to the control plane endpoint (no egress firewall blocking)
- Worker nodes have GNU tar ≥ 1.28, zstd, and stunnel available (or these can be bundled in the agent container image)
- S3-compatible object storage is available and accessible from all clusters for cross-cluster transfers
- Source blockchain nodes are downscaled (pods stopped) before transfer initiation — the tool does not manage this
- Both source and destination PVCs exist and are provisioned before transfer initiation
- Node-local storage (TopoLVM, OpenEBS) creates node-affinity constraints that the agent resolves locally
- The control plane has a single publicly reachable endpoint that all agents can connect to

**Open Questions**

| # | Question | Owner | Target Date | Impact if Unresolved |
|---|----------|-------|-------------|----------------------|
| 1 | S3 bucket topology: shared bucket across all clusters or per-cluster buckets? | Infra team | v0.1 design phase | Cross-cluster transfer architecture blocked |
| 2 | S3 credential model: require STS support or support presigned URL fallback from v0.1? | Infra team | v0.1 design phase | OSS adopters on non-AWS S3 stores cannot use cross-cluster transfers |
| 3 | User authentication method: standalone identity store or delegate to external OIDC provider? | Platform team | v0.1 design phase | Web UI and API access control cannot be implemented |

## 12. Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| S3 bucket topology decision delayed | Cross-cluster transfers blocked until bucket strategy (shared vs per-cluster) is decided | Decide during v0.1 implementation; start with single shared bucket, migrate later if needed |
| GNU tar version mismatch across nodes | Source-side resume unavailable on nodes without `--sort` support; transfers degrade to full restart on failure | Mandate GNU tar ≥ 1.28 as deployment prerequisite; agent verifies at startup and warns |
| Control plane single point of failure | No new transfers during CP downtime (active transfers continue) | Acceptable for v1; HA deferred to post-v1 with explicit scope in Out of Scope |
| S3 credential model compatibility | OSS adopters using non-AWS S3-compatible stores may lack STS support | Presigned URL fallback planned for v0.2; document S3 compatibility requirements for v0.1 |
| Agent container image size | Bundling tar, zstd, stunnel increases image size, affecting DaemonSet rollout time | Use multi-stage builds; evaluate static linking; measure impact during v0.1 |
| Fan-out transfers not in v1 | Operators requesting "upload once, download to 5 clusters" must create 5 separate transfers | ObjectStoreMover architecture naturally supports fan-out; defer CRD-level support to v0.3 |
| Support engineer adoption | Support engineers may resist new tool if UX is not simple enough | Invest in UX design for Web UI; validate with support team during v0.2; 10-minute onboarding target |
