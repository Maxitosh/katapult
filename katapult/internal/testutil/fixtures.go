package testutil

import (
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-shared-helpers:p2

// AgentOption configures a test Agent.
type AgentOption func(*domain.Agent)

// @cpt-begin:cpt-katapult-dod-integration-tests-shared-helpers:p2:inst-fixture-builders

// WithAgentCluster sets the cluster ID.
func WithAgentCluster(cluster string) AgentOption {
	return func(a *domain.Agent) { a.ClusterID = cluster }
}

// WithAgentNode sets the node name.
func WithAgentNode(node string) AgentOption {
	return func(a *domain.Agent) { a.NodeName = node }
}

// WithAgentState sets the agent state.
func WithAgentState(state domain.AgentState) AgentOption {
	return func(a *domain.Agent) {
		a.State = state
		a.Healthy = state == domain.AgentStateHealthy
	}
}

// WithAgentPVCs sets the PVC list.
func WithAgentPVCs(pvcs []domain.PVCInfo) AgentOption {
	return func(a *domain.Agent) { a.PVCs = pvcs }
}

// WithAgentJWTNamespace sets the JWT namespace.
func WithAgentJWTNamespace(ns string) AgentOption {
	return func(a *domain.Agent) { a.JWTNamespace = ns }
}

// NewTestAgent creates a test Agent with sensible defaults and optional overrides.
func NewTestAgent(opts ...AgentOption) *domain.Agent {
	a := &domain.Agent{
		ID:            uuid.New(),
		ClusterID:     "test-cluster",
		NodeName:      "test-node",
		State:         domain.AgentStateHealthy,
		Healthy:       true,
		LastHeartbeat: time.Now(),
		Tools:         domain.ToolVersions{Tar: "1.35", Zstd: "1.5.5", Stunnel: "5.72"},
		RegisteredAt:  time.Now(),
		JWTNamespace:  "default",
		PVCs: []domain.PVCInfo{
			{PVCName: "default/test-pvc", SizeBytes: 1024, StorageClass: "standard", NodeAffinity: "test-node"},
		},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// TransferOption configures a test Transfer.
type TransferOption func(*domain.Transfer)

// WithTransferSource sets source cluster and PVC.
func WithTransferSource(cluster, pvc string) TransferOption {
	return func(t *domain.Transfer) {
		t.SourceCluster = cluster
		t.SourcePVC = pvc
	}
}

// WithTransferDest sets destination cluster and PVC.
func WithTransferDest(cluster, pvc string) TransferOption {
	return func(t *domain.Transfer) {
		t.DestinationCluster = cluster
		t.DestinationPVC = pvc
	}
}

// WithTransferState sets the transfer state.
func WithTransferState(state domain.TransferState) TransferOption {
	return func(t *domain.Transfer) { t.State = state }
}

// WithTransferStrategy sets the transfer strategy.
func WithTransferStrategy(strategy domain.TransferStrategy) TransferOption {
	return func(t *domain.Transfer) { t.Strategy = strategy }
}

// WithTransferCreatedBy sets the created_by field.
func WithTransferCreatedBy(by string) TransferOption {
	return func(t *domain.Transfer) { t.CreatedBy = by }
}

// NewTestTransfer creates a test Transfer with sensible defaults and optional overrides.
func NewTestTransfer(opts ...TransferOption) *domain.Transfer {
	t := &domain.Transfer{
		ID:                 uuid.New(),
		SourceCluster:      "cluster-a",
		SourcePVC:          "ns/pvc-source",
		DestinationCluster: "cluster-b",
		DestinationPVC:     "ns/pvc-dest",
		State:              domain.TransferStatePending,
		AllowOverwrite:     false,
		RetryMax:           3,
		CreatedBy:          "test-operator",
		CreatedAt:          time.Now(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// NewTestPVC creates a test PVCInfo.
func NewTestPVC(name string) domain.PVCInfo {
	return domain.PVCInfo{
		PVCName:      name,
		SizeBytes:    1024 * 1024 * 100,
		StorageClass: "standard",
		NodeAffinity: "test-node",
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-shared-helpers:p2:inst-fixture-builders
