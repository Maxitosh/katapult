# Feature: Local Development Environment

- [ ] `p2` - **ID**: `cpt-katapult-featstatus-local-dev-env`

<!-- toc -->

- [1. Feature Context](#1-feature-context)
  - [1.1 Overview](#11-overview)
  - [1.2 Purpose](#12-purpose)
  - [1.3 Configuration Parameters](#13-configuration-parameters)
  - [1.4 Actors](#14-actors)
  - [1.5 References](#15-references)
- [2. Actor Flows (CDSL)](#2-actor-flows-cdsl)
  - [Provision Local Environment](#provision-local-environment)
  - [Tear Down Local Environment](#tear-down-local-environment)
  - [Rebuild Component](#rebuild-component)
  - [Seed Data Provisioning](#seed-data-provisioning)
- [3. Processes / Business Logic (CDSL)](#3-processes-business-logic-cdsl)
  - [Provision Kind Cluster](#provision-kind-cluster)
  - [Deploy Supporting Services](#deploy-supporting-services)
  - [Deploy Katapult Stack](#deploy-katapult-stack)
  - [Cleanup Resources](#cleanup-resources)
- [4. States (CDSL)](#4-states-cdsl)
  - [Not Applicable](#not-applicable)
- [5. Definitions of Done](#5-definitions-of-done)
  - [Local Environment Provisioning](#local-environment-provisioning)
  - [Single-Cluster Mode](#single-cluster-mode)
  - [Multi-Cluster Mode](#multi-cluster-mode)
  - [Seed Data](#seed-data)
  - [Fast Rebuild Cycle](#fast-rebuild-cycle)
  - [Environment Teardown](#environment-teardown)
- [6. Acceptance Criteria](#6-acceptance-criteria)
- [7. Non-Applicable Domains](#7-non-applicable-domains)

<!-- /toc -->

## 1. Feature Context

- [ ] `p2` - `cpt-katapult-feature-local-dev-env`

### 1.1 Overview

Single-command local development environment that provisions the full Katapult stack — Kind clusters, PostgreSQL, MinIO, control plane, agents, and Web UI — for interactive development, testing, and demos. Developers run `make local-up` and get a fully functional environment with realistic seed data, ready for immediate transfer testing.

Problem: Setting up a multi-component distributed system locally requires manually provisioning Kind clusters, deploying databases, building and loading container images, applying manifests, and seeding data. This multi-step process discourages testing and slows feature development.
Primary value: Enables developers to run and test all Katapult capabilities locally with a single command, maintaining fast iteration cycles through incremental rebuilds.
Key assumptions: Developer machine has Docker, Kind, kubectl, and Go installed. Sufficient resources for at least one Kind cluster with multiple worker nodes.

### 1.2 Purpose

Enable developers to provision, use, and tear down a complete local Katapult environment for interactive development and demos — providing the developer tooling layer that accelerates feature development and testing.

This feature addresses:
- `cpt-katapult-fr-local-env-provision` — single-command setup of the full Katapult stack
- `cpt-katapult-fr-local-single-cluster` — Kind cluster with multiple worker nodes for intra-cluster transfers
- `cpt-katapult-fr-local-multi-cluster` — two Kind clusters with MinIO for cross-cluster transfer testing
- `cpt-katapult-fr-local-seed-data` — pre-populated PVCs, agents, and transfer history
- `cpt-katapult-fr-local-fast-rebuild` — incremental component rebuild without full reprovisioning
- `cpt-katapult-fr-local-env-teardown` — clean teardown of all local resources

NFR coverage:
- `cpt-katapult-nfr-local-env-startup` — local environment ready in <3 minutes (single-cluster mode)

### 1.3 Configuration Parameters

| Parameter | Default | Description | Referenced In |
|-----------|---------|-------------|---------------|
| MODE | single | Cluster mode: `single` (1 cluster, N workers) or `multi` (2 clusters + MinIO staging) | `cpt-katapult-flow-local-dev-env-provision` |
| WORKERS | 2 | Number of Kind worker nodes per cluster | `cpt-katapult-algo-local-dev-env-provision-kind` |
| COMPONENT | (required) | Component to rebuild: `controlplane`, `agent`, `webui` | `cpt-katapult-flow-local-dev-env-rebuild` |
| POSTGRES_PORT | 5432 | Host port for PostgreSQL | `cpt-katapult-algo-local-dev-env-deploy-services` |
| MINIO_PORT | 9000 | Host port for MinIO API | `cpt-katapult-algo-local-dev-env-deploy-services` |
| MINIO_CONSOLE_PORT | 9001 | Host port for MinIO console | `cpt-katapult-algo-local-dev-env-deploy-services` |
| WEBUI_PORT | 8080 | Host port for Web UI | `cpt-katapult-algo-local-dev-env-deploy-stack` |

### 1.4 Actors

| Actor | Role in Feature |
|-------|-----------------|
| `cpt-katapult-actor-developer` | Provisions local environment, develops and tests features, rebuilds components, tears down environment |

### 1.5 References

- **PRD**: [PRD.md](../PRD.md)
- **Design**: [DESIGN.md](../DESIGN.md)
- **Dependencies**: `cpt-katapult-feature-agent-system`, `cpt-katapult-feature-transfer-engine`, `cpt-katapult-feature-api-cli`, `cpt-katapult-feature-web-ui`

## 2. Actor Flows (CDSL)

### Provision Local Environment

- [ ] `p2` - **ID**: `cpt-katapult-flow-local-dev-env-provision`

**Actor**: `cpt-katapult-actor-developer`

**Success Scenarios**:
- Developer runs `make local-up` and gets a fully functional single-cluster environment with seed data in <3 minutes
- Developer runs `make local-up MODE=multi` and gets a two-cluster environment with MinIO staging for cross-cluster transfer testing

**Error Scenarios**:
- Docker is not running and the command fails with a clear error message indicating Docker must be started
- Kind cluster creation fails due to port conflict and the command reports the conflicting port
- A previous environment was not torn down and the command detects it, offering to clean up first

**Steps**:
1. [ ] - `p2` - Developer runs `make local-up` (default: single-cluster) or `make local-up MODE=multi` - `inst-run-local-up`
2. [ ] - `p2` - **IF** existing local environment detected (Kind cluster exists with katapult prefix), prompt developer to tear down first or abort - `inst-check-existing`
3. [ ] - `p2` - Validate prerequisites: Docker running, Kind installed, kubectl installed, Go installed - `inst-validate-prereqs`
4. [ ] - `p2` - **IF** any prerequisite missing **RETURN** error listing missing tools with install instructions - `inst-prereq-fail`
5. [ ] - `p2` - Provision Kind cluster(s) using algorithm `cpt-katapult-algo-local-dev-env-provision-kind` - `inst-provision-kind`
6. [ ] - `p2` - Deploy supporting services using algorithm `cpt-katapult-algo-local-dev-env-deploy-services` - `inst-deploy-services`
7. [ ] - `p2` - Deploy Katapult stack using algorithm `cpt-katapult-algo-local-dev-env-deploy-stack` - `inst-deploy-stack`
8. [ ] - `p2` - Provision seed data using flow `cpt-katapult-flow-local-dev-env-seed` - `inst-seed-data`
9. [ ] - `p2` - Wait for all pods to be Ready and agents to register with control plane - `inst-wait-ready`
10. [ ] - `p2` - Print environment summary: cluster name(s), API endpoint, Web UI URL, MinIO console URL (if multi), number of registered agents, number of PVCs available - `inst-print-summary`
11. [ ] - `p2` - **RETURN** environment ready - `inst-return-ready`

### Tear Down Local Environment

- [ ] `p2` - **ID**: `cpt-katapult-flow-local-dev-env-teardown`

**Actor**: `cpt-katapult-actor-developer`

**Success Scenarios**:
- Developer runs `make local-down` and all local resources are removed with no orphans

**Error Scenarios**:
- No local environment exists and the command exits cleanly with "nothing to tear down"
- Kind cluster deletion fails and the command reports the error, suggesting manual cleanup steps

**Steps**:
1. [ ] - `p2` - Developer runs `make local-down` - `inst-run-local-down`
2. [ ] - `p2` - **IF** no local environment detected, print "No local environment found" and **RETURN** - `inst-check-nothing`
3. [ ] - `p2` - Clean up resources using algorithm `cpt-katapult-algo-local-dev-env-cleanup` - `inst-cleanup`
4. [ ] - `p2` - Verify no orphaned docker containers, networks, or volumes with katapult prefix remain - `inst-verify-clean`
5. [ ] - `p2` - **IF** orphaned resources found, force-remove them and warn developer - `inst-force-clean`
6. [ ] - `p2` - Print teardown summary: resources removed - `inst-print-teardown`
7. [ ] - `p2` - **RETURN** environment torn down - `inst-return-torn-down`

### Rebuild Component

- [ ] `p2` - **ID**: `cpt-katapult-flow-local-dev-env-rebuild`

**Actor**: `cpt-katapult-actor-developer`

**Success Scenarios**:
- Developer changes control plane code, runs `make local-rebuild COMPONENT=controlplane`, and the updated control plane is running within seconds without reprovisioning clusters or other services

**Error Scenarios**:
- Go build fails and the command reports compilation errors
- No local environment is running and the command instructs developer to run `make local-up` first
- Invalid COMPONENT value and the command lists valid options

**Steps**:
1. [ ] - `p2` - Developer runs `make local-rebuild COMPONENT=<name>` - `inst-run-rebuild`
2. [ ] - `p2` - **IF** no local environment detected **RETURN** error "No local environment running. Run make local-up first." - `inst-check-env-exists`
3. [ ] - `p2` - **IF** COMPONENT not in [controlplane, agent, webui] **RETURN** error listing valid component names - `inst-validate-component`
4. [ ] - `p2` - Build Go binary for the specified component - `inst-build-binary`
5. [ ] - `p2` - Build container image with the updated binary - `inst-build-image`
6. [ ] - `p2` - Load container image into Kind cluster(s) - `inst-load-image`
7. [ ] - `p2` - Restart the Kubernetes Deployment or DaemonSet for the component to pick up the new image - `inst-restart-deployment`
8. [ ] - `p2` - Wait for rollout to complete (pods Ready) - `inst-wait-rollout`
9. [ ] - `p2` - **RETURN** component rebuilt and redeployed - `inst-return-rebuilt`

### Seed Data Provisioning

- [ ] `p2` - **ID**: `cpt-katapult-flow-local-dev-env-seed`

**Actor**: `cpt-katapult-actor-developer`

**Success Scenarios**:
- After provisioning, PVCs with sample data exist on worker nodes, agents are registered, and sample transfer history is populated

**Error Scenarios**:
- PVC creation fails due to insufficient storage and the command reports the storage limitation
- Agent registration times out and the command reports which agents failed to register

**Steps**:
1. [ ] - `p2` - Create PVCs with sample data on each worker node (varying sizes: small 100MB, medium 1GB, large 5GB) - `inst-create-pvcs`
2. [ ] - `p2` - Populate PVCs with realistic sample data using a busybox job (random files with directory structures) - `inst-populate-data`
3. [ ] - `p2` - Wait for agents to register with the control plane (poll agent list API) - `inst-wait-agents`
4. [ ] - `p2` - **IF** agents not registered within 60 seconds **RETURN** error listing unregistered nodes - `inst-agent-timeout`
5. [ ] - `p2` - Trigger a sample intra-cluster transfer and wait for completion to populate transfer history - `inst-sample-transfer`
6. [ ] - `p2` - **IF** MODE=multi, trigger a sample cross-cluster S3-staged transfer - `inst-sample-cross-transfer`
7. [ ] - `p2` - **RETURN** seed data provisioned - `inst-return-seeded`

## 3. Processes / Business Logic (CDSL)

### Provision Kind Cluster

- [ ] `p2` - **ID**: `cpt-katapult-algo-local-dev-env-provision-kind`

**Input**: Cluster mode (single or multi), number of worker nodes

**Output**: Kind cluster(s) running with kubeconfig configured

**Steps**:
1. [ ] - `p2` - Generate Kind cluster configuration: 1 control-plane node + WORKERS worker nodes, with extraPortMappings for API server and Web UI access - `inst-gen-kind-config`
2. [ ] - `p2` - **IF** MODE=single, create one Kind cluster named `katapult-dev` - `inst-create-single`
3. [ ] - `p2` - **IF** MODE=multi, create two Kind clusters named `katapult-dev-src` and `katapult-dev-dst` - `inst-create-multi`
4. [ ] - `p2` - Configure kubeconfig contexts for created cluster(s) - `inst-configure-kubeconfig`
5. [ ] - `p2` - **IF** MODE=multi, configure network connectivity between the two Kind clusters (shared Docker network) - `inst-configure-network`
6. [ ] - `p2` - **RETURN** cluster(s) ready with kubeconfig configured - `inst-return-clusters`

### Deploy Supporting Services

- [ ] `p2` - **ID**: `cpt-katapult-algo-local-dev-env-deploy-services`

**Input**: Cluster mode, port configuration

**Output**: PostgreSQL and MinIO running and accessible

**Steps**:
1. [ ] - `p2` - Generate docker-compose.yaml with PostgreSQL service (port POSTGRES_PORT) - `inst-gen-compose`
2. [ ] - `p2` - **IF** MODE=multi, add MinIO service to docker-compose.yaml (ports MINIO_PORT, MINIO_CONSOLE_PORT) - `inst-add-minio`
3. [ ] - `p2` - **IF** MODE=single, add MinIO service for optional local S3 testing - `inst-add-minio-single`
4. [ ] - `p2` - Run `docker-compose up -d` to start services - `inst-start-services`
5. [ ] - `p2` - Wait for PostgreSQL to accept connections (poll with pg_isready or TCP check) - `inst-wait-postgres`
6. [ ] - `p2` - Run database migrations (create schema, tables) against PostgreSQL - `inst-run-migrations`
7. [ ] - `p2` - **IF** MinIO started, create the S3 bucket for transfer staging using MinIO client - `inst-create-bucket`
8. [ ] - `p2` - **RETURN** supporting services ready - `inst-return-services`

### Deploy Katapult Stack

- [ ] `p2` - **ID**: `cpt-katapult-algo-local-dev-env-deploy-stack`

**Input**: Kind cluster name(s), PostgreSQL and MinIO connection details

**Output**: Control plane, agents, and Web UI deployed and running

**Steps**:
1. [ ] - `p2` - Build Go binaries for control plane and agent (`go build`) - `inst-build-binaries`
2. [ ] - `p2` - Build container images for control plane, agent, and Web UI - `inst-build-images`
3. [ ] - `p2` - Load container images into Kind cluster(s) using `kind load docker-image` - `inst-load-images`
4. [ ] - `p2` - Apply CRD manifests (VolumeTransfer CRD) to cluster(s) - `inst-apply-crds`
5. [ ] - `p2` - Apply control plane Deployment manifest (references PostgreSQL and MinIO connection details via environment variables) - `inst-deploy-controlplane`
6. [ ] - `p2` - Apply agent DaemonSet manifest to worker nodes - `inst-deploy-agents`
7. [ ] - `p2` - Apply Web UI Deployment manifest with Service exposing on WEBUI_PORT - `inst-deploy-webui`
8. [ ] - `p2` - **IF** MODE=multi, deploy agents and control plane connectivity to both clusters - `inst-multi-deploy`
9. [ ] - `p2` - Wait for all Deployments and DaemonSets to reach Ready state - `inst-wait-deployments`
10. [ ] - `p2` - **RETURN** Katapult stack deployed - `inst-return-stack`

### Cleanup Resources

- [ ] `p2` - **ID**: `cpt-katapult-algo-local-dev-env-cleanup`

**Input**: None (discovers existing resources by naming convention)

**Output**: All local environment resources removed

**Steps**:
1. [ ] - `p2` - Delete Kind cluster(s) matching katapult prefix (`kind delete cluster --name katapult-dev*`) - `inst-delete-clusters`
2. [ ] - `p2` - Stop and remove docker-compose services (`docker-compose down -v`) - `inst-stop-services`
3. [ ] - `p2` - Remove docker networks created for inter-cluster connectivity - `inst-remove-networks`
4. [ ] - `p2` - Remove docker volumes with katapult prefix - `inst-remove-volumes`
5. [ ] - `p2` - Remove generated kubeconfig contexts - `inst-remove-kubeconfig`
6. [ ] - `p2` - **RETURN** all resources cleaned up - `inst-return-cleaned`

## 4. States (CDSL)

### Not Applicable

Not applicable because the local development environment does not introduce entity lifecycle states. The environment is either provisioned or not — managed through `make local-up` and `make local-down` commands. There are no intermediate states, transitions, or guards. Transfer states (Pending, Validating, Transferring, Completed, Failed, Cancelled) are defined in the Transfer Engine feature (`cpt-katapult-feature-transfer-engine`) and exercised but not modified by this feature.

## 5. Definitions of Done

### Local Environment Provisioning

- [ ] `p2` - **ID**: `cpt-katapult-dod-local-dev-env-provision`

The system **MUST** provide a single-command setup (`make local-up`) that provisions the full Katapult stack locally: Kind cluster(s), PostgreSQL, MinIO, control plane, agent(s), and Web UI. The provisioned environment **MUST** be fully functional for interactive development, testing, and demos without requiring manual configuration steps.

**Implements**:
- `cpt-katapult-flow-local-dev-env-provision`
- `cpt-katapult-algo-local-dev-env-provision-kind`
- `cpt-katapult-algo-local-dev-env-deploy-services`
- `cpt-katapult-algo-local-dev-env-deploy-stack`

**Covers (PRD)**:
- `cpt-katapult-fr-local-env-provision`
- `cpt-katapult-nfr-local-env-startup`

**Covers (DESIGN)**:
- `cpt-katapult-component-local-dev-env`
- `cpt-katapult-component-api-server`
- `cpt-katapult-component-agent-runtime`
- `cpt-katapult-component-transfer-orchestrator`
- `cpt-katapult-component-web-ui`

**Touches**:
- CLI: `make local-up`, `make local-up MODE=multi`
- Entities: `Agent`, `Transfer`, `PVCInfo` (seed data)

### Single-Cluster Mode

- [ ] `p2` - **ID**: `cpt-katapult-dod-local-dev-env-single-cluster`

The local environment **MUST** support a single-cluster mode as the default: one Kind cluster with one control-plane node and multiple worker nodes. Agents **MUST** be deployed as a DaemonSet on worker nodes. The environment **MUST** enable real intra-cluster data movement between worker nodes to validate streaming transfer flows.

**Implements**:
- `cpt-katapult-flow-local-dev-env-provision`
- `cpt-katapult-algo-local-dev-env-provision-kind`

**Covers (PRD)**:
- `cpt-katapult-fr-local-single-cluster`

**Covers (DESIGN)**:
- `cpt-katapult-component-local-dev-env`

**Touches**:
- CLI: `make local-up` (default mode)
- Entities: `Agent` (one per worker node)

### Multi-Cluster Mode

- [ ] `p2` - **ID**: `cpt-katapult-dod-local-dev-env-multi-cluster`

The local environment **MUST** support a multi-cluster mode: two Kind clusters simulating a cross-cluster topology with MinIO as the S3-compatible staging area. The environment **MUST** enable testing the full S3-staged transfer path (upload from source cluster, MinIO staging, download to destination cluster).

**Implements**:
- `cpt-katapult-flow-local-dev-env-provision`
- `cpt-katapult-algo-local-dev-env-provision-kind`

**Covers (PRD)**:
- `cpt-katapult-fr-local-multi-cluster`

**Covers (DESIGN)**:
- `cpt-katapult-component-local-dev-env`

**Touches**:
- CLI: `make local-up MODE=multi`
- Entities: `Agent` (agents on both clusters)

### Seed Data

- [ ] `p2` - **ID**: `cpt-katapult-dod-local-dev-env-seed-data`

The local environment **MUST** pre-populate realistic seed data: PVCs with sample data on worker nodes, pre-registered agents, and sample transfer history. The seed data **MUST** provide a ready-to-use environment where a developer can immediately initiate a transfer without manual data setup.

**Implements**:
- `cpt-katapult-flow-local-dev-env-seed`

**Covers (PRD)**:
- `cpt-katapult-fr-local-seed-data`

**Covers (DESIGN)**:
- `cpt-katapult-component-local-dev-env`

**Touches**:
- API: `POST /api/v1alpha1/transfers` (sample transfer)
- DB: `transfers`, `agents`, `agent_pvcs`, `transfer_events` (seed data)
- Entities: `Transfer`, `Agent`, `PVCInfo`

### Fast Rebuild Cycle

- [ ] `p2` - **ID**: `cpt-katapult-dod-local-dev-env-fast-rebuild`

The local environment **MUST** support rebuilding and redeploying individual components (control plane, agent, Web UI) without reprovisioning the entire environment. A code change to a single component **MUST** be testable by rebuilding and redeploying only that component.

**Implements**:
- `cpt-katapult-flow-local-dev-env-rebuild`

**Covers (PRD)**:
- `cpt-katapult-fr-local-fast-rebuild`

**Covers (DESIGN)**:
- `cpt-katapult-component-local-dev-env`

**Touches**:
- CLI: `make local-rebuild COMPONENT=<name>`

### Environment Teardown

- [ ] `p2` - **ID**: `cpt-katapult-dod-local-dev-env-teardown`

The system **MUST** provide a single-command cleanup (`make local-down`) that removes all local resources: Kind clusters, docker containers, docker networks, and persistent volumes. Teardown **MUST** leave no orphaned resources.

**Implements**:
- `cpt-katapult-flow-local-dev-env-teardown`
- `cpt-katapult-algo-local-dev-env-cleanup`

**Covers (PRD)**:
- `cpt-katapult-fr-local-env-teardown`

**Covers (DESIGN)**:
- `cpt-katapult-component-local-dev-env`

**Touches**:
- CLI: `make local-down`

## 6. Acceptance Criteria

- [ ] `make local-up` provisions a single-cluster environment with control plane, agents, and Web UI running
- [ ] Single-cluster environment is transfer-ready (agents registered, seed data loaded, Web UI accessible) within 3 minutes
- [ ] `make local-up MODE=multi` provisions two Kind clusters with MinIO for cross-cluster testing
- [ ] An intra-cluster streaming transfer completes in the single-cluster environment
- [ ] A cross-cluster S3-staged transfer completes in the multi-cluster environment
- [ ] Seed data includes PVCs with sample data on worker nodes, registered agents, and transfer history
- [ ] `make local-rebuild COMPONENT=controlplane` rebuilds and redeploys only the control plane without reprovisioning clusters
- [ ] `make local-rebuild COMPONENT=agent` rebuilds and redeploys only the agent DaemonSet
- [ ] `make local-down` removes all Kind clusters, docker containers, networks, and volumes
- [ ] No orphaned docker resources (containers, networks, volumes) remain after teardown
- [ ] Web UI is accessible at localhost:WEBUI_PORT after provisioning
- [ ] MinIO console is accessible at localhost:MINIO_CONSOLE_PORT in multi-cluster mode

## 7. Non-Applicable Domains

**SEC** (Security): Not applicable because the local development environment runs entirely on the developer's machine with no network exposure. All endpoints are localhost-only. Authentication and authorization for the API are handled by the Security feature (`cpt-katapult-feature-security`). No new auth boundaries are introduced.

**PERF** (Performance): Not applicable as a standalone concern. The only performance-relevant requirement is the startup time NFR (`cpt-katapult-nfr-local-env-startup` — <3 minutes), which is covered as an acceptance criterion. No runtime performance optimization is needed for developer tooling.

**UX** (Usability): Not applicable because this feature has no graphical user interface. Developers interact exclusively via Makefile targets (`make local-up`, `make local-down`, `make local-rebuild`). CLI usability is addressed through clear error messages and summary output documented in the actor flows.

**COMPL** (Compliance): Not applicable because this is developer tooling that runs locally. No personal data (PII) is handled — seed data is synthetic. GDPR, HIPAA, PCI DSS do not apply per PRD Section 6.2 NFR Exclusions.

**REL** (Reliability): Not applicable because the local environment is ephemeral and single-user. There are no availability, fault tolerance, or recoverability requirements. If the environment breaks, the developer tears it down and reprovisions. No data durability guarantees are needed.

**DATA** (Data): Not applicable as a standalone concern. Data handling is limited to seed data provisioning (synthetic PVCs, sample transfers) and cleanup. No real user data, no persistence requirements beyond the ephemeral environment lifecycle, no data migration or retention concerns.
