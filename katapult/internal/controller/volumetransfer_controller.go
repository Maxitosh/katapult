package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/maxitosh/katapult/api/v1alpha1"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/transfer"
)

// @cpt-dod:cpt-katapult-dod-api-cli-crd-controller:p1
// @cpt-algo:cpt-katapult-algo-api-cli-reconcile-crd:p1

const (
	finalizerName = "katapult.io/transfer-cleanup"
	requeueDelay  = 5 * time.Second
)

// TransferOrchestrator defines the subset of orchestrator methods needed by the controller.
type TransferOrchestrator interface {
	CreateTransfer(ctx context.Context, req transfer.CreateTransferRequest) (*transfer.CreateTransferResponse, error)
	GetTransfer(ctx context.Context, id uuid.UUID) (*domain.Transfer, error)
	CancelTransfer(ctx context.Context, transferID uuid.UUID) error
}

// VolumeTransferReconciler reconciles VolumeTransfer CRDs.
// @cpt-flow:cpt-katapult-flow-api-cli-create-transfer-crd:p1
type VolumeTransferReconciler struct {
	client.Client
	Orchestrator TransferOrchestrator
}

// Reconcile handles a single reconciliation loop for a VolumeTransfer CR.
// @cpt-algo:cpt-katapult-algo-api-cli-reconcile-crd:p1
func (r *VolumeTransferReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-read-spec
	var vt v1alpha1.VolumeTransfer
	if err := r.Get(ctx, req.NamespacedName, &vt); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-read-spec

	// Handle deletion with finalizer.
	if !vt.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&vt, finalizerName) {
			if vt.Status.TransferID != "" {
				transferID, err := uuid.Parse(vt.Status.TransferID)
				if err == nil {
					if cancelErr := r.Orchestrator.CancelTransfer(ctx, transferID); cancelErr != nil {
						logger.Info("cancel on deletion failed (may already be terminal)", "error", cancelErr)
					}
				}
			}
			controllerutil.RemoveFinalizer(&vt, finalizerName)
			if err := r.Update(ctx, &vt); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present.
	if !controllerutil.ContainsFinalizer(&vt, finalizerName) {
		controllerutil.AddFinalizer(&vt, finalizerName)
		if err := r.Update(ctx, &vt); err != nil {
			return ctrl.Result{}, err
		}
	}

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-create-transfer
	if vt.Status.TransferID == "" {
		var strategy *string
		if vt.Spec.Strategy != "" {
			strategy = &vt.Spec.Strategy
		}
		var retryMax *int
		if vt.Spec.RetryMax > 0 {
			retryMax = &vt.Spec.RetryMax
		}

		resp, err := r.Orchestrator.CreateTransfer(ctx, transfer.CreateTransferRequest{
			SourceCluster:      vt.Spec.SourceCluster,
			SourcePVC:          vt.Spec.SourcePVC,
			DestinationCluster: vt.Spec.DestinationCluster,
			DestinationPVC:     vt.Spec.DestinationPVC,
			StrategyOverride:   strategy,
			AllowOverwrite:     vt.Spec.AllowOverwrite,
			RetryMax:           retryMax,
			CreatedBy:          fmt.Sprintf("crd/%s/%s", vt.Namespace, vt.Name),
		})

		// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-create-failed
		if err != nil {
			setCondition(&vt, "Ready", metav1.ConditionFalse, "CreateFailed", err.Error())
			if statusErr := r.Status().Update(ctx, &vt); statusErr != nil {
				logger.Error(statusErr, "failed to update status")
			}
			return ctrl.Result{RequeueAfter: requeueDelay}, nil
		}
		// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-create-failed

		// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-store-id
		vt.Status.TransferID = resp.TransferID.String()
		vt.Status.Phase = string(resp.State)
		setCondition(&vt, "Ready", metav1.ConditionFalse, "InProgress", "Transfer created")
		if err := r.Status().Update(ctx, &vt); err != nil {
			return ctrl.Result{}, err
		}
		// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-store-id

		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-create-transfer

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-poll-state
	transferID, err := uuid.Parse(vt.Status.TransferID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing transfer ID: %w", err)
	}

	t, err := r.Orchestrator.GetTransfer(ctx, transferID)
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}
	if t == nil {
		setCondition(&vt, "Ready", metav1.ConditionFalse, "NotFound", "Transfer not found")
		_ = r.Status().Update(ctx, &vt)
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	}
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-poll-state

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-update-phase
	vt.Status.Phase = string(t.State)
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-update-phase

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-update-progress
	vt.Status.BytesTransferred = t.BytesTransferred
	vt.Status.BytesTotal = t.BytesTotal
	vt.Status.ChunksCompleted = t.ChunksCompleted
	vt.Status.ChunksTotal = t.ChunksTotal
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-update-progress

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-update-conditions
	if t.IsTerminal() {
		reason := "Completed"
		if t.State == domain.TransferStateFailed {
			reason = "Failed"
		} else if t.State == domain.TransferStateCancelled {
			reason = "Cancelled"
		}
		status := metav1.ConditionTrue
		if t.State != domain.TransferStateCompleted {
			status = metav1.ConditionFalse
		}
		setCondition(&vt, "Ready", status, reason, fmt.Sprintf("Transfer %s", t.State))
	} else {
		setCondition(&vt, "Ready", metav1.ConditionFalse, "InProgress", fmt.Sprintf("Transfer %s", t.State))
	}
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-update-conditions

	if err := r.Status().Update(ctx, &vt); err != nil {
		return ctrl.Result{}, err
	}

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-terminal
	if t.IsTerminal() {
		return ctrl.Result{}, nil
	}
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-terminal

	// @cpt-begin:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-requeue
	return ctrl.Result{RequeueAfter: requeueDelay}, nil
	// @cpt-end:cpt-katapult-algo-api-cli-reconcile-crd:p1:inst-requeue
}

// SetupWithManager registers the controller with the manager.
func (r *VolumeTransferReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VolumeTransfer{}).
		Complete(r)
}

func setCondition(vt *v1alpha1.VolumeTransfer, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	for i, c := range vt.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				vt.Status.Conditions[i].LastTransitionTime = now
			}
			vt.Status.Conditions[i].Status = status
			vt.Status.Conditions[i].Reason = reason
			vt.Status.Conditions[i].Message = message
			vt.Status.Conditions[i].ObservedGeneration = vt.Generation
			return
		}
	}
	vt.Status.Conditions = append(vt.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: vt.Generation,
	})
}
