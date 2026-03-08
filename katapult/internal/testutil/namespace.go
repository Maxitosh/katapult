//go:build integration

package testutil

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateTestNamespace creates an isolated namespace with GenerateName for parallel-safe
// test execution. It registers a t.Cleanup to delete the namespace when the test finishes.
func CreateTestNamespace(t *testing.T, k8sClient client.Client, prefix string) string {
	t.Helper()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix + "-",
		},
	}
	if err := k8sClient.Create(context.Background(), ns); err != nil {
		t.Fatalf("CreateTestNamespace: failed to create namespace with prefix %q: %v", prefix, err)
	}

	name := ns.Name
	t.Cleanup(func() {
		_ = k8sClient.Delete(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		})
	})

	return name
}
