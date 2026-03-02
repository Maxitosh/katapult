---
status: proposed
date: 2026-03-02
decision-makers: Infrastructure team
---

# ADR-0004: Use gRPC Bidirectional Streaming for Agent-to-Control-Plane Communication

**ID**: `cpt-katapult-adr-grpc-agent-communication`

<!-- toc -->

- [Context and Problem Statement](#context-and-problem-statement)
- [Decision Drivers](#decision-drivers)
- [Considered Options](#considered-options)
- [Decision Outcome](#decision-outcome)
  - [Consequences](#consequences)
  - [Confirmation](#confirmation)
- [Pros and Cons of the Options](#pros-and-cons-of-the-options)
  - [gRPC Bidirectional Streaming](#grpc-bidirectional-streaming)
  - [REST with Polling](#rest-with-polling)
  - [WebSockets](#websockets)
  - [Message Queue (NATS/RabbitMQ)](#message-queue-natsrabbitmq)
- [More Information](#more-information)
- [Traceability](#traceability)

<!-- /toc -->

## Context and Problem Statement

Katapult's hub-and-spoke architecture requires a communication protocol between distributed agents (DaemonSet pods on every worker node across 10+ clusters) and the centralized control plane. The protocol must support three concurrent communication patterns: agents registering and sending heartbeats to the control plane, the control plane dispatching transfer commands to specific agents, and agents streaming real-time progress updates back to the control plane during transfers.

Agents operate behind firewalls and can only initiate outbound connections — the control plane cannot push to agents without an agent-initiated channel. Progress updates must reach the UI within 5 seconds of the agent reporting them. The protocol must handle hundreds of persistent agent connections efficiently and support multiple concurrent transfers per agent on a single connection.

## Decision Drivers

* Agents must initiate all connections outbound — the control plane cannot open connections into clusters behind firewalls
* The control plane must push commands to agents without agents polling — command dispatch must be immediate, not delayed by polling intervals
* Agents must stream real-time progress (bytes transferred, speed, chunks completed) with ≤5 second end-to-end latency to the UI
* The protocol must support long-lived connections — agents maintain persistent connections for the lifetime of the DaemonSet pod
* Strong typing and code generation from a single interface definition reduce integration bugs between independently deployed control plane and agents
* Connection multiplexing is required — multiple concurrent transfers per agent over a single connection

## Considered Options

* gRPC bidirectional streaming — Long-lived HTTP/2 streams with protobuf serialization and code generation
* REST with polling — Agents periodically poll for commands and POST progress updates
* WebSockets — Persistent bidirectional connections over HTTP/1.1 upgrade
* Message queue (NATS/RabbitMQ) — Decoupled pub/sub communication via an intermediary broker

## Decision Outcome

Chosen option: "gRPC bidirectional streaming", because it provides agent-initiated long-lived HTTP/2 connections that satisfy the outbound-only connectivity requirement, supports server-streaming for immediate command dispatch to agents (StreamCommands RPC) and client-streaming for real-time progress reporting (ReportProgress RPC), generates strongly-typed Go client/server code from protobuf definitions reducing integration errors between independently versioned control plane and agents, and multiplexes multiple RPCs over a single TCP connection per agent. gRPC-Gateway additionally generates a REST API from the same protobuf definitions, enabling external clients (Web UI, CLI) to use REST while agents use native gRPC.

### Consequences

* Good, because agent-initiated gRPC connections satisfy outbound-only connectivity — agents open the HTTP/2 channel, the control plane pushes commands through the server-streaming RPC without requiring inbound firewall rules
* Good, because server-streaming RPC (StreamCommands) enables immediate command dispatch to agents — no polling delay, commands arrive within milliseconds of dispatch
* Good, because client-streaming RPC (ReportProgress) enables continuous progress updates with low overhead, meeting the ≤5 second end-to-end latency requirement
* Good, because protobuf definitions generate type-safe Go client/server code, catching interface mismatches at compile time rather than at runtime
* Good, because HTTP/2 multiplexes multiple concurrent RPCs (Register, Heartbeat, StreamCommands, ReportProgress) over a single TCP connection per agent — efficient connection management
* Good, because gRPC-Gateway generates a parallel REST API from the same protobuf definitions — Web UI and CLI consume REST, agents consume native gRPC, one source of truth
* Bad, because gRPC requires HTTP/2, which complicates debugging with standard HTTP tools (curl, browser dev tools) — mitigated by gRPC-Gateway providing a debuggable REST API for the same operations
* Bad, because long-lived gRPC streams require reconnection handling — agents must detect disconnects and re-establish streams with exponential backoff

### Confirmation

Confirmed when:

- An agent behind a firewall successfully registers with the control plane via outbound gRPC connection with no inbound ports opened
- Transfer commands dispatched from the control plane reach the agent within 1 second via the StreamCommands server-streaming RPC
- Progress updates streamed from an agent via ReportProgress reach the Web UI within 5 seconds end-to-end
- An agent automatically reconnects and re-registers after a simulated control plane restart

## Pros and Cons of the Options

### gRPC Bidirectional Streaming

Long-lived HTTP/2 connections between agents and the control plane. Agents initiate the connection. The control plane uses server-streaming RPCs (StreamCommands) to push transfer commands and client-streaming RPCs (ReportProgress) to receive continuous progress updates. Protobuf definitions generate type-safe Go code for both sides. gRPC-Gateway generates a REST API from the same definitions for external clients.

* Good, because agent-initiated HTTP/2 connections enable server-push without inbound firewall rules — the connection is established outbound, the control plane pushes through it
* Good, because server-streaming and client-streaming RPCs map directly to the command dispatch and progress reporting patterns
* Good, because protobuf code generation produces type-safe Go clients and servers from a single `.proto` definition — contract changes are caught at compile time
* Good, because HTTP/2 multiplexes multiple RPCs over one TCP connection — efficient for per-agent overhead at hundreds of agents
* Good, because gRPC-Gateway generates a REST API from the same protobuf definitions — external clients use REST, agents use native gRPC, no duplicate API maintenance
* Good, because the Go gRPC library is the reference implementation maintained by the gRPC team — best-in-class Go support
* Neutral, because gRPC interceptors provide a natural extension point for authentication (ServiceAccount JWT validation) and logging middleware
* Bad, because HTTP/2 requirement complicates debugging — standard HTTP tools cannot inspect gRPC traffic directly without specialized tooling (grpcurl, grpc-web)
* Bad, because long-lived streams require explicit reconnection logic with backoff on network interruptions or control plane restarts

### REST with Polling

Agents periodically issue HTTP GET requests to check for pending commands and POST requests to report progress updates. Standard HTTP/1.1 request/response pattern.

* Good, because the implementation is straightforward — standard HTTP handlers, no streaming or connection management complexity
* Good, because every tool supports REST — curl, browser dev tools, HTTP proxies, load balancers all work out of the box
* Good, because the protocol is stateless — no connection state to manage, no reconnection logic needed
* Bad, because polling introduces latency proportional to the poll interval — a 5-second interval means up to 5 seconds of command dispatch delay before the agent sees a new command
* Bad, because polling at scale generates significant wasteful traffic — hundreds of agents polling every few seconds produces thousands of empty responses per minute
* Bad, because there is no server-push capability — the control plane cannot notify agents of urgent commands (e.g., cancellation) between poll cycles
* Bad, because progress updates arrive in discrete batches per poll cycle, not as continuous streams — the UI sees staircase progress updates rather than smooth real-time updates

### WebSockets

Persistent bidirectional connections between agents and the control plane over HTTP/1.1 upgrade. Messages framed in JSON or a custom binary format.

* Good, because persistent bidirectional channels support both server-push (commands) and client-push (progress) on the same connection
* Good, because the HTTP/1.1 upgrade path works through most HTTP proxies and load balancers without special configuration
* Bad, because there is no built-in code generation or type system — message framing, serialization, and deserialization must be implemented manually or with additional libraries, increasing integration error surface
* Bad, because WebSockets provide no native multiplexing — concurrent message types (commands, progress, heartbeats) require application-level framing and routing
* Bad, because the Go WebSocket ecosystem (gorilla/websocket, nhooyr/websocket) is less mature for server-to-server communication than the gRPC ecosystem
* Bad, because there is no gRPC-Gateway equivalent — a separate REST API must be built and maintained independently for external clients

### Message Queue (NATS/RabbitMQ)

Agents and the control plane communicate through an intermediary message broker. Agents subscribe to command topics, control plane publishes commands. Agents publish progress to a progress topic, control plane subscribes.

* Good, because communication is fully decoupled — agents and control plane connect independently to the broker, enabling temporal decoupling
* Good, because the broker provides built-in fan-out and routing patterns for multi-agent command distribution
* Bad, because adding a message broker introduces a stateful infrastructure dependency — the broker must be deployed, monitored, scaled, and maintained alongside the control plane
* Bad, because agent-to-broker connections still require outbound connectivity to a separate endpoint — no simplification over direct gRPC, but now two services to reach instead of one
* Bad, because message ordering and exactly-once delivery guarantees for transfer command sequencing add configuration and operational complexity
* Bad, because this contradicts the architectural goal of minimal infrastructure — Katapult's control plane is a single Deployment, adding a stateful broker significantly increases operational surface

## More Information

- The AgentService protobuf definition includes four RPCs: Register (unary), Heartbeat (unary), StreamCommands (server-streaming), and ReportProgress (client-streaming)
- gRPC-Gateway is maintained as part of the gRPC ecosystem, used by etcd, Envoy control plane, and many CNCF projects
- Agent reconnection follows standard gRPC patterns: detect stream closure, exponential backoff (1s → 2s → 4s → ... → 60s max), re-register on reconnect
- The same protobuf definitions serve as the API contract documentation — no separate OpenAPI spec maintenance for the agent-facing API

## Traceability

- **PRD**: [PRD.md](../PRD.md)
- **DESIGN**: [DESIGN.md](../DESIGN.md)

This decision directly addresses the following requirements or design elements:

* `cpt-katapult-fr-agent-registration` — Agents register via gRPC Register RPC with cluster identity and capabilities
* `cpt-katapult-fr-realtime-progress` — Agents stream progress via gRPC ReportProgress client-streaming RPC; API Server exposes SSE/WebSocket to UI/CLI
* `cpt-katapult-nfr-progress-latency` — gRPC streaming from agent to control plane enables ≤5 second end-to-end progress latency
* `cpt-katapult-principle-outbound-only` — Agent-initiated gRPC connections satisfy outbound-only connectivity without inbound firewall rules
* `cpt-katapult-principle-api-first` — gRPC-Gateway generates REST API from the same protobuf definitions, ensuring API-first design for all clients
