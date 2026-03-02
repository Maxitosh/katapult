package agent

import (
	"context"
	"log/slog"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func setupFakeCluster(t *testing.T) *fake.Clientset {
	t.Helper()

	client := fake.NewSimpleClientset(
		// PVC: bound, correct node
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-data"},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		},
		// PVC: bound, wrong node
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "remote-pvc", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-remote"},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		},
		// PVC: pending (should be skipped)
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pending-pvc", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: ""},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		},
		// PV: local node
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-data"},
			Spec: corev1.PersistentVolumeSpec{
				Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("100Gi")},
				StorageClassName: "local-path",
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"worker-1"},
							}},
						}},
					},
				},
			},
		},
		// PV: different node
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-remote"},
			Spec: corev1.PersistentVolumeSpec{
				Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("200Gi")},
				StorageClassName: "local-path",
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"worker-2"},
							}},
						}},
					},
				},
			},
		},
	)
	return client
}

func TestDiscover_FiltersCorrectly(t *testing.T) {
	client := setupFakeCluster(t)

	disc := NewPVCDiscoverer(client, DiscoveryConfig{
		NodeName:       "worker-1",
		MaxRetries:     1,
		RetryBaseDelay: time.Millisecond,
	}, slog.Default())

	pvcs, err := disc.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pvcs) != 1 {
		t.Fatalf("expected 1 PVC (local node only), got %d", len(pvcs))
	}

	pvc := pvcs[0]
	if pvc.PVCName != "default/data-pvc" {
		t.Fatalf("expected default/data-pvc, got %s", pvc.PVCName)
	}
	if pvc.StorageClass != "local-path" {
		t.Fatalf("expected storage class local-path, got %s", pvc.StorageClass)
	}
	if pvc.NodeAffinity != "worker-1" {
		t.Fatalf("expected node affinity worker-1, got %s", pvc.NodeAffinity)
	}
	// 100Gi = 107374182400 bytes
	if pvc.SizeBytes != 107374182400 {
		t.Fatalf("expected 107374182400 bytes, got %d", pvc.SizeBytes)
	}
}

func TestDiscover_NamespaceFilter(t *testing.T) {
	client := setupFakeCluster(t)

	disc := NewPVCDiscoverer(client, DiscoveryConfig{
		NodeName:       "worker-1",
		Namespaces:     []string{"other-ns"}, // No PVCs in this namespace.
		MaxRetries:     1,
		RetryBaseDelay: time.Millisecond,
	}, slog.Default())

	pvcs, err := disc.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pvcs) != 0 {
		t.Fatalf("expected 0 PVCs in other-ns, got %d", len(pvcs))
	}
}

func TestDiscover_SkipsPendingPVCs(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "pending-pvc", Namespace: "default"},
			Spec:       corev1.PersistentVolumeClaimSpec{},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		},
	)

	disc := NewPVCDiscoverer(client, DiscoveryConfig{
		NodeName:       "worker-1",
		MaxRetries:     1,
		RetryBaseDelay: time.Millisecond,
	}, slog.Default())

	pvcs, err := disc.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pvcs) != 0 {
		t.Fatalf("expected 0 PVCs (all pending), got %d", len(pvcs))
	}
}
