//go:build e2e

// @cpt-flow:cpt-katapult-flow-integration-tests-run-e2e-tests:p2
// @cpt-algo:cpt-katapult-algo-integration-tests-data-integrity-check:p2

package e2e_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestE2E_CreateTransfer_ReturnsCreated(t *testing.T) {
	reqBody := map[string]any{
		"source_cluster":      "kind",
		"source_pvc":          "default/src-api-test",
		"destination_cluster": "kind",
		"destination_pvc":     "default/dst-api-test",
	}

	resp := httpDo(t, http.MethodPost, "/api/v1alpha1/transfers", jsonBody(t, reqBody))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 from POST /transfers, got %d: %s", resp.StatusCode, body)
	}

	var created struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode response: %s", err)
	}
	if created.ID == "" {
		t.Fatal("transfer ID is empty")
	}
	if created.State == "" {
		t.Fatal("state is empty in response")
	}
	t.Logf("transfer created: id=%s state=%s", created.ID, created.State)

	// Verify the transfer appears in the list.
	listResp := httpDo(t, http.MethodGet, "/api/v1alpha1/transfers", nil)
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		t.Fatalf("expected 200 from GET /transfers, got %d: %s", listResp.StatusCode, body)
	}

	var list struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode list response: %s", err)
	}

	found := false
	for _, item := range list.Items {
		if item.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created transfer %s not found in list", created.ID)
	}
}

func TestE2E_GetTransfer_ReturnsDetail(t *testing.T) {
	// Create a transfer first.
	reqBody := map[string]any{
		"source_cluster":      "kind",
		"source_pvc":          "default/src-detail",
		"destination_cluster": "kind",
		"destination_pvc":     "default/dst-detail",
	}

	resp := httpDo(t, http.MethodPost, "/api/v1alpha1/transfers", jsonBody(t, reqBody))
	defer resp.Body.Close()

	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode create response: %s", err)
	}

	// Get transfer detail.
	getResp := httpDo(t, http.MethodGet, "/api/v1alpha1/transfers/"+created.ID, nil)
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("expected 200 from GET /transfers/%s, got %d: %s", created.ID, getResp.StatusCode, body)
	}

	var detail struct {
		ID               string `json:"id"`
		State            string `json:"state"`
		SourceCluster    string `json:"source_cluster"`
		DestinationCluster string `json:"destination_cluster"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode detail response: %s", err)
	}

	if detail.ID != created.ID {
		t.Fatalf("expected id %q, got %q", created.ID, detail.ID)
	}
	if detail.SourceCluster != "kind" {
		t.Fatalf("expected source_cluster 'kind', got %q", detail.SourceCluster)
	}
}

// TestE2E_IntraClusterStreamingTransfer tests the full data-movement pipeline.
// Skipped until the agent commander is wired to actually transfer data.
func TestE2E_IntraClusterStreamingTransfer(t *testing.T) {
	t.Skip("requires wired agent commander for actual data transfer")

	const namespace = "e2e-transfer"
	createNamespace(t, namespace)

	createTestPVC(t, namespace, "src-stream", "1Gi")
	populatePVC(t, namespace, "src-stream", map[string]string{
		"file1.txt":        "hello world",
		"subdir/file2.txt": "katapult e2e test data",
		"binary.bin":       "\x00\x01\x02\x03\x04\x05",
	})
	createTestPVC(t, namespace, "dst-stream", "1Gi")

	reqBody := map[string]any{
		"source_cluster":      "kind",
		"source_pvc":          namespace + "/src-stream",
		"destination_cluster": "kind",
		"destination_pvc":     namespace + "/dst-stream",
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
	json.NewDecoder(resp.Body).Decode(&created)
	t.Logf("transfer created: %s", created.ID)

	// @cpt-begin:cpt-katapult-algo-integration-tests-data-integrity-check:p2:inst-verify-streaming
	// Uncomment when commander is wired:
	// state := waitForTransferComplete(t, created.ID, 5*time.Minute)
	// srcChecksum := computePVCChecksum(t, kubeconfig, namespace, "src-stream")
	// dstChecksum := computePVCChecksum(t, kubeconfig, namespace, "dst-stream")
	// @cpt-end:cpt-katapult-algo-integration-tests-data-integrity-check:p2:inst-verify-streaming
}

// TestE2E_CrossClusterS3StagedTransfer tests the S3-staged transfer pipeline.
// Skipped until the agent commander is wired to actually transfer data.
func TestE2E_CrossClusterS3StagedTransfer(t *testing.T) {
	t.Skip("requires wired agent commander for actual data transfer")

	const namespace = "e2e-transfer-s3"
	createNamespace(t, namespace)

	createTestPVC(t, namespace, "src-s3", "1Gi")
	populatePVC(t, namespace, "src-s3", map[string]string{
		"doc.txt":          "cross cluster test content",
		"nested/data.csv": "a,b,c\n1,2,3\n4,5,6",
	})
	createTestPVC(t, namespace, "dst-s3", "1Gi")

	reqBody := map[string]any{
		"source_cluster":      "kind",
		"source_pvc":          namespace + "/src-s3",
		"destination_cluster": "kind",
		"destination_pvc":     namespace + "/dst-s3",
		"strategy":            "s3",
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
	json.NewDecoder(resp.Body).Decode(&created)
	t.Logf("s3 transfer created: %s", created.ID)

	// @cpt-begin:cpt-katapult-algo-integration-tests-data-integrity-check:p2:inst-verify-s3
	// Uncomment when commander is wired:
	// state := waitForTransferComplete(t, created.ID, 5*time.Minute)
	// srcChecksum := computePVCChecksum(t, kubeconfig, namespace, "src-s3")
	// dstChecksum := computePVCChecksum(t, kubeconfig, namespace, "dst-s3")
	// @cpt-end:cpt-katapult-algo-integration-tests-data-integrity-check:p2:inst-verify-s3
}
