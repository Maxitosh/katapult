package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/chainstack/katapult/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DiscoveryConfig holds PVC boundary configuration.
type DiscoveryConfig struct {
	// Namespaces to discover PVCs from. Empty means all namespaces.
	Namespaces []string
	// LabelSelector to filter PVCs. Empty means no label filtering.
	LabelSelector string
	// NodeName is the current node name for affinity filtering.
	NodeName string
	// MaxRetries for Kubernetes API calls.
	MaxRetries int
	// RetryBaseDelay for exponential backoff.
	RetryBaseDelay time.Duration
}

// PVCDiscoverer discovers PVCs on the local node via the Kubernetes API.
type PVCDiscoverer struct {
	client kubernetes.Interface
	config DiscoveryConfig
	logger *slog.Logger
}

// NewPVCDiscoverer creates a new PVC discoverer.
func NewPVCDiscoverer(client kubernetes.Interface, config DiscoveryConfig, logger *slog.Logger) *PVCDiscoverer {
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryBaseDelay <= 0 {
		config.RetryBaseDelay = time.Second
	}
	return &PVCDiscoverer{client: client, config: config, logger: logger}
}

// Discover queries Kubernetes for PVCs matching the boundary config,
// resolves PV bindings, and filters by node affinity.
func (d *PVCDiscoverer) Discover(ctx context.Context) ([]domain.PVCInfo, error) {
	namespaces := d.config.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{metav1.NamespaceAll}
	}

	var inventory []domain.PVCInfo

	for _, ns := range namespaces {
		pvcs, err := d.listPVCsWithRetry(ctx, ns)
		if err != nil {
			return nil, fmt.Errorf("PVC discovery failed: Kubernetes API unavailable: %w", err)
		}

		for i := range pvcs {
			pvc := &pvcs[i]

			if pvc.Status.Phase != corev1.ClaimBound {
				continue
			}

			pvName := pvc.Spec.VolumeName
			if pvName == "" {
				continue
			}

			pv, err := d.getPVWithRetry(ctx, pvName)
			if err != nil {
				d.logger.Warn("failed to get PV, skipping PVC", "pv", pvName, "pvc", pvc.Name, "error", err)
				continue
			}

			nodeAffinity := extractNodeAffinity(pv)
			if nodeAffinity != "" && nodeAffinity != d.config.NodeName {
				continue
			}

			sizeBytes := int64(0)
			if storage, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
				sizeBytes = storage.Value()
			}

			storageClass := ""
			if pv.Spec.StorageClassName != "" {
				storageClass = pv.Spec.StorageClassName
			}

			pvcName := pvc.Namespace + "/" + pvc.Name
			inventory = append(inventory, domain.PVCInfo{
				PVCName:      pvcName,
				SizeBytes:    sizeBytes,
				StorageClass: storageClass,
				NodeAffinity: nodeAffinity,
			})
		}
	}

	return inventory, nil
}

func (d *PVCDiscoverer) listPVCsWithRetry(ctx context.Context, namespace string) ([]corev1.PersistentVolumeClaim, error) {
	opts := metav1.ListOptions{}
	if d.config.LabelSelector != "" {
		opts.LabelSelector = d.config.LabelSelector
	}

	var lastErr error
	for attempt := range d.config.MaxRetries {
		list, err := d.client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, opts)
		if err == nil {
			return list.Items, nil
		}
		lastErr = err
		d.logger.Warn("Kubernetes API call failed, retrying", "attempt", attempt+1, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d.config.RetryBaseDelay * time.Duration(1<<attempt)):
		}
	}
	return nil, lastErr
}

func (d *PVCDiscoverer) getPVWithRetry(ctx context.Context, name string) (*corev1.PersistentVolume, error) {
	var lastErr error
	for attempt := range d.config.MaxRetries {
		pv, err := d.client.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return pv, nil
		}
		lastErr = err
		if attempt < d.config.MaxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(d.config.RetryBaseDelay * time.Duration(1<<attempt)):
			}
		}
	}
	return nil, lastErr
}

// extractNodeAffinity reads the node affinity from a PV's spec.
func extractNodeAffinity(pv *corev1.PersistentVolume) string {
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return ""
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == "kubernetes.io/hostname" && expr.Operator == corev1.NodeSelectorOpIn && len(expr.Values) > 0 {
				return expr.Values[0]
			}
		}
	}
	return ""
}
