//go:build integration

// @cpt-dod:cpt-katapult-dod-integration-tests-controller-tests:p2
// @cpt-flow:cpt-katapult-flow-integration-tests-run-controller-tests:p2
// @cpt-algo:cpt-katapult-algo-integration-tests-envtest-setup:p2

package controller_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"

	v1alpha1 "github.com/maxitosh/katapult/api/v1alpha1"
	"github.com/maxitosh/katapult/internal/controller"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/testutil"
	"github.com/maxitosh/katapult/internal/transfer"
)

// mockOrchestrator implements controller.TransferOrchestrator with configurable
// return values and call recording for test assertions.
type mockOrchestrator struct {
	mu sync.Mutex

	createResp *transfer.CreateTransferResponse
	createErr  error

	getTransfer *domain.Transfer
	getErr      error

	cancelErr error

	createCalled bool
	cancelCalled bool
}

func (m *mockOrchestrator) CreateTransfer(_ context.Context, _ transfer.CreateTransferRequest) (*transfer.CreateTransferResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalled = true
	return m.createResp, m.createErr
}

func (m *mockOrchestrator) GetTransfer(_ context.Context, _ uuid.UUID) (*domain.Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getTransfer, m.getErr
}

func (m *mockOrchestrator) CancelTransfer(_ context.Context, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelCalled = true
	return m.cancelErr
}

func (m *mockOrchestrator) wasCreateCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createCalled
}

func (m *mockOrchestrator) wasCancelCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancelCalled
}

