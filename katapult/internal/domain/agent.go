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
var validTransitions = map[AgentState][]AgentState{
	AgentStateRegistering:  {AgentStateHealthy, AgentStateDisconnected},
	AgentStateHealthy:      {AgentStateUnhealthy, AgentStateRegistering},
	AgentStateUnhealthy:    {AgentStateHealthy, AgentStateDisconnected},
	AgentStateDisconnected: {AgentStateRegistering},
}

// TransitionTo attempts to transition the agent to the target state.
// Returns an error if the transition is invalid.
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
