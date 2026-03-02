---
status: proposed
date: 2026-03-02
decision-makers: Infrastructure team
---

# ADR-0003: Use Go as Implementation Language

**ID**: `cpt-katapult-adr-use-go`

<!-- toc -->

- [Context and Problem Statement](#context-and-problem-statement)
- [Decision Drivers](#decision-drivers)
- [Considered Options](#considered-options)
- [Decision Outcome](#decision-outcome)
  - [Consequences](#consequences)
  - [Confirmation](#confirmation)
- [Pros and Cons of the Options](#pros-and-cons-of-the-options)
  - [Go](#go)
  - [Rust](#rust)
  - [Python](#python)
- [More Information](#more-information)
- [Traceability](#traceability)

<!-- /toc -->

## Context and Problem Statement

Katapult requires a programming language for both the control plane (API server, transfer orchestrator, agent registry, credential manager, CRD controller) and the distributed agents (DaemonSet pods running on every worker node). The chosen language must support Kubernetes-native development — CRD controllers via Kubebuilder, Kubernetes API access via client-go — as well as high-performance gRPC bidirectional streaming for agent communication, efficient systems-level operations (process execution, file I/O, network tunneling), and compilation to static binaries for minimal container images.

The team building Katapult operates within Chainstack's infrastructure organization where Go is the primary backend language. The language choice affects hiring, code review velocity, and long-term maintainability.

## Decision Drivers

* Kubernetes ecosystem is Go-native — Kubebuilder, client-go, and controller-runtime are Go libraries with no equivalent maturity in other languages
* Agents must execute systems-level operations: tar/zstd process pipelines, stunnel tunnel management, file I/O on persistent volumes, Kubernetes API queries
* Single static binary deployment simplifies DaemonSet agent images (scratch/distroless base) and CLI distribution
* gRPC-Gateway enables serving both REST and gRPC from the same codebase without maintaining separate API servers
* Team has existing Go expertise within Chainstack's infrastructure organization — reduces ramp-up time and review friction
* Agent memory footprint should be minimal — agents run as DaemonSets on every worker node, competing for resources with blockchain workloads

## Considered Options

* Go — Static binary, native Kubernetes ecosystem, built-in concurrency, mature gRPC support
* Rust — Memory-safe systems language, growing Kubernetes ecosystem (kube-rs), steeper learning curve
* Python — Rapid development, mature Kubernetes client, interpreted runtime with higher resource overhead

## Decision Outcome

Chosen option: "Go", because the Kubernetes ecosystem is built in Go — Kubebuilder and client-go provide battle-tested CRD controller scaffolding and Kubernetes API access with no equivalent maturity in other languages. Go compiles to single static binaries suitable for minimal container images, has built-in concurrency primitives (goroutines, channels) that naturally fit managing parallel agent operations, and the team has existing Go expertise that eliminates ramp-up time. gRPC-Gateway enables serving REST and gRPC from a single codebase, and the Go gRPC library is the reference implementation maintained by the gRPC team.

### Consequences

* Good, because Kubebuilder generates CRD controller scaffolding with reconciliation loops, RBAC markers, and admission webhooks — avoiding weeks of custom controller development
* Good, because client-go provides native, well-maintained Kubernetes API access for PVC discovery, Service/Secret management, and CRD operations
* Good, because single static binary compilation simplifies agent container images (scratch/distroless base) and CLI distribution — no runtime dependencies
* Good, because built-in concurrency (goroutines, channels) naturally fits managing multiple concurrent transfers, progress streams, and agent connections without callback complexity
* Good, because the team's existing Go expertise reduces ramp-up time and enables immediate productive contribution
* Bad, because Go's error handling is verbose — every function call requires explicit error checking, increasing boilerplate compared to Rust's `?` operator or exception-based languages
* Bad, because Go's type system is less expressive than Rust's — no sum types, no pattern matching, requiring more discipline for domain modeling (transfer states, mover types)

### Confirmation

Confirmed when:

- A CRD controller built with Kubebuilder successfully reconciles VolumeTransfer custom resources in a test cluster
- An agent binary compiled as a static Go binary runs in a distroless container image on worker nodes
- gRPC-Gateway serves both REST and gRPC endpoints from a single binary with no compatibility issues

## Pros and Cons of the Options

### Go

Go is a statically typed, compiled language designed at Google for systems programming and cloud infrastructure. It is the implementation language of Kubernetes itself, Docker, Prometheus, and most CNCF projects.

* Good, because Kubebuilder and controller-runtime are Go-native — CRD controller development follows established patterns with code generation, reconciliation loops, and operator SDK support
* Good, because client-go is the official Kubernetes API client, maintained by the Kubernetes team, with complete API coverage and informer/watch support
* Good, because single static binary compilation (CGO_ENABLED=0) produces self-contained executables ideal for minimal container images
* Good, because goroutines enable lightweight concurrency — thousands of concurrent operations with minimal memory overhead, fitting the multi-agent/multi-transfer architecture
* Good, because gRPC-Gateway is a mature Go library providing REST/gRPC dual-serving from a single protobuf definition
* Good, because the AWS SDK for Go v2 provides native S3 client with multipart upload, presigned URLs, and STS support
* Neutral, because Go's garbage collector adds small, predictable latency pauses — acceptable for coordination workloads, not a concern for data-path-heavy agent operations
* Bad, because error handling is verbose — explicit `if err != nil` on every call increases boilerplate
* Bad, because the type system lacks sum types and pattern matching — transfer state machines and mover type dispatch require manual switch statements or interface patterns

### Rust

Rust is a systems programming language focused on memory safety without garbage collection, offering zero-cost abstractions and strong type guarantees.

* Good, because memory safety without garbage collection — no GC pauses, deterministic performance for agent data-path operations
* Good, because the type system (enums, pattern matching, Result type) naturally models transfer state machines and error handling with compile-time guarantees
* Good, because kube-rs provides a growing Kubernetes client with CRD derive macros and controller runtime, though less mature than client-go
* Bad, because the Kubernetes ecosystem in Rust is significantly less mature — kube-rs has fewer contributors, less documentation, and less battle-testing than client-go and Kubebuilder
* Bad, because the learning curve is steep — ownership/borrowing model requires significant ramp-up time for a team with Go expertise, slowing initial development
* Bad, because compile times are substantially longer than Go, reducing iteration speed during development
* Bad, because hiring Rust developers with Kubernetes experience is significantly harder than hiring Go developers in the infrastructure space

### Python

Python is a dynamically typed, interpreted language widely used for scripting, automation, and rapid prototyping.

* Good, because rapid development — dynamic typing and extensive standard library enable fast prototyping and iteration
* Good, because the official kubernetes-python client provides complete Kubernetes API coverage
* Good, because a large pool of developers are familiar with Python
* Bad, because the interpreted runtime has significantly higher memory footprint — each agent DaemonSet pod consumes more resources on worker nodes already running blockchain workloads
* Bad, because Python's GIL (Global Interpreter Lock) limits true parallelism — concurrent transfer operations and progress streaming require multiprocessing or async complexity
* Bad, because no static binary compilation — agent deployment requires bundling a Python runtime or using PyInstaller/Nuitka with larger, more fragile images
* Bad, because Kubebuilder has no Python equivalent — CRD controller development requires kopf or custom reconciliation loops without the same level of scaffolding and code generation
* Bad, because gRPC performance in Python is measurably lower than Go or Rust for high-throughput bidirectional streaming

## More Information

- Go is the implementation language of Kubernetes itself (kubelet, kube-apiserver, kubectl), Docker/containerd, Prometheus, and most CNCF projects — the tooling ecosystem assumes Go
- Kubebuilder operator pattern is the recommended approach for CRD controllers per Kubernetes documentation
- gRPC-Gateway is maintained as part of the gRPC ecosystem and used by major projects (Envoy control plane, etcd)
- The choice of Go for both control plane and agent means a single build toolchain, shared libraries, and consistent code style across the entire codebase

## Traceability

- **PRD**: [PRD.md](../PRD.md)
- **DESIGN**: [DESIGN.md](../DESIGN.md)

This decision directly addresses the following requirements or design elements:

* `cpt-katapult-tech-go` — Go is the technology choice for all layers: API, Domain, Infrastructure, CLI, and Agent Runtime
* `cpt-katapult-constraint-k8s-only` — Go's native Kubernetes ecosystem (Kubebuilder, client-go) directly supports the Kubernetes-only deployment constraint
* `cpt-katapult-fr-crd` — CRD Controller built with Kubebuilder (Go) reconciles VolumeTransfer custom resources
* `cpt-katapult-fr-agent-registration` — Agent gRPC client implemented in Go connects to control plane gRPC server
