//go:build integration

package testutil

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	v1alpha1 "github.com/maxitosh/katapult/api/v1alpha1"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-shared-helpers:p2
// @cpt-algo:cpt-katapult-algo-integration-tests-envtest-setup:p2

// @cpt-begin:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-resolve-envtest-assets

// SetupEnvtest starts an envtest environment with the VolumeTransfer CRD installed.
// It returns the REST config, a Kubernetes client, and a cancel function for cleanup.
// crdPath should point to the directory containing CRD YAML files (e.g., deploy/crd/bases).
func SetupEnvtest(t *testing.T, crdPath string) (*rest.Config, client.Client, context.CancelFunc) {
	t.Helper()

	// @cpt-begin:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-start-envtest
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{crdPath},
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-start-envtest

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("starting envtest: %v (hint: run 'setup-envtest use' to install binaries)", err)
	}

	// @cpt-begin:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-create-manager
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		testEnv.Stop()
		t.Fatalf("adding client-go scheme: %v", err)
	}
	if err := v1alpha1.AddToScheme(s); err != nil {
		testEnv.Stop()
		t.Fatalf("adding v1alpha1 scheme: %v", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		testEnv.Stop()
		t.Fatalf("creating k8s client: %v", err)
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-create-manager

	// @cpt-begin:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-return-envtest
	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx // caller uses cancel for cleanup

	cleanup := func() {
		cancel()
		if err := testEnv.Stop(); err != nil {
			t.Logf("stopping envtest: %v", err)
		}
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-return-envtest

	return cfg, k8sClient, cleanup
}

// @cpt-end:cpt-katapult-algo-integration-tests-envtest-setup:p2:inst-resolve-envtest-assets
