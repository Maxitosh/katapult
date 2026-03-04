package domain

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// AgentState represents the lifecycle state of an agent.
type AgentState string

const (
	AgentStateRegistering  AgentState = "registering"
	AgentStateHealthy      AgentState = "healthy"
	AgentStateUnhealthy    AgentState = "unhealthy"
	AgentStateDisconnected AgentState = "disconnected"
)

// ToolVersions holds the versions of required tools on an agent node.
type ToolVersions struct {
	Tar     string `json:"tar"`
	Zstd    string `json:"zstd"`
	Stunnel string `json:"stunnel"`
}

// Agent represents a registered agent running on a Kubernetes worker node.
type Agent struct {
	ID            uuid.UUID    `json:"id"`
	ClusterID     string       `json:"cluster_id"`
	NodeName      string       `json:"node_name"`
	State         AgentState   `json:"state"`
	Healthy       bool         `json:"healthy"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	Tools         ToolVersions `json:"tools"`
	RegisteredAt  time.Time    `json:"registered_at"`
	PVCs          []PVCInfo    `json:"pvcs,omitempty"`
	// JWTNamespace stores the Kubernetes namespace from the JWT at registration
	// time, creating an immutable binding between agent_id and JWT identity.
	JWTNamespace string `json:"jwt_namespace"`
}

// PVCInfo represents a discovered PVC with its resolved PV metadata.
type PVCInfo struct {
	PVCName      string `json:"pvc_name"`
	SizeBytes    int64  `json:"size_bytes"`
	StorageClass string `json:"storage_class"`
	NodeAffinity string `json:"node_affinity"`
}

// validTransitions defines the allowed state transitions for the agent lifecycle.
// @cpt-state:cpt-katapult-state-agent-system-agent-lifecycle:p1
var validTransitions = map[AgentState][]AgentState{
	// @cpt-begin:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-reg-to-healthy
	// @cpt-begin:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-reg-to-disconnected
	AgentStateRegistering: {AgentStateHealthy, AgentStateDisconnected},
	// @cpt-end:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-reg-to-disconnected
	// @cpt-end:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-reg-to-healthy
	// @cpt-begin:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-healthy-to-unhealthy
	// @cpt-begin:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-healthy-to-registering
	AgentStateHealthy: {AgentStateUnhealthy, AgentStateRegistering},
	// @cpt-end:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-healthy-to-registering
	// @cpt-end:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-healthy-to-unhealthy
	// @cpt-begin:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-unhealthy-to-healthy
	// @cpt-begin:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-unhealthy-to-disconnected
	AgentStateUnhealthy: {AgentStateHealthy, AgentStateDisconnected},
	// @cpt-end:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-unhealthy-to-disconnected
	// @cpt-end:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-unhealthy-to-healthy
	// @cpt-begin:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-disconnected-to-registering
	AgentStateDisconnected: {AgentStateRegistering},
	// @cpt-end:cpt-katapult-state-agent-system-agent-lifecycle:p1:inst-disconnected-to-registering
}

// TransitionTo attempts to transition the agent to the target state.
// Returns an error if the transition is invalid.
// @cpt-state:cpt-katapult-state-agent-system-agent-lifecycle:p1
func (a *Agent) TransitionTo(target AgentState) error {
	allowed, ok := validTransitions[a.State]
	if !ok {
		return fmt.Errorf("no transitions defined from state %q", a.State)
	}
	if !slices.Contains(allowed, target) {
		return fmt.Errorf("invalid transition from %q to %q", a.State, target)
	}
	a.State = target
	a.Healthy = target == AgentStateHealthy
	return nil
}
