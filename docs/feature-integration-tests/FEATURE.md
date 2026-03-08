# Feature: Integration Tests


<!-- toc -->

- [1. Feature Context](#1-feature-context)
  - [1.1 Overview](#11-overview)
  - [1.2 Purpose](#12-purpose)
  - [1.3 Actors](#13-actors)
  - [1.4 References](#14-references)
- [2. Actor Flows (CDSL)](#2-actor-flows-cdsl)
  - [Run Controller Integration Tests](#run-controller-integration-tests)
  - [Run Component Integration Tests](#run-component-integration-tests)
  - [Run E2E Tests](#run-e2e-tests)
- [3. Processes / Business Logic (CDSL)](#3-processes-business-logic-cdsl)
  - [Envtest Environment Setup](#envtest-environment-setup)
  - [Testcontainers Orchestration](#testcontainers-orchestration)
  - [Kind Cluster Lifecycle](#kind-cluster-lifecycle)
  - [Data Integrity Verification](#data-integrity-verification)
- [4. States (CDSL)](#4-states-cdsl)
  - [Not Applicable](#not-applicable)
- [5. Definitions of Done](#5-definitions-of-done)
  - [Controller Integration Tests](#controller-integration-tests)
  - [Component Integration Tests](#component-integration-tests)
  - [E2E Tests](#e2e-tests)
  - [Build Tag Separation](#build-tag-separation)
  - [Shared Test Helpers](#shared-test-helpers)
- [6. Acceptance Criteria](#6-acceptance-criteria)
- [7. Additional Context](#7-additional-context)
  - [Non-Applicable Checklist Domains](#non-applicable-checklist-domains)

<!-- /toc -->

- [x] `p2` - **ID**: `cpt-katapult-featstatus-integration-tests`
## 1. Feature Context

- [x] `p2` - `cpt-katapult-feature-integration-tests`

### 1.1 Overview

Three-tier test infrastructure for Katapult providing envtest-based CRD Controller tests, testcontainers-based component integration tests, and Kind-based end-to-end tests. Each tier validates a different layer of the system — from Kubernetes reconciliation loops through cross-component wire-level interactions to full transfer workflows in ephemeral clusters.

Problem: Unit tests with in-memory repositories verify business logic but miss wire-level issues (serialization, connection handling, credential propagation, CRD reconciliation). Only real infrastructure can validate the full data path.
Primary value: Reproducible, isolated test environments that catch integration defects before production without requiring shared or long-lived clusters.
Key assumptions: CI environment supports Docker (for testcontainers and Kind). Developers have Docker available locally for running integration and E2E tests.

### 1.2 Purpose

Enable developers and CI pipelines to validate Katapult across three tiers — CRD Controller reconciliation (envtest), cross-component interactions (testcontainers), and full transfer workflows (Kind) — with reproducible, ephemeral test environments that complement existing unit tests.

Success criteria: All three tiers pass in CI, controller tests run without a real cluster, component tests validate gRPC/REST/S3 wire paths, and E2E tests complete real PVC transfers with data integrity verification.

**Covers (PRD)**:
- `cpt-katapult-fr-controller-integration-tests`
- `cpt-katapult-fr-component-integration-tests`
- `cpt-katapult-fr-e2e-tests`
- `cpt-katapult-nfr-test-execution-time`

**Covers (DESIGN)**:
- `cpt-katapult-component-test-harness`

### 1.3 Actors

| Actor | Role in Feature |
|-------|-----------------|
| `cpt-katapult-actor-infra-engineer` | Writes test cases, configures CI pipelines, runs test tiers locally and in CI |
| `cpt-katapult-actor-control-plane` | Validated target — controller reconciliation, API server, transfer orchestrator are exercised by tests |
| `cpt-katapult-actor-agent` | Validated target — agent registration, heartbeat, and transfer execution are exercised by E2E tests |

### 1.4 References

- **PRD**: [PRD.md](../PRD.md)
- **Design**: [DESIGN.md](../DESIGN.md)
- **Dependencies**: `cpt-katapult-feature-agent-system`, `cpt-katapult-feature-transfer-engine`, `cpt-katapult-feature-api-cli`

## 2. Actor Flows (CDSL)

### Run Controller Integration Tests

- [x] `p2` - **ID**: `cpt-katapult-flow-integration-tests-run-controller-tests`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- All VolumeTransfer CRD reconciliation tests pass; status subresource updates are verified; finalizer behavior is correct

**Error Scenarios**:
- Envtest fails to start (missing envtest binaries) — test framework reports setup error with remediation hint
- CRD installation fails (malformed YAML) — test reports schema validation error

**Steps**:
1. [ ] - `p2` - Developer invokes `go test -tags integration ./internal/crd/...` - `inst-invoke-controller-tests`
2. [ ] - `p2` - Algorithm: initialize envtest environment using `cpt-katapult-algo-integration-tests-envtest-setup` - `inst-setup-envtest`
3. [ ] - `p2` - Test creates a VolumeTransfer CR with source and destination PVC references - `inst-create-vt-cr`
4. [ ] - `p2` - Reconciler processes the CR and updates status subresource (phase, conditions, progress) - `inst-reconcile-cr`
5. [ ] - `p2` - Test asserts status fields match expected values for each reconciliation cycle - `inst-assert-status`
6. [ ] - `p2` - Test deletes VolumeTransfer CR and verifies finalizer runs cleanup logic - `inst-verify-finalizer`
7. [ ] - `p2` - **IF** any assertion fails **RETURN** test failure with diff of expected vs actual status - `inst-check-assertion`
8. [ ] - `p2` - Algorithm: tear down envtest environment (stop API server, etcd) - `inst-teardown-envtest`
9. [ ] - `p2` - **RETURN** test results (pass/fail count, duration) - `inst-return-controller-results`

### Run Component Integration Tests

- [x] `p2` - **ID**: `cpt-katapult-flow-integration-tests-run-component-tests`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- gRPC agent registration and heartbeat flows succeed against real server; API-to-orchestrator transfer lifecycle completes; S3-staged path uploads/downloads via MinIO

**Error Scenarios**:
- Testcontainers fail to start (Docker unavailable) — test reports container startup error
- Migration fails (schema drift) — test reports migration error with failing SQL statement
- gRPC connection refused — test reports connection error with port/address

**Steps**:
1. [ ] - `p2` - Developer invokes `go test -tags integration ./...` - `inst-invoke-component-tests`
2. [ ] - `p2` - Algorithm: start test containers using `cpt-katapult-algo-integration-tests-testcontainers-setup` - `inst-setup-containers`
3. [ ] - `p2` - Test starts gRPC server with real PostgreSQL repository and MinIO credential manager - `inst-start-grpc-server`
4. [ ] - `p2` - Test exercises agent registration flow: agent client connects, sends Register RPC, verifies agent appears in database - `inst-test-registration`
5. [ ] - `p2` - Test exercises heartbeat flow: agent sends Heartbeat RPC, verifies last_seen timestamp updates - `inst-test-heartbeat`
6. [ ] - `p2` - Test exercises transfer lifecycle: API creates transfer, orchestrator transitions through states, progress is reported - `inst-test-transfer-lifecycle`
7. [ ] - `p2` - Test exercises S3-staged path: upload chunks to MinIO, download and verify content matches - `inst-test-s3-path`
8. [ ] - `p2` - **IF** any assertion fails **RETURN** test failure with component and operation context - `inst-check-component-assertion`
9. [ ] - `p2` - Algorithm: stop and remove all test containers - `inst-teardown-containers`
10. [ ] - `p2` - **RETURN** test results (pass/fail count, duration) - `inst-return-component-results`

### Run E2E Tests

- [x] `p2` - **ID**: `cpt-katapult-flow-integration-tests-run-e2e-tests`

**Actor**: `cpt-katapult-actor-infra-engineer`

**Success Scenarios**:
- Full Katapult stack deploys in Kind; intra-cluster streaming transfer completes with matching checksums; cross-cluster S3-staged transfer completes; cancellation cleans up resources; CLI commands execute successfully

**Error Scenarios**:
- Kind cluster creation fails (insufficient resources) — test reports cluster creation error
- Katapult deployment fails (image pull error) — test reports pod status with events
- Transfer times out — test reports transfer state and last progress update

**Steps**:
1. [ ] - `p2` - Developer invokes `go test -tags e2e ./test/e2e/...` - `inst-invoke-e2e-tests`
2. [ ] - `p2` - Algorithm: create Kind cluster and deploy Katapult using `cpt-katapult-algo-integration-tests-kind-lifecycle` - `inst-setup-kind`
3. [ ] - `p2` - Test creates source PVC and populates it with known test data (deterministic content for checksum) - `inst-create-source-pvc`
4. [ ] - `p2` - Test creates destination PVC (empty) - `inst-create-dest-pvc`
5. [ ] - `p2` - Test initiates intra-cluster streaming transfer via REST API - `inst-initiate-intra-transfer`
6. [ ] - `p2` - Test polls transfer status until completion or timeout - `inst-poll-transfer-status`
7. [ ] - `p2` - Algorithm: verify data integrity using `cpt-katapult-algo-integration-tests-data-integrity-check` - `inst-verify-intra-integrity`
8. [ ] - `p2` - Test initiates cross-cluster S3-staged transfer via MinIO sidecar - `inst-initiate-cross-transfer`
9. [ ] - `p2` - Algorithm: verify data integrity using `cpt-katapult-algo-integration-tests-data-integrity-check` - `inst-verify-cross-integrity`
10. [ ] - `p2` - Test initiates a transfer and immediately cancels it, verifies destination PVC is cleaned up and transfer state is CANCELLED - `inst-test-cancellation`
11. [ ] - `p2` - Test executes CLI commands (`katapult transfer list`, `katapult agent list`) against live cluster and verifies output - `inst-test-cli`
12. [ ] - `p2` - **IF** any assertion fails **RETURN** test failure with cluster state snapshot (pod logs, events) - `inst-check-e2e-assertion`
13. [ ] - `p2` - Algorithm: delete Kind cluster and clean up resources using `cpt-katapult-algo-integration-tests-kind-lifecycle` - `inst-teardown-kind`
14. [ ] - `p2` - **RETURN** test results (pass/fail count, duration, cluster resource usage) - `inst-return-e2e-results`

## 3. Processes / Business Logic (CDSL)

### Envtest Environment Setup

- [x] `p2` - **ID**: `cpt-katapult-algo-integration-tests-envtest-setup`

**Input**: CRD YAML directory path, reconciler constructor function

**Output**: Running envtest environment with configured controller manager, or error if setup fails

**Steps**:
1. [ ] - `p2` - Resolve envtest binary assets path from `KUBEBUILDER_ASSETS` environment variable - `inst-resolve-envtest-assets`
2. [ ] - `p2` - **IF** envtest assets not found **RETURN** error with download instructions (`setup-envtest use`) - `inst-check-envtest-assets`
3. [ ] - `p2` - Start envtest environment (local etcd + API server) with CRD install options pointing to VolumeTransfer CRD YAML - `inst-start-envtest`
4. [ ] - `p2` - Create controller manager with the envtest REST config - `inst-create-manager`
5. [ ] - `p2` - Register VolumeTransfer reconciler with the manager - `inst-register-reconciler`
6. [ ] - `p2` - Start manager in background goroutine with cancellable context - `inst-start-manager`
7. [ ] - `p2` - **RETURN** envtest environment handle, manager cancel function, and Kubernetes client - `inst-return-envtest`

### Testcontainers Orchestration

- [x] `p2` - **ID**: `cpt-katapult-algo-integration-tests-testcontainers-setup`

**Input**: Container image versions (PostgreSQL, MinIO), migration directory path, seed fixture definitions

**Output**: Running containers with connection strings (PostgreSQL DSN, MinIO endpoint + credentials), or error if startup fails

**Steps**:
1. [ ] - `p2` - Start PostgreSQL container using `testcontainers-go` with health check wait strategy - `inst-start-postgres`
2. [ ] - `p2` - Run database migrations against PostgreSQL container using migration files from `internal/store/postgres/migrations/` - `inst-run-migrations`
3. [ ] - `p2` - Start MinIO container with preconfigured bucket and access credentials - `inst-start-minio`
4. [ ] - `p2` - **IF** any container fails to start **RETURN** error with container logs - `inst-check-container-start`
5. [ ] - `p2` - Seed test fixtures into PostgreSQL (agents, transfers) using fixture builder helpers - `inst-seed-fixtures`
6. [ ] - `p2` - **RETURN** PostgreSQL DSN, MinIO endpoint, MinIO access key, MinIO secret key - `inst-return-container-config`

### Kind Cluster Lifecycle

- [x] `p2` - **ID**: `cpt-katapult-algo-integration-tests-kind-lifecycle`

**Input**: Cluster name, node count, Katapult container images (pre-built), Kubernetes manifests directory

**Output**: Running Kind cluster with deployed Katapult stack (control plane pod ready, agent DaemonSet running), or error if setup fails

**Steps**:
1. [ ] - `p2` - Create Kind cluster with specified node count and cluster name using `sigs.k8s.io/kind` Go API - `inst-create-kind-cluster`
2. [ ] - `p2` - Load pre-built Katapult container images into Kind nodes (`kind load docker-image`) - `inst-load-images`
3. [ ] - `p2` - Install local-path-provisioner for dynamic PVC provisioning - `inst-install-provisioner`
4. [ ] - `p2` - Deploy MinIO into the cluster for S3-staged transfer tests - `inst-deploy-minio`
5. [ ] - `p2` - Apply Katapult Kubernetes manifests (control plane Deployment, agent DaemonSet, CRDs, RBAC) - `inst-apply-manifests`
6. [ ] - `p2` - Wait for control plane pod to reach Ready state with timeout - `inst-wait-control-plane`
7. [ ] - `p2` - Wait for agent DaemonSet pods to reach Ready state on all nodes with timeout - `inst-wait-agents`
8. [ ] - `p2` - **IF** any pod fails to reach Ready **RETURN** error with pod status, events, and container logs - `inst-check-pod-ready`
9. [ ] - `p2` - **RETURN** kubeconfig path, cluster name, and cleanup function - `inst-return-kind-config`

### Data Integrity Verification

- [x] `p2` - **ID**: `cpt-katapult-algo-integration-tests-data-integrity-check`

**Input**: Source PVC name/namespace, destination PVC name/namespace, Kubernetes client

**Output**: Pass with matching checksums, or fail with checksum mismatch details

**Steps**:
1. [ ] - `p2` - Create a temporary pod mounting the source PVC and compute SHA-256 checksum of all files (recursive, sorted) - `inst-checksum-source`
2. [ ] - `p2` - Create a temporary pod mounting the destination PVC and compute SHA-256 checksum of all files (recursive, sorted) - `inst-checksum-dest`
3. [ ] - `p2` - **IF** source checksum equals destination checksum **RETURN** pass - `inst-compare-checksums`
4. [ ] - `p2` - **ELSE** **RETURN** fail with source checksum, destination checksum, and first differing file path - `inst-return-mismatch`

## 4. States (CDSL)

### Not Applicable

Not applicable because test infrastructure follows a setup-execute-teardown pattern without persistent entity state. Test environments are ephemeral — created before each test run and destroyed after. No entity lifecycle transitions are managed by the test harness itself; it validates state transitions in the components under test (e.g., Transfer state machine) but does not own them.

## 5. Definitions of Done

### Controller Integration Tests

- [x] `p2` - **ID**: `cpt-katapult-dod-integration-tests-controller-tests`

The system **MUST** provide envtest-based integration tests that validate VolumeTransfer CRD reconciliation without requiring a real Kubernetes cluster. Tests **MUST** verify: CR creation triggers reconciliation, status subresource updates reflect transfer state, and finalizer logic runs on CR deletion.

**Implements**:
- `cpt-katapult-flow-integration-tests-run-controller-tests`
- `cpt-katapult-algo-integration-tests-envtest-setup`

**Covers (PRD)**:
- `cpt-katapult-fr-controller-integration-tests`

**Covers (DESIGN)**:
- `cpt-katapult-component-test-harness`
- `cpt-katapult-component-crd-controller`

### Component Integration Tests

- [x] `p2` - **ID**: `cpt-katapult-dod-integration-tests-component-tests`

The system **MUST** provide testcontainers-based integration tests that validate cross-component interactions with real infrastructure. Tests **MUST** verify: gRPC agent registration and heartbeat against a real server with PostgreSQL persistence, API-to-orchestrator transfer lifecycle, and S3-staged transfer path via MinIO. Tests **MUST** use `//go:build integration` build tag.

**Implements**:
- `cpt-katapult-flow-integration-tests-run-component-tests`
- `cpt-katapult-algo-integration-tests-testcontainers-setup`

**Covers (PRD)**:
- `cpt-katapult-fr-component-integration-tests`
- `cpt-katapult-nfr-test-execution-time`

**Covers (DESIGN)**:
- `cpt-katapult-component-test-harness`
- `cpt-katapult-component-api-server`
- `cpt-katapult-component-agent-registry`
- `cpt-katapult-component-transfer-orchestrator`

### E2E Tests

- [x] `p2` - **ID**: `cpt-katapult-dod-integration-tests-e2e-tests`

The system **MUST** provide Kind-based end-to-end tests that deploy the full Katapult stack and execute real PVC transfers. Tests **MUST** verify: intra-cluster streaming transfer with data integrity validation (SHA-256 checksum comparison), cross-cluster S3-staged transfer via MinIO, transfer cancellation with resource cleanup verification, and CLI command execution against a live cluster. Tests **MUST** use `//go:build e2e` build tag.

**Implements**:
- `cpt-katapult-flow-integration-tests-run-e2e-tests`
- `cpt-katapult-algo-integration-tests-kind-lifecycle`
- `cpt-katapult-algo-integration-tests-data-integrity-check`

**Covers (PRD)**:
- `cpt-katapult-fr-e2e-tests`
- `cpt-katapult-nfr-test-execution-time`

**Covers (DESIGN)**:
- `cpt-katapult-component-test-harness`
- `cpt-katapult-component-agent-runtime`
- `cpt-katapult-component-transfer-orchestrator`

### Build Tag Separation

- [x] `p2` - **ID**: `cpt-katapult-dod-integration-tests-build-tag-separation`

The system **MUST** separate test tiers via Go build tags: unit tests run with `go test ./...` (no tags), component integration tests require `//go:build integration`, and E2E tests require `//go:build e2e`. Each tier **MUST** be independently executable without triggering other tiers.

**Implements**:
- `cpt-katapult-flow-integration-tests-run-controller-tests`
- `cpt-katapult-flow-integration-tests-run-component-tests`
- `cpt-katapult-flow-integration-tests-run-e2e-tests`

**Covers (PRD)**:
- `cpt-katapult-fr-controller-integration-tests`
- `cpt-katapult-fr-component-integration-tests`
- `cpt-katapult-fr-e2e-tests`

**Covers (DESIGN)**:
- `cpt-katapult-component-test-harness`

### Shared Test Helpers

- [x] `p2` - **ID**: `cpt-katapult-dod-integration-tests-shared-helpers`

The system **MUST** provide shared test helper packages reusable across tiers: fixture builders for domain entities (Agent, Transfer, PVCInfo), assertion utilities for comparing domain states and Kubernetes resource conditions, and container/cluster lifecycle management wrappers for testcontainers and Kind.

**Implements**:
- `cpt-katapult-algo-integration-tests-envtest-setup`
- `cpt-katapult-algo-integration-tests-testcontainers-setup`
- `cpt-katapult-algo-integration-tests-kind-lifecycle`
- `cpt-katapult-algo-integration-tests-data-integrity-check`

**Covers (PRD)**:
- `cpt-katapult-fr-controller-integration-tests`
- `cpt-katapult-fr-component-integration-tests`
- `cpt-katapult-fr-e2e-tests`

**Covers (DESIGN)**:
- `cpt-katapult-component-test-harness`

## 6. Acceptance Criteria

- [x] Controller integration tests validate VolumeTransfer CRD reconciliation, status updates, and finalizer behavior without requiring a real Kubernetes cluster
- [x] Component integration tests validate gRPC agent registration/heartbeat, API-to-orchestrator transfer lifecycle, and S3-staged transfer path via MinIO using testcontainers
- [x] E2E tests deploy full Katapult stack in Kind and complete an intra-cluster streaming transfer with SHA-256 data integrity verification
- [x] E2E tests complete a cross-cluster S3-staged transfer via MinIO in Kind
- [x] E2E tests verify transfer cancellation cleans up destination resources
- [x] CLI commands (`katapult transfer list`, `katapult agent list`) execute successfully against a live Kind cluster
- [x] Test tiers execute independently: `go test ./...` runs only unit tests, `-tags integration` adds component tests, `-tags e2e` runs E2E tests
- [x] Integration tests (tier 2) complete in under 5 minutes
- [x] E2E tests (tier 3) complete in under 15 minutes for core transfer scenarios

## 7. Additional Context

### Non-Applicable Checklist Domains

**Performance (PERF)**: Not applicable as a standalone concern because the test harness itself is not a production component with performance SLAs. Test execution time targets are covered via `cpt-katapult-nfr-test-execution-time` in the acceptance criteria.

**Security (SEC)**: Not applicable because the test harness does not handle production authentication, authorization, or sensitive data. Test fixtures use synthetic data. MinIO test credentials are ephemeral and local to test containers.

**Reliability (REL)**: Not applicable because test environments are ephemeral by design — created and destroyed per test run. There are no availability, fault tolerance, or recovery requirements for test infrastructure itself.

**Data (DATA)**: Not applicable because test data consists of transient fixtures seeded into ephemeral containers. No persistent data lifecycle, retention, or privacy concerns apply.

**Operations (OPS)**: Not applicable because the test harness runs in developer environments and CI pipelines, not in production. No observability, health checks, or rollout strategies apply to test infrastructure.

**Compliance (COMPL)**: Not applicable because no regulatory or privacy compliance requirements apply to ephemeral test environments with synthetic data.

**Usability (UX)**: Not applicable because the test harness is a developer tool invoked via `go test` commands, not a user-facing interface.
