//go:build e2e

// @cpt-flow:cpt-katapult-flow-integration-tests-run-e2e-tests:p2

package e2e_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestE2E_TransferCancellation_CleansUp(t *testing.T) {
	// Create a transfer.
	reqBody := map[string]any{
		"source_cluster":      "kind",
		"source_pvc":          "default/src-cancel",
		"destination_cluster": "kind",
		"destination_pvc":     "default/dst-cancel",
	}

	resp := httpDo(t, http.MethodPost, "/api/v1alpha1/transfers", jsonBody(t, reqBody))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 from POST /transfers, got %d: %s", resp.StatusCode, body)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode transfer response: %s", err)
	}
	if created.ID == "" {
		t.Fatal("transfer ID is empty")
	}

	t.Logf("transfer to cancel: %s", created.ID)

	// Cancel the transfer.
	cancelResp := httpDo(t, http.MethodDelete, "/api/v1alpha1/transfers/"+created.ID, nil)
	defer cancelResp.Body.Close()

	if cancelResp.StatusCode != http.StatusOK && cancelResp.StatusCode != http.StatusAccepted && cancelResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(cancelResp.Body)
		t.Fatalf("expected 200/202/204 from DELETE /transfers/%s, got %d: %s", created.ID, cancelResp.StatusCode, body)
	}

	// Poll until the transfer reaches cancelled state.
	state := waitForTransferComplete(t, created.ID, 30*time.Second)
	if state != "cancelled" {
		t.Fatalf("expected transfer state 'cancelled', got %q", state)
	}
	t.Log("transfer successfully cancelled")
}
