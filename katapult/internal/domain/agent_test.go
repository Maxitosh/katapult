package domain

import (
	"testing"

	"github.com/google/uuid"
)

func newTestAgent(state AgentState) *Agent {
	return &Agent{
		ID:    uuid.New(),
		State: state,
	}
}

func TestTransitionTo_ValidTransitions(t *testing.T) {
	tests := []struct {
		from AgentState
		to   AgentState
	}{
		{AgentStateRegistering, AgentStateHealthy},
		{AgentStateRegistering, AgentStateDisconnected},
		{AgentStateHealthy, AgentStateUnhealthy},
		{AgentStateHealthy, AgentStateRegistering},
		{AgentStateUnhealthy, AgentStateHealthy},
		{AgentStateUnhealthy, AgentStateDisconnected},
		{AgentStateDisconnected, AgentStateRegistering},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			a := newTestAgent(tt.from)
			if err := a.TransitionTo(tt.to); err != nil {
				t.Fatalf("expected valid transition from %s to %s, got error: %v", tt.from, tt.to, err)
			}
			if a.State != tt.to {
				t.Fatalf("expected state %s, got %s", tt.to, a.State)
			}
		})
	}
}

func TestTransitionTo_InvalidTransitions(t *testing.T) {
	tests := []struct {
		from AgentState
		to   AgentState
	}{
		{AgentStateDisconnected, AgentStateHealthy},
		{AgentStateHealthy, AgentStateDisconnected},
		{AgentStateRegistering, AgentStateUnhealthy},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			a := newTestAgent(tt.from)
			if err := a.TransitionTo(tt.to); err == nil {
				t.Fatalf("expected error for invalid transition from %s to %s", tt.from, tt.to)
			}
			if a.State != tt.from {
				t.Fatalf("state should not change on invalid transition, expected %s got %s", tt.from, a.State)
			}
		})
	}
}

func TestTransitionTo_HealthyFlag(t *testing.T) {
	a := newTestAgent(AgentStateRegistering)
	_ = a.TransitionTo(AgentStateHealthy)
	if !a.Healthy {
		t.Fatal("expected Healthy=true after transitioning to healthy")
	}

	_ = a.TransitionTo(AgentStateUnhealthy)
	if a.Healthy {
		t.Fatal("expected Healthy=false after transitioning to unhealthy")
	}
}
