//go:build e2e

// @cpt-flow:cpt-katapult-flow-integration-tests-run-e2e-tests:p2

package e2e_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestE2E_CLI_TransferList(t *testing.T) {
	cmd := exec.Command(
		"go", "run", "../../cmd/katapult/main.go",
		"transfer", "list",
		"--server", httpBaseURL,
		"--token", "test-operator-token",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI transfer list failed: %s\n%s", err, out)
	}

	output := string(out)
	t.Logf("transfer list output:\n%s", output)

	// Verify the output contains expected table headers.
	expectedColumns := []string{"ID", "STATE"}
	for _, col := range expectedColumns {
		if !strings.Contains(strings.ToUpper(output), col) {
			t.Errorf("expected column %q in transfer list output, not found in:\n%s", col, output)
		}
	}
}

func TestE2E_CLI_AgentList(t *testing.T) {
	cmd := exec.Command(
		"go", "run", "../../cmd/katapult/main.go",
		"agent", "list",
		"--server", httpBaseURL,
		"--token", "test-operator-token",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI agent list failed: %s\n%s", err, out)
	}

	output := string(out)
	t.Logf("agent list output:\n%s", output)

	// Verify agent list produces output (at least one agent should be running).
	if strings.TrimSpace(output) == "" {
		t.Error("expected non-empty agent list output")
	}
}
