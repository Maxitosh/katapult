---
status: proposed
date: 2026-03-01
decision-makers: Infrastructure team
---

# ADR-0002: Use Multiple Transfer Strategies with Automatic Selection

**ID**: `cpt-katapult-adr-multi-strategy-transfers`

<!-- toc -->

- [Context and Problem Statement](#context-and-problem-statement)
- [Decision Drivers](#decision-drivers)
- [Considered Options](#considered-options)
- [Decision Outcome](#decision-outcome)
  - [Consequences](#consequences)
  - [Confirmation](#confirmation)
- [Pros and Cons of the Options](#pros-and-cons-of-the-options)
  - [Multiple Strategies with Automatic Selection](#multiple-strategies-with-automatic-selection)
  - [Single Universal Strategy (S3-Staged Only)](#single-universal-strategy-s3-staged-only)
  - [Single Universal Strategy (Streaming Only)](#single-universal-strategy-streaming-only)
- [More Information](#more-information)
- [Traceability](#traceability)

<!-- /toc -->

## Context and Problem Statement

Katapult must transfer persistent volumes ranging from 200 GiB to 15 TB across two fundamentally different network topologies: intra-cluster (node-to-node on the same network) and cross-cluster (between geographically distributed clusters connected over the public internet). These topologies have radically different characteristics — intra-cluster has high-bandwidth low-latency direct connectivity, while cross-cluster has lower bandwidth, higher latency, and is prone to intermittent failures.

A single transfer mechanism cannot optimally serve both scenarios without either sacrificing throughput for intra-cluster transfers or losing resume capability for cross-cluster transfers. Additionally, not all environments have S3-compatible object storage available, requiring a fallback for cross-cluster transfers.

## Decision Drivers

* Intra-cluster 5 TB transfers must complete in under 30 minutes on 10 Gbps networks — demanding maximum throughput with no intermediate storage overhead
* Cross-cluster 5 TB transfers must complete within 12 hours with chunk-level resume on failure — failures must not require restarting from zero
* Maximum wasted work on failure must be bounded to 1 chunk (S3 mover) or 1 pipeline run (streaming mover)
* Cross-cluster transfers over the public internet are failure-prone — restarting multi-TB transfers from zero wastes hours of work and is unacceptable for production operations
* Operators must be able to override automatic strategy selection for edge cases and testing
* S3-compatible object storage may not be available in all environments — a fallback for cross-cluster transfers is required

## Considered Options

* Multiple strategies with automatic selection — Three pluggable movers with topology-based auto-selection
* Single universal strategy using S3-staged transfers only
* Single universal strategy using streaming transfers only

## Decision Outcome

Chosen option: "Multiple strategies with automatic selection", because different network topologies have fundamentally different optimal transfer mechanisms. Intra-cluster transfers benefit from zero-overhead direct streaming (tar+zstd piped via stunnel), achieving maximum throughput by eliminating intermediate storage. Cross-cluster transfers require the chunk-level resume capability of S3-staged transfers, bounding failure cost to one chunk. A direct mover (rsync/tar via stunnel) provides a necessary fallback when S3 is unavailable. The strategy engine auto-selects based on source and destination topology, reducing operator burden while preserving manual override for edge cases.

### Consequences

* Good, because intra-cluster transfers achieve maximum throughput — direct node-to-node pipe with no intermediate storage overhead, meeting the <30 minute target for 5 TB on 10 Gbps
* Good, because cross-cluster transfers get chunk-level resume — failures waste at most one chunk (default 4 GiB) of work instead of restarting from zero
* Good, because S3 unavailability does not block cross-cluster transfers — the direct mover provides a functional fallback (without chunk-level resume)
* Good, because automatic selection reduces operator burden — the strategy engine selects the optimal mover without manual intervention in the common case
* Bad, because three mover implementations increase code complexity and testing surface — each mover has distinct data path, error handling, and progress reporting logic
* Bad, because the strategy engine adds decision logic that must be validated for correctness across all topology combinations

### Confirmation

Confirmed when:

- An intra-cluster 5 TB transfer completes in under 30 minutes on a 10 Gbps network using the streaming mover
- A cross-cluster transfer resumes from the last completed chunk after a simulated failure at 50% using the S3 mover
- A cross-cluster transfer completes using the direct mover when S3 is not configured
- The strategy engine correctly selects streaming for intra-cluster and S3 for cross-cluster without manual input

## Pros and Cons of the Options

### Multiple Strategies with Automatic Selection

Three pluggable mover implementations within the Agent Runtime: a stream mover (tar+zstd piped via stunnel) for intra-cluster transfers, an S3 mover (chunked upload/download via S3 API) for cross-cluster transfers, and a direct mover (rsync/tar via stunnel) as a cross-cluster fallback. The Transfer Orchestrator's strategy engine evaluates source and destination topology and selects the optimal mover automatically. Operators can override via an API parameter.

* Good, because each topology gets an optimized transfer mechanism tailored to its network characteristics
* Good, because chunk-level resume for cross-cluster eliminates multi-hour restart penalties on failure
* Good, because a fallback exists for environments without S3-compatible object storage
* Good, because operator override preserves flexibility for edge cases and testing scenarios
* Neutral, because all three movers share the Agent Runtime infrastructure (gRPC progress reporting, staging directory, cleanup)
* Bad, because three mover implementations must be built, tested, and maintained independently
* Bad, because strategy selection logic adds a decision layer that must handle topology edge cases correctly

### Single Universal Strategy (S3-Staged Only)

All transfers — both intra-cluster and cross-cluster — go through S3-compatible object storage. Source agent uploads chunks to S3, destination agent downloads chunks from S3.

* Good, because one implementation to build and maintain — simplest codebase and lowest development cost
* Good, because resume capability is available for all transfers, including intra-cluster
* Bad, because intra-cluster transfers pay unnecessary S3 overhead — data travels source → S3 → destination instead of source → destination, doubling network traversal
* Bad, because intra-cluster throughput is severely degraded — S3 round-trip adds latency and reduces effective bandwidth, likely failing the <30 minute target for 5 TB
* Bad, because S3 becomes a hard requirement for all transfers — blocks deployments without S3 access even for intra-cluster use cases
* Bad, because S3 costs increase linearly with transfer volume — unnecessary expense for intra-cluster transfers that could use free node-to-node bandwidth

### Single Universal Strategy (Streaming Only)

All transfers — both intra-cluster and cross-cluster — use direct node-to-node streaming (tar+zstd piped via stunnel).

* Good, because simplest data path — direct pipe between source and destination agents
* Good, because maximum throughput for intra-cluster transfers with no intermediate storage
* Bad, because no resume capability — any failure requires restarting the entire transfer from the beginning
* Bad, because cross-cluster transfers of multi-TB volumes over unreliable internet links fail frequently, wasting hours of transfer time on each failure
* Bad, because this directly violates the bounded failure cost requirement and the resumable cross-cluster transfer requirement

## More Information

- The strategy engine selection logic is intentionally simple for v1: same cluster = stream, different cluster + S3 configured = S3, different cluster + no S3 = direct
- All three movers share the same agent-side infrastructure: staging directory writes, atomic move on completion, progress reporting via gRPC, and resource cleanup on terminal state
- The mover plugin interface enables adding future strategies (e.g., parallel multi-stream, network-aware routing) without modifying the orchestration layer
- Agent tool dependencies (`cpt-katapult-constraint-agent-tools`) are driven by this decision: tar and zstd for streaming/direct movers, stunnel for TLS on direct data paths

## Traceability

- **PRD**: [PRD.md](../PRD.md)
- **DESIGN**: [DESIGN.md](../DESIGN.md)

This decision directly addresses the following requirements or design elements:

* `cpt-katapult-fr-strategy-selection` — Strategy engine auto-selects mover based on source/destination topology; manual override via API parameter
* `cpt-katapult-fr-resumable-transfer` — S3 mover implements chunk-level resume; failures restart from the first missing chunk
* `cpt-katapult-nfr-throughput-intra` — Streaming mover achieves maximum throughput for intra-cluster transfers (tar+zstd via stunnel, no S3 overhead)
* `cpt-katapult-nfr-throughput-cross` — S3 mover supports chunked parallel upload/download for cross-cluster throughput
* `cpt-katapult-nfr-bounded-failure` — S3 mover bounds failure cost to one chunk; streaming mover bounds to one pipeline run
* `cpt-katapult-constraint-s3-required` — Direct mover provides cross-cluster fallback when S3 is unavailable
