---
status: accepted
date: 2026-03-11
decision-makers: Infrastructure team
---

# ADR-0005: Use Database-Authoritative Projection for Transfer State Management

**ID**: `cpt-katapult-adr-database-authoritative-projection`

<!-- toc -->

- [Context and Problem Statement](#context-and-problem-statement)
- [Decision Drivers](#decision-drivers)
- [Considered Options](#considered-options)
- [Decision Outcome](#decision-outcome)
  - [Consequences](#consequences)
  - [Confirmation](#confirmation)
- [Pros and Cons of the Options](#pros-and-cons-of-the-options)
  - [CRD-First (etcd as sole source of truth)](#crd-first-etcd-as-sole-source-of-truth)
  - [DB-First (drop VolumeTransfer CRD)](#db-first-drop-volumetransfer-crd)
  - [Database-Authoritative Projection (hybrid)](#database-authoritative-projection-hybrid)
- [More Information](#more-information)
  - [Production case studies](#production-case-studies)
  - [Planned refinements](#planned-refinements)
- [Traceability](#traceability)

<!-- /toc -->

## Context and Problem Statement

Katapult manages high-frequency execution state — transfer lifecycle, progress updates every few seconds per active transfer, and append-only audit events — that must be queryable via SQL (Web UI filtering, sorting, pagination, historical analysis) and visible via Kubernetes-native tooling (kubectl, GitOps, admission webhooks). Where should the authoritative state live, and how should other interfaces access it?

Research evaluated three architectural patterns against production case studies from Argo Workflows, Tekton, Velero, Open Cluster Management (OCM), Crossplane, and Karmada. Research evaluated three patterns against production case studies.

## Decision Drivers

* Web UI and REST API require SQL-style queries (filtering by status, cluster, date range; pagination; aggregation for metrics) that etcd cannot serve — etcd has no server-side sorting, no aggregate queries, no JOINs, and pagination via expiring continue tokens only
* Transfer progress updates are high-frequency (every few seconds per active transfer), causing etcd performance degradation at scale — Argo Workflows hit etcd limits at ~500 concurrent workflows, Crossplane experienced API server CPU saturation at 2,000+ CRDs
* kubectl visibility is a real requirement for operator workflows and GitOps integration (PRD `cpt-katapult-fr-crd`)
* Transfer history and audit events must persist beyond cluster lifecycle and support compliance queries — Kubernetes Events have a 1h default TTL, etcd has an 8 GB maximum
* Kubernetes admission webhooks and RBAC provide validation and access control for CRD-based transfer creation

## Considered Options

* CRD-First — etcd as sole source of truth, drop PostgreSQL
* DB-First — PostgreSQL as sole source of truth, drop VolumeTransfer CRD
* Database-Authoritative Projection — PostgreSQL as source of truth, VolumeTransfer CRD status as read-only projection

## Decision Outcome

Chosen option: "Database-Authoritative Projection", because it preserves SQL query power for the Web UI and historical analysis while maintaining Kubernetes-native integration for operator workflows. Production evidence validates this hybrid approach: Argo Workflows and Tekton both started CRD-first and were forced to add PostgreSQL for historical queries and audit logging. Velero uses the same "external store authoritative, CRD as projection" pattern. OCM's ManifestWork pattern confirms this is how multi-cluster platforms handle state distribution.

The pattern: PostgreSQL owns all transfer state, agent registry, and audit events. The CRD Controller reads from the Transfer Orchestrator (backed by PostgreSQL) and writes to the VolumeTransfer CRD status subresource as a one-way sync. Both REST API and CRD controller delegate to the same Transfer Orchestrator for writes.

### Consequences

* Good, because all transfer state queries use PostgreSQL indexes and pagination, supporting the Web UI's filtering, sorting, and analytics needs
* Good, because VolumeTransfer CRDs provide kubectl visibility, GitOps declarative management, and Kubernetes RBAC/admission webhook integration
* Good, because transfer history and audit events persist in PostgreSQL beyond cluster lifecycle, supporting compliance and troubleshooting
* Good, because the pattern is validated by production operators (Velero, OCM, Crossplane) and avoids the etcd scaling issues experienced by Argo and Tekton
* Good, because both REST API and CRD paths converge on the same Transfer Orchestrator, ensuring consistent business logic regardless of entry point
* Bad, because two state representations (PostgreSQL + CRD status) create a synchronization surface that must be carefully managed
* Bad, because CRD status may lag behind PostgreSQL state (currently 5s polling) until event-driven sync is implemented
* Bad, because transfers created via REST API are not visible via kubectl unless a "CRD shadow" mechanism is added

### Confirmation

Confirmed when:

- Transfer created via REST API is visible in PostgreSQL immediately and reflected in VolumeTransfer CRD status within the sync interval
- Transfer created via `kubectl apply` (CRD) is persisted to PostgreSQL by the CRD Controller and subsequently reflected in CRD status
- Web UI can filter, sort, and paginate transfers using SQL queries that would be impossible with etcd alone
- `kubectl get volumetransfers` shows current phase and progress for all CRD-originated transfers
- CRD status includes observable sync metadata (lastSyncedAt, sourceRevision) confirming the projection is current

## Pros and Cons of the Options

### CRD-First (etcd as sole source of truth)

VolumeTransfer CRD becomes the sole state store. REST API creates CRDs instead of writing to PostgreSQL. Agent registry becomes a custom resource. Transfer events become Kubernetes Events. No PostgreSQL dependency.

* Good, because it eliminates PostgreSQL as an infrastructure dependency
* Good, because native `kubectl get volumetransfers` shows full state with no sync delay
* Good, because GitOps tools (ArgoCD, Flux) can manage transfer lifecycle declaratively
* Good, because reconciliation loop handles crash recovery natively
* Bad, because etcd has no server-side sorting, no aggregate queries, no JOINs — Web UI filtering and pagination become impossible or require client-side processing
* Bad, because etcd has an 8 GB maximum database size and 1 MiB per-object limit — Argo Workflows with ~200+ tasks exceed the object limit, and ~500 concurrent workflows fill etcd
* Bad, because Kubernetes Events have a 1h default TTL — not suitable for audit logging or compliance
* Bad, because multi-cluster state aggregation has no CRD-native solution — every production multi-cluster platform uses a central database or hub etcd
* Bad, because migration cost is very high: complete rewrite of API layer, Web UI backend, audit system, and agent registry

**Recommendation: Reject.** Every production operator managing high-volume execution state (Argo, Tekton) eventually added PostgreSQL for the exact requirements Katapult already satisfies.

### DB-First (drop VolumeTransfer CRD)

PostgreSQL is the only state store. Remove VolumeTransfer CRD, CRD controller, and Kubebuilder dependency entirely. REST API, CLI, and Web UI remain as-is.

* Good, because it is the simplest architecture — one state store, no synchronization, no consistency bugs
* Good, because it provides full SQL query power for all use cases
* Good, because it removes the 5s sync delay and all projection complexity
* Good, because it scales better for high transfer volumes without etcd pressure
* Bad, because no `kubectl` visibility into transfers — operators lose a primary debugging tool
* Bad, because no GitOps workflow support — cannot manage transfers via manifests
* Bad, because no reconciliation loop — crash recovery must be implemented from scratch
* Bad, because it removes Kubernetes RBAC and admission webhook integration for transfer operations

**Recommendation: Defer.** Revisit only if the CRD sync mechanism causes operational incidents. The CRD provides real value for Kubernetes-native users even though PostgreSQL handles all core requirements.

### Database-Authoritative Projection (hybrid)

PostgreSQL is the authoritative store for all transfer state, agent registry, and audit events. The VolumeTransfer CRD status is a read-only projection synced by the CRD Controller. Both REST API and CRD controller delegate to the same Transfer Orchestrator for state mutations.

* Good, because it combines SQL query power with Kubernetes-native kubectl/GitOps integration
* Good, because it matches production patterns validated at scale (Velero, OCM, Karmada)
* Good, because the architecture is already implemented — zero migration cost
* Good, because graceful degradation is possible (DB works without CRD, CRD works without DB queries)
* Neutral, because it requires a synchronization mechanism between PostgreSQL and CRD status, adding operational surface
* Bad, because CRD status can be stale during the sync interval (currently 5s polling)
* Bad, because transfers created via REST API are invisible to kubectl without a shadow CRD mechanism

**Recommendation: Adopt and refine.** This is the current architecture and it is architecturally sound. Focus on operational refinements rather than architectural migration.

## More Information

### Production case studies

- **Argo Workflows**: Started CRD-first, hit etcd limits at ~500 concurrent workflows. Added optional PostgreSQL offload (`nodeStatusOffLoad`) and Workflow Archive for history. Exactly the migration path Katapult's requirements would demand if starting CRD-first.
- **Tekton**: Created Tekton Results — a PostgreSQL-backed gRPC service — because "once the CRD object is gone from etcd, the results are gone with it." A pruner CronJob deletes completed runs from etcd after archiving.
- **Crossplane**: Pure CRD-first for configuration state. Hit severe scaling at 2,000+ CRDs: API server CPU saturation, ~2.75 GiB memory for 600 CRDs, 56-minute stabilization on GKE. Responded by reducing CRD count, not by adding a database — because configuration state (low-cardinality, small, infrequent) differs fundamentally from execution state.
- **Velero**: S3-compatible storage is authoritative. If backup exists in S3 but no CRD exists → Velero auto-creates the CRD. If CRD exists but no S3 backup → Velero deletes the CRD. External store always wins. Closest production analog to Katapult's architecture.
- **OCM (Open Cluster Management)**: Hub cluster holds ManifestWork CRDs in per-cluster namespaces. Agents pull and execute, write status back. Designed for 5,000+ managed clusters with weak hub dependency — agents continue autonomously during hub outages.

### Planned refinements

1. **Event-driven sync**: Replace 5s polling with PostgreSQL LISTEN/NOTIFY or in-process event channels. Keep 30-60s polling as a consistency safety net.
2. **Projection contract formalization**: Add `lastSyncedAt` timestamp and `sourceRevision` (database row version) to CRD status to make sync state observable.
3. **Dual-write protection**: Write "pending" state to PostgreSQL before performing remote operations, then update to "confirmed." Detect and resolve unconfirmed pending states on reconciliation (Crossplane's annotation protocol model).
4. **CRD pruning**: Completed transfers remain as CRDs for a configurable retention period (e.g., 1 hour), then are pruned from etcd while remaining queryable in PostgreSQL indefinitely (Tekton's model).
5. **ManifestWork pattern for multi-cluster**: Use dedicated per-cluster namespaces on the management cluster. Control plane writes transfer work specs; agents pull and execute, write status back (OCM/Karmada model).

## Traceability

- **PRD**: [PRD.md](../PRD.md)
- **DESIGN**: [DESIGN.md](../DESIGN.md)

This decision directly addresses the following requirements or design elements:

* `cpt-katapult-fr-crd` — VolumeTransfer CRD serves as the Kubernetes-native projection of transfer state, enabling kubectl and GitOps workflows without owning the authoritative state
* `cpt-katapult-component-crd-controller` — CRD Controller is explicitly a projection layer: reads from Transfer Orchestrator (backed by PostgreSQL), writes to CRD status as a one-way sync
* `cpt-katapult-component-transfer-orchestrator` — Transfer Orchestrator persists all state to PostgreSQL as the authoritative store; both REST API and CRD paths converge here
* `cpt-katapult-component-api-server` — API Server reads from PostgreSQL directly (single source of truth), not from CRD status
