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

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
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

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-vt-finalizer

func TestReconcile_CreateVT_AddsFinalizerAndCreatesTransfer(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	g := gomega.NewWithT(t)
	ns := testutil.CreateTestNamespace(t, k8sClient, "ctrl-test")

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
			Namespace: ns,
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

	key := types.NamespacedName{Name: "test-vt-create", Namespace: ns}

	// Wait for finalizer to be added.
	g.Eventually(func(g gomega.Gomega) {
		var vt v1alpha1.VolumeTransfer
		g.Expect(k8sClient.Get(context.Background(), key, &vt)).To(gomega.Succeed())
		g.Expect(vt.Finalizers).To(gomega.ContainElement("katapult.io/transfer-cleanup"))
	}, testutil.ShortTimeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())

	// Wait for orchestrator CreateTransfer to be called and status.transferID set.
	g.Eventually(func(g gomega.Gomega) {
		var vt v1alpha1.VolumeTransfer
		g.Expect(k8sClient.Get(context.Background(), key, &vt)).To(gomega.Succeed())
		g.Expect(vt.Status.TransferID).NotTo(gomega.BeEmpty())
		g.Expect(vt.Status.TransferID).To(gomega.Equal(transferID.String()))
	}, testutil.ShortTimeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())

	if !mockOrch.wasCreateCalled() {
		t.Fatalf("expected orchestrator.CreateTransfer to be called")
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-vt-finalizer

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-existing-transfer-updates-status

func TestReconcile_ExistingTransfer_UpdatesStatus(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	g := gomega.NewWithT(t)
	ns := testutil.CreateTestNamespace(t, k8sClient, "ctrl-test")

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
			Namespace:  ns,
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

	key := types.NamespacedName{Name: "test-vt-existing", Namespace: ns}

	// Wait for status to be updated with progress from GetTransfer.
	g.Eventually(func(g gomega.Gomega) {
		var vt v1alpha1.VolumeTransfer
		g.Expect(k8sClient.Get(context.Background(), key, &vt)).To(gomega.Succeed())
		g.Expect(vt.Status.Phase).To(gomega.Equal(string(domain.TransferStateTransferring)))
		g.Expect(vt.Status.BytesTransferred).To(gomega.Equal(int64(5000)))
		g.Expect(vt.Status.BytesTotal).To(gomega.Equal(int64(10000)))
		g.Expect(vt.Status.ChunksCompleted).To(gomega.Equal(2))
		g.Expect(vt.Status.ChunksTotal).To(gomega.Equal(4))
	}, testutil.ShortTimeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-existing-transfer-updates-status

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-completed-transfer-ready

func TestReconcile_CompletedTransfer_SetsReadyCondition(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	g := gomega.NewWithT(t)
	ns := testutil.CreateTestNamespace(t, k8sClient, "ctrl-test")

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
			Namespace:  ns,
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

	key := types.NamespacedName{Name: "test-vt-completed", Namespace: ns}

	// Wait for Ready=True condition with reason Completed.
	g.Eventually(func(g gomega.Gomega) {
		var vt v1alpha1.VolumeTransfer
		g.Expect(k8sClient.Get(context.Background(), key, &vt)).To(gomega.Succeed())
		testutil.ExpectCondition(g, vt.Status.Conditions, "Ready", metav1.ConditionTrue, "Completed")
	}, testutil.ShortTimeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-completed-transfer-ready

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-delete-vt-finalizer-cleanup

func TestReconcile_DeleteVT_RunsFinalizerCleanup(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	g := gomega.NewWithT(t)
	ns := testutil.CreateTestNamespace(t, k8sClient, "ctrl-test")

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
			Namespace: ns,
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

	key := types.NamespacedName{Name: "test-vt-delete", Namespace: ns}

	// Wait for finalizer and transferID to be set by reconciler.
	g.Eventually(func(g gomega.Gomega) {
		var vt v1alpha1.VolumeTransfer
		g.Expect(k8sClient.Get(context.Background(), key, &vt)).To(gomega.Succeed())
		g.Expect(vt.Status.TransferID).NotTo(gomega.BeEmpty())
	}, testutil.ShortTimeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())

	// Now delete the VolumeTransfer. The finalizer should trigger CancelTransfer.
	toDelete := &v1alpha1.VolumeTransfer{}
	if err := k8sClient.Get(context.Background(), key, toDelete); err != nil {
		t.Fatalf("getting VolumeTransfer for deletion: %v", err)
	}
	if err := k8sClient.Delete(context.Background(), toDelete); err != nil {
		t.Fatalf("deleting VolumeTransfer: %v", err)
	}

	// Wait for the object to be fully removed (finalizer cleared).
	g.Eventually(func() error {
		var check v1alpha1.VolumeTransfer
		return k8sClient.Get(context.Background(), key, &check)
	}, testutil.ShortTimeout, testutil.DefaultPollingInterval).ShouldNot(gomega.Succeed())

	if !mockOrch.wasCancelCalled() {
		t.Fatalf("expected orchestrator.CancelTransfer to be called during finalizer cleanup")
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-delete-vt-finalizer-cleanup

// @cpt-begin:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-fails-condition

func TestReconcile_CreateFails_SetsFailedCondition(t *testing.T) {
	cfg, k8sClient, cleanup := testutil.SetupEnvtest(t, "../../../deploy/crd/bases")
	defer cleanup()

	g := gomega.NewWithT(t)
	ns := testutil.CreateTestNamespace(t, k8sClient, "ctrl-test")

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
			Namespace: ns,
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

	key := types.NamespacedName{Name: "test-vt-create-fail", Namespace: ns}

	// Wait for the Ready=False condition with reason CreateFailed.
	g.Eventually(func(g gomega.Gomega) {
		var vt v1alpha1.VolumeTransfer
		g.Expect(k8sClient.Get(context.Background(), key, &vt)).To(gomega.Succeed())
		testutil.ExpectCondition(g, vt.Status.Conditions, "Ready", metav1.ConditionFalse, "CreateFailed")
	}, testutil.ShortTimeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())
}

// @cpt-end:cpt-katapult-dod-integration-tests-controller-tests:p2:inst-create-fails-condition
