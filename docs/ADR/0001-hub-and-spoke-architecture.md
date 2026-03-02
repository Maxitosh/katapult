---
status: proposed
date: 2026-03-01
decision-makers: Infrastructure team
---

# ADR-0001: Use Hub-and-Spoke Architecture for Multi-Cluster Transfer Coordination

**ID**: `cpt-katapult-adr-hub-and-spoke`

<!-- toc -->

- [Context and Problem Statement](#context-and-problem-statement)
- [Decision Drivers](#decision-drivers)
- [Considered Options](#considered-options)
- [Decision Outcome](#decision-outcome)
  - [Consequences](#consequences)
  - [Confirmation](#confirmation)
- [Pros and Cons of the Options](#pros-and-cons-of-the-options)
  - [Hub-and-Spoke](#hub-and-spoke)
  - [Peer-to-Peer Mesh](#peer-to-peer-mesh)
  - [Per-Cluster Operator with Federation](#per-cluster-operator-with-federation)
- [More Information](#more-information)
- [Traceability](#traceability)

<!-- /toc -->

## Context and Problem Statement

Katapult must coordinate volume transfers ranging from 200 GiB to 15 TB across 10+ globally distributed Kubernetes clusters. The architecture must support both intra-cluster (node-to-node) and cross-cluster transfers, manage a fleet of per-node agents that execute data movement, and provide centralized observability and control.

Clusters operate behind restrictive firewall policies where worker nodes cannot accept inbound connections from external systems. The architecture must function with outbound-only connectivity from agents to a coordination endpoint, while enabling support engineers to manage transfers across all clusters through a single interface without SSH access.

## Decision Drivers

* Agents run behind firewalls with no inbound connectivity; the architecture must work with outbound-only connections from agents
* Centralized audit trail and transfer history required across all 10 clusters for compliance and troubleshooting
* Support engineers need a single pane of glass to manage transfers without SSH access to individual clusters
* Active transfers must survive control plane outages once the data path is established (agent autonomy)
* System must scale from current 10 clusters without per-cluster configuration overhead

## Considered Options

* Hub-and-spoke — Centralized control plane coordinating distributed per-node agents
* Peer-to-peer mesh — Agents discover and communicate directly with each other
* Per-cluster operator with federation — Independent Kubernetes operator per cluster with cross-cluster federation layer

## Decision Outcome

Chosen option: "Hub-and-spoke", because it naturally supports outbound-only agent connectivity (agents initiate gRPC connections to the control plane), provides a single point for audit and observability across all clusters, enables support engineer self-service through a unified Web UI and API, and aligns with the agent autonomy principle where agents execute transfers independently once commanded.

### Consequences

* Good, because a single control plane provides unified API, Web UI, CLI, and audit trail across all clusters without per-cluster management
* Good, because outbound-only connectivity from agents eliminates firewall changes in all participating clusters
* Good, because agents operate autonomously after command dispatch — active transfers survive control plane downtime
* Good, because credential management is centralized (one Credential Manager issuing scoped, ephemeral credentials per transfer)
* Bad, because the single control plane is a potential availability bottleneck for management operations (mitigated: v1 runs a single replica; HA with leader election deferred to post-v1)
* Bad, because all agent state flows through one point — control plane must handle connection fan-in from all agents across all clusters (mitigated: gRPC bidirectional streaming supports efficient multiplexing)

### Confirmation

Confirmed when:

- Agents across multiple clusters successfully register with the control plane via outbound gRPC connections without any firewall changes
- A transfer initiated from the Web UI completes end-to-end across two clusters
- An active transfer continues to completion after the control plane is restarted mid-transfer

## Pros and Cons of the Options

### Hub-and-Spoke

Centralized control plane coordinates all agents. Agents run as DaemonSets on worker nodes and initiate outbound gRPC connections to the control plane. The control plane manages transfer lifecycle, agent registry, credential issuance, and provides the API/UI surface.

* Good, because outbound-only agent connectivity fits restrictive firewall environments without configuration changes
* Good, because centralized observability provides a single audit trail, progress dashboard, and transfer history across all clusters
* Good, because a single deployment for the control plane simplifies operations — one service to monitor, upgrade, and configure
* Good, because credential management is straightforward — one service issues and revokes per-transfer credentials
* Neutral, because Kubernetes-native integration (CRD controller) works naturally alongside the centralized API
* Bad, because the control plane is a single point of failure for management operations (not for active data transfers per agent autonomy principle)
* Bad, because connection fan-in scales linearly with total agent count — hundreds of persistent gRPC connections on one endpoint

### Peer-to-Peer Mesh

Agents discover each other through a shared discovery mechanism and coordinate transfers directly without a central coordinator.

* Good, because there is no single point of failure for coordination — any agent can initiate and manage transfers
* Good, because it scales horizontally without a central bottleneck — each agent communicates only with relevant peers
* Bad, because agents must accept inbound connections from other agents, which violates the firewall constraint of no inbound connectivity
* Bad, because distributed consensus is required for transfer coordination and state management — significantly higher implementation complexity
* Bad, because there is no centralized audit trail or real-time progress view without building additional aggregation infrastructure
* Bad, because credential distribution across a mesh without a central authority introduces security complexity

### Per-Cluster Operator with Federation

Each cluster runs an independent Kubernetes operator that manages local transfers. A federation layer coordinates cross-cluster transfers by orchestrating between operators.

* Good, because each cluster operates independently — high fault isolation, one cluster's issues do not affect others
* Good, because the per-cluster operator pattern is familiar in the Kubernetes ecosystem (CRDs, controllers, reconciliation)
* Bad, because cross-cluster transfers require a separate federation layer that must coordinate between independent operators — significant additional complexity
* Bad, because there is no unified view across clusters without building a meta-API that aggregates state from all operators
* Bad, because credential and S3 coordination across independent operators requires a shared credential service or complex inter-operator trust
* Bad, because operational overhead multiplies — N clusters means N operator deployments to monitor, upgrade, and troubleshoot

## More Information

- The hub-and-spoke pattern is well-established for multi-cluster Kubernetes management (similar to Rancher, Loft, Argo CD hub model)
- gRPC bidirectional streaming enables efficient long-lived connections with low overhead per agent
- Agent autonomy principle ensures the hub is not in the data path — it coordinates but does not relay transfer data

## Traceability

- **PRD**: [PRD.md](../PRD.md)
- **DESIGN**: [DESIGN.md](../DESIGN.md)

This decision directly addresses the following requirements or design elements:

* `cpt-katapult-fr-agent-registration` — Agents initiate outbound gRPC connection to the hub; hub maintains agent registry
* `cpt-katapult-fr-transfer-autonomy` — Hub dispatches commands but is never in the data path; agents execute independently
* `cpt-katapult-nfr-cp-availability` — Hub-and-spoke separates coordination from execution; active transfers survive hub downtime
* `cpt-katapult-principle-agent-autonomy` — Hub-and-spoke enables autonomous agents that operate independently after receiving commands
* `cpt-katapult-principle-outbound-only` — Hub model naturally supports outbound-only connectivity from agents to central endpoint