// pollVolumeTransfer repeatedly fetches the VolumeTransfer CR until the check function
// returns true or the timeout expires.
func pollVolumeTransfer(t *testing.T, k8sClient client.Client, key types.NamespacedName, timeout time.Duration, check func(*v1alpha1.VolumeTransfer) bool) *v1alpha1.VolumeTransfer {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var vt v1alpha1.VolumeTransfer
		if err := k8sClient.Get(context.Background(), key, &vt); err == nil {
			if check(&vt) {
				return &vt
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for VolumeTransfer %s to satisfy check", key)
	return nil
}

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-vt-finalizer

func TestReconcile_CreateVT_AddsFinalizerAndCreatesTransfer(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	transferID := uuid.New()
	mockOrch := &mockOrchestrator{
		createResp: &transfer.CreateTransferResponse{
			TransferID: transferID,
			State:      domain.TransferStatePending,
		},
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: k8sClient.Scheme(), Controller: config.Controller{SkipNameValidation: ptr.To(true)}})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}

	if err := (&controller.VolumeTransferReconciler{
		Client:       mgr.GetClient(),
		Orchestrator: mockOrch,
	}).SetupWithManager(mgr); err != nil {
		t.Fatalf("setting up reconciler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	vt := &v1alpha1.VolumeTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vt-create",
			Namespace: "default",
		},
		Spec: v1alpha1.VolumeTransferSpec{
			SourceCluster:      "cluster-a",
			SourcePVC:          "ns/pvc-src",
			DestinationCluster: "cluster-b",
			DestinationPVC:     "ns/pvc-dst",
		},
	}

	if err := k8sClient.Create(context.Background(), vt); err != nil {
		t.Fatalf("creating VolumeTransfer: %v", err)
	}

	key := types.NamespacedName{Name: "test-vt-create", Namespace: "default"}

	// Wait for finalizer to be added.
	got := pollVolumeTransfer(t, k8sClient, key, 10*time.Second, func(vt *v1alpha1.VolumeTransfer) bool {
		for _, f := range vt.Finalizers {
			if f == "katapult.io/transfer-cleanup" {
				return true
			}
		}
		return false
	})

	hasFinalizer := false
	for _, f := range got.Finalizers {
		if f == "katapult.io/transfer-cleanup" {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		t.Fatalf("expected finalizer katapult.io/transfer-cleanup to be present")
	}

	// Wait for orchestrator CreateTransfer to be called and status.transferID set.
	got = pollVolumeTransfer(t, k8sClient, key, 10*time.Second, func(vt *v1alpha1.VolumeTransfer) bool {
		return vt.Status.TransferID != ""
	})

	if !mockOrch.wasCreateCalled() {
		t.Fatalf("expected orchestrator.CreateTransfer to be called")
	}

	if got.Status.TransferID != transferID.String() {
		t.Fatalf("expected status.transferID = %q, got %q", transferID.String(), got.Status.TransferID)
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-vt-finalizer

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-existing-transfer-updates-status

func TestReconcile_ExistingTransfer_UpdatesStatus(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	transferID := uuid.New()
	mockOrch := &mockOrchestrator{
		getTransfer: &domain.Transfer{
			ID:               transferID,
			State:            domain.TransferStateTransferring,
			BytesTransferred: 5000,
			BytesTotal:       10000,
			ChunksCompleted:  2,
			ChunksTotal:      4,
		},
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: k8sClient.Scheme(), Controller: config.Controller{SkipNameValidation: ptr.To(true)}})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}

	if err := (&controller.VolumeTransferReconciler{
		Client:       mgr.GetClient(),
		Orchestrator: mockOrch,
	}).SetupWithManager(mgr); err != nil {
		t.Fatalf("setting up reconciler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	// Create VT with transferID already set in status (pre-seeded).
	vt := &v1alpha1.VolumeTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-vt-existing",
			Namespace:  "default",
			Finalizers: []string{"katapult.io/transfer-cleanup"},
		},
		Spec: v1alpha1.VolumeTransferSpec{
			SourceCluster:      "cluster-a",
			SourcePVC:          "ns/pvc-src",
			DestinationCluster: "cluster-b",
			DestinationPVC:     "ns/pvc-dst",
		},
	}

	if err := k8sClient.Create(context.Background(), vt); err != nil {
		t.Fatalf("creating VolumeTransfer: %v", err)
	}

	// Patch status with transferID to simulate pre-seeded state.
	vt.Status.TransferID = transferID.String()
	vt.Status.Phase = "pending"
	if err := k8sClient.Status().Update(context.Background(), vt); err != nil {
		t.Fatalf("updating VolumeTransfer status: %v", err)
	}

	key := types.NamespacedName{Name: "test-vt-existing", Namespace: "default"}

	// Wait for status to be updated with progress from GetTransfer.
	got := pollVolumeTransfer(t, k8sClient, key, 10*time.Second, func(vt *v1alpha1.VolumeTransfer) bool {
		return vt.Status.Phase == string(domain.TransferStateTransferring) && vt.Status.BytesTransferred == 5000
	})

	if got.Status.Phase != string(domain.TransferStateTransferring) {
		t.Fatalf("expected status.phase = %q, got %q", domain.TransferStateTransferring, got.Status.Phase)
	}
	if got.Status.BytesTransferred != 5000 {
		t.Fatalf("expected status.bytesTransferred = 5000, got %d", got.Status.BytesTransferred)
	}
	if got.Status.BytesTotal != 10000 {
		t.Fatalf("expected status.bytesTotal = 10000, got %d", got.Status.BytesTotal)
	}
	if got.Status.ChunksCompleted != 2 {
		t.Fatalf("expected status.chunksCompleted = 2, got %d", got.Status.ChunksCompleted)
	}
	if got.Status.ChunksTotal != 4 {
		t.Fatalf("expected status.chunksTotal = 4, got %d", got.Status.ChunksTotal)
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-existing-transfer-updates-status

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-completed-transfer-ready

func TestReconcile_CompletedTransfer_SetsReadyCondition(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	transferID := uuid.New()
	mockOrch := &mockOrchestrator{
		getTransfer: &domain.Transfer{
			ID:               transferID,
			State:            domain.TransferStateCompleted,
			BytesTransferred: 10000,
			BytesTotal:       10000,
			ChunksCompleted:  4,
			ChunksTotal:      4,
		},
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: k8sClient.Scheme(), Controller: config.Controller{SkipNameValidation: ptr.To(true)}})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}

	if err := (&controller.VolumeTransferReconciler{
		Client:       mgr.GetClient(),
		Orchestrator: mockOrch,
	}).SetupWithManager(mgr); err != nil {
		t.Fatalf("setting up reconciler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	vt := &v1alpha1.VolumeTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-vt-completed",
			Namespace:  "default",
			Finalizers: []string{"katapult.io/transfer-cleanup"},
		},
		Spec: v1alpha1.VolumeTransferSpec{
			SourceCluster:      "cluster-a",
			SourcePVC:          "ns/pvc-src",
			DestinationCluster: "cluster-b",
			DestinationPVC:     "ns/pvc-dst",
		},
	}

	if err := k8sClient.Create(context.Background(), vt); err != nil {
		t.Fatalf("creating VolumeTransfer: %v", err)
	}

	// Pre-seed status with transferID so reconciler polls GetTransfer.
	vt.Status.TransferID = transferID.String()
	vt.Status.Phase = "transferring"
	if err := k8sClient.Status().Update(context.Background(), vt); err != nil {
		t.Fatalf("updating VolumeTransfer status: %v", err)
	}

	key := types.NamespacedName{Name: "test-vt-completed", Namespace: "default"}

	// Wait for Ready=True condition with reason Completed.
	got := pollVolumeTransfer(t, k8sClient, key, 10*time.Second, func(vt *v1alpha1.VolumeTransfer) bool {
		for _, c := range vt.Status.Conditions {
			if c.Type == "Ready" && c.Status == metav1.ConditionTrue && c.Reason == "Completed" {
				return true
			}
		}
		return false
	})

	testutil.AssertCondition(t, got.Status.Conditions, "Ready", metav1.ConditionTrue, "Completed")
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-completed-transfer-ready

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-delete-vt-finalizer-cleanup

func TestReconcile_DeleteVT_RunsFinalizerCleanup(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	transferID := uuid.New()
	mockOrch := &mockOrchestrator{
		// CreateTransfer is needed for the initial reconcile before deletion.
		createResp: &transfer.CreateTransferResponse{
			TransferID: transferID,
			State:      domain.TransferStatePending,
		},
		// GetTransfer is called during polling after creation.
		getTransfer: &domain.Transfer{
			ID:    transferID,
			State: domain.TransferStateTransferring,
		},
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: k8sClient.Scheme(), Controller: config.Controller{SkipNameValidation: ptr.To(true)}})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}

	if err := (&controller.VolumeTransferReconciler{
		Client:       mgr.GetClient(),
		Orchestrator: mockOrch,
	}).SetupWithManager(mgr); err != nil {
		t.Fatalf("setting up reconciler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	vt := &v1alpha1.VolumeTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vt-delete",
			Namespace: "default",
		},
		Spec: v1alpha1.VolumeTransferSpec{
			SourceCluster:      "cluster-a",
			SourcePVC:          "ns/pvc-src",
			DestinationCluster: "cluster-b",
			DestinationPVC:     "ns/pvc-dst",
		},
	}

	if err := k8sClient.Create(context.Background(), vt); err != nil {
		t.Fatalf("creating VolumeTransfer: %v", err)
	}

	key := types.NamespacedName{Name: "test-vt-delete", Namespace: "default"}

	// Wait for finalizer and transferID to be set by reconciler.
	pollVolumeTransfer(t, k8sClient, key, 10*time.Second, func(vt *v1alpha1.VolumeTransfer) bool {
		return vt.Status.TransferID != ""
	})

	// Now delete the VolumeTransfer. The finalizer should trigger CancelTransfer.
	toDelete := &v1alpha1.VolumeTransfer{}
	if err := k8sClient.Get(context.Background(), key, toDelete); err != nil {
		t.Fatalf("getting VolumeTransfer for deletion: %v", err)
	}
	if err := k8sClient.Delete(context.Background(), toDelete); err != nil {
		t.Fatalf("deleting VolumeTransfer: %v", err)
	}

	// Wait for the object to be fully removed (finalizer cleared).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.VolumeTransfer
		err := k8sClient.Get(context.Background(), key, &check)
		if err != nil {
			// Object deleted successfully.
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !mockOrch.wasCancelCalled() {
		t.Fatalf("expected orchestrator.CancelTransfer to be called during finalizer cleanup")
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-delete-vt-finalizer-cleanup

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-fails-condition

func TestReconcile_CreateFails_SetsFailedCondition(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	mockOrch := &mockOrchestrator{
		createErr: fmt.Errorf("validation failed: source PVC not found"),
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: k8sClient.Scheme(), Controller: config.Controller{SkipNameValidation: ptr.To(true)}})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}

	if err := (&controller.VolumeTransferReconciler{
		Client:       mgr.GetClient(),
		Orchestrator: mockOrch,
	}).SetupWithManager(mgr); err != nil {
		t.Fatalf("setting up reconciler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	vt := &v1alpha1.VolumeTransfer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vt-create-fail",
			Namespace: "default",
		},
		Spec: v1alpha1.VolumeTransferSpec{
			SourceCluster:      "cluster-a",
			SourcePVC:          "ns/pvc-src",
			DestinationCluster: "cluster-b",
			DestinationPVC:     "ns/pvc-dst",
		},
	}

	if err := k8sClient.Create(context.Background(), vt); err != nil {
		t.Fatalf("creating VolumeTransfer: %v", err)
	}

	key := types.NamespacedName{Name: "test-vt-create-fail", Namespace: "default"}

	// Wait for the Ready=False condition with reason CreateFailed.
	got := pollVolumeTransfer(t, k8sClient, key, 10*time.Second, func(vt *v1alpha1.VolumeTransfer) bool {
		for _, c := range vt.Status.Conditions {
			if c.Type == "Ready" && c.Status == metav1.ConditionFalse && c.Reason == "CreateFailed" {
				return true
			}
		}
		return false
	})

	testutil.AssertCondition(t, got.Status.Conditions, "Ready", metav1.ConditionFalse, "CreateFailed")
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-fails-condition
