package testutil

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/maxitosh/katapult/internal/domain"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-shared-helpers:p2

// @cpt-begin:cpt-katapult-dod-integration-tests-shared-helpers:p2:inst-assertion-helpers

// AssertCondition verifies that a Kubernetes condition with the given type exists
// and matches the expected status and reason.
func AssertCondition(t *testing.T, conditions []metav1.Condition, condType string, status metav1.ConditionStatus, reason string) {
	t.Helper()
	for _, c := range conditions {
		if c.Type == condType {
			if c.Status != status {
				t.Fatalf("condition %q: expected status %q, got %q", condType, status, c.Status)
			}
			if c.Reason != reason {
				t.Fatalf("condition %q: expected reason %q, got %q", condType, reason, c.Reason)
			}
			return
		}
	}
	t.Fatalf("condition %q not found in %d conditions", condType, len(conditions))
}

// AssertTransferState verifies that a transfer is in the expected state.
func AssertTransferState(t *testing.T, actual *domain.Transfer, expected domain.TransferState) {
	t.Helper()
	if actual == nil {
		t.Fatal("expected transfer, got nil")
	}
	if actual.State != expected {
		t.Fatalf("expected transfer state %q, got %q", expected, actual.State)
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-shared-helpers:p2:inst-assertion-helpers
