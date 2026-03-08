//go:build e2e

// @cpt-dod:cpt-katapult-dod-integration-tests-e2e-tests:p2
// @cpt-algo:cpt-katapult-algo-integration-tests-kind-lifecycle:p2
// @cpt-algo:cpt-katapult-algo-integration-tests-data-integrity-check:p2

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/maxitosh/katapult/internal/testutil"
)

var (
	clusterName = "katapult-e2e-test"
	kubeconfig  string
	httpBaseURL string
	k8sClient   kubernetes.Interface
)

// TestMain orchestrates the full e2e lifecycle: cluster creation, deployment,
// NodePort setup, test execution, and teardown.
func TestMain(m *testing.M) {
	code := runSuite(m)
	os.Exit(code)
}

func runSuite(m *testing.M) int {
	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-create-cluster
	fmt.Println("e2e: creating Kind cluster", clusterName)
	out, err := exec.Command("kind", "create", "cluster",
		"--name", clusterName,
		"--config", "../../../deploy/test/kind-config.yaml",
		"--wait", "60s",
	).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: kind create cluster failed: %s\n%s\n", err, out)
		return 1
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-create-cluster

	defer func() {
		// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-delete-cluster
		fmt.Println("e2e: deleting Kind cluster", clusterName)
		del, delErr := exec.Command("kind", "delete", "cluster", "--name", clusterName).CombinedOutput()
		if delErr != nil {
			fmt.Fprintf(os.Stderr, "e2e: kind delete cluster failed: %s\n%s\n", delErr, del)
		}
		// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-delete-cluster
	}()

	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-get-kubeconfig
	kcOut, err := exec.Command("kind", "get", "kubeconfig", "--name", clusterName).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: kind get kubeconfig failed: %s\n", err)
		return 1
	}
	tmpKubeconfig, err := os.CreateTemp("", "katapult-e2e-kubeconfig-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: failed to create temp kubeconfig: %s\n", err)
		return 1
	}
	defer os.Remove(tmpKubeconfig.Name())
	if _, err := tmpKubeconfig.Write(kcOut); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: failed to write kubeconfig: %s\n", err)
		return 1
	}
	tmpKubeconfig.Close()
	kubeconfig = tmpKubeconfig.Name()

	restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: failed to build rest config: %s\n", err)
		return 1
	}
	k8sClient, err = kubernetes.NewForConfig(restCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: failed to create k8s client: %s\n", err)
		return 1
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-get-kubeconfig

	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-load-images
	fmt.Println("e2e: loading images into Kind cluster")
	out, err = exec.Command(
		"kind", "load", "docker-image",
		"katapult-controlplane:test", "katapult-agent:test",
		"--name", clusterName,
	).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: kind load images failed: %s\n%s\n", err, out)
		return 1
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-load-images

	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-deploy
	fmt.Println("e2e: deploying katapult test manifests")
	kustomize := kustomizeBuildCommand("../../../deploy/test")
	apply := exec.Command("kubectl", "apply", "-f", "-", "--kubeconfig", kubeconfig)

	pipe, err := kustomize.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: pipe failed: %s\n", err)
		return 1
	}
	apply.Stdin = pipe

	if err := kustomize.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: kustomize start failed: %s\n", err)
		return 1
	}
	applyOut, err := apply.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: kubectl apply failed: %s\n%s\n", err, applyOut)
		return 1
	}
	if err := kustomize.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: kustomize wait failed: %s\n", err)
		return 1
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-deploy

	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-wait-ready
	fmt.Println("e2e: waiting for deployments to become ready")
	out, err = exec.Command(
		"kubectl", "wait", "--for=condition=available",
		"deployment/katapult-controlplane", "-n", "katapult-system",
		"--timeout=120s", "--kubeconfig", kubeconfig,
	).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: controlplane not ready: %s\n%s\n", err, out)
		return 1
	}

	out, err = exec.Command(
		"kubectl", "rollout", "status",
		"daemonset/katapult-agent", "-n", "katapult-system",
		"--timeout=120s", "--kubeconfig", kubeconfig,
	).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: agent daemonset not ready: %s\n%s\n", err, out)
		return 1
	}
	// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-wait-ready

	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-nodeport
	httpBaseURL = "http://127.0.0.1:30080"
	fmt.Println("e2e: controlplane available at", httpBaseURL)
	// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-nodeport

	return m.Run()
}

// kustomizeBuildCommand prefers standalone kustomize and falls back to
// "kubectl kustomize" when the standalone binary is unavailable.
func kustomizeBuildCommand(path string) *exec.Cmd {
	if _, err := exec.LookPath("kustomize"); err == nil {
		return exec.Command("kustomize", "build", path)
	}
	return exec.Command("kubectl", "kustomize", path)
}

// computePVCChecksum creates a temporary pod that mounts the given PVC and
// computes sha256 checksums of all regular files under /data.
//
// @cpt-begin:cpt-katapult-algo-integration-tests-data-integrity-check:p2:inst-compute-checksum
func computePVCChecksum(t *testing.T, namespace, pvcName string) string {
	t.Helper()

	g := gomega.NewWithT(t)
	podName := fmt.Sprintf("checksum-%s-%d", pvcName, time.Now().UnixNano()%100000)
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "hasher",
				Image:   "busybox:latest",
				Command: []string{"sh", "-c", "find /data -type f | sort | xargs sha256sum"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "vol",
					MountPath: "/data",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "vol",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			}},
		},
	}

	_, err := k8sClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("computePVCChecksum: failed to create pod %s: %s", podName, err)
	}

	// Wait for pod completion using Gomega Eventually.
	g.Eventually(func(g gomega.Gomega) {
		pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(pod.Status.Phase).To(gomega.BeElementOf(corev1.PodSucceeded, corev1.PodFailed))
	}, testutil.DefaultTimeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())

	// Verify it succeeded.
	finalPod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("computePVCChecksum: failed to get pod %s: %s", podName, err)
	}
	if finalPod.Status.Phase == corev1.PodFailed {
		t.Fatalf("computePVCChecksum: pod %s failed", podName)
	}

	// Retrieve logs (the checksum output).
	logStream, err := k8sClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		t.Fatalf("computePVCChecksum: failed to get logs from pod %s: %s", podName, err)
	}
	defer logStream.Close()
	logsOut, err := io.ReadAll(logStream)
	if err != nil {
		t.Fatalf("computePVCChecksum: failed to read logs from pod %s: %s", podName, err)
	}

	// Cleanup the pod.
	_ = k8sClient.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})

	return strings.TrimSpace(string(logsOut))
}

// @cpt-end:cpt-katapult-algo-integration-tests-data-integrity-check:p2:inst-compute-checksum

// waitForTransferComplete polls the transfer status until a terminal state is
// reached or the timeout expires.
func waitForTransferComplete(t *testing.T, transferID string, timeout time.Duration) string {
	t.Helper()

	g := gomega.NewWithT(t)
	var finalState string

	g.Eventually(func() bool {
		resp := httpDo(t, http.MethodGet, "/api/v1alpha1/transfers/"+transferID, nil)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return false
		}

		var result struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return false
		}

		switch result.State {
		case "completed", "failed", "cancelled":
			finalState = result.State
			return true
		}
		return false
	}, timeout, testutil.E2EPollingInterval).Should(gomega.BeTrue(),
		"transfer %s did not reach terminal state within %s", transferID, timeout)

	return finalState
}

// httpDo performs an HTTP request against the controlplane with the test auth
// token and returns the response.
func httpDo(t *testing.T, method, path string, body io.Reader) *http.Response {
	t.Helper()

	url := httpBaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("httpDo: failed to create request: %s", err)
	}
	req.Header.Set("Authorization", "Bearer test-operator-token")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("httpDo: request %s %s failed: %s", method, path, err)
	}
	return resp
}

// kubectlExec runs kubectl with the suite kubeconfig and returns stdout. It
// calls t.Fatalf on any error.
func kubectlExec(t *testing.T, args ...string) string {
	t.Helper()

	fullArgs := append([]string{"--kubeconfig", kubeconfig}, args...)
	out, err := exec.Command("kubectl", fullArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("kubectlExec %v failed: %s\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// createTestPVC creates a PVC in the given namespace.
func createTestPVC(t *testing.T, namespace, name, size string) {
	t.Helper()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}

	_, err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("createTestPVC: failed to create PVC %s: %s", name, err)
	}
}

// waitForPodCompletion polls the pod phase until it reaches Succeeded or Failed,
// or the timeout expires.
func waitForPodCompletion(t *testing.T, namespace, podName string, timeout time.Duration) {
	t.Helper()

	g := gomega.NewWithT(t)
	ctx := context.Background()

	g.Eventually(func(g gomega.Gomega) {
		pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(pod.Status.Phase).To(gomega.BeElementOf(corev1.PodSucceeded, corev1.PodFailed))
	}, timeout, testutil.DefaultPollingInterval).Should(gomega.Succeed())

	// Verify it didn't fail.
	pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("waitForPodCompletion: error getting pod %s: %s", podName, err)
	}
	if pod.Status.Phase == corev1.PodFailed {
		t.Fatalf("waitForPodCompletion: pod %s failed", podName)
	}
}

// populatePVC creates a pod that writes known test data into the given PVC.
func populatePVC(t *testing.T, namespace, pvcName string, data map[string]string) {
	t.Helper()

	podName := fmt.Sprintf("populate-%s-%d", pvcName, time.Now().UnixNano()%100000)

	// Build a shell command that writes each file.
	var cmds []string
	for path, content := range data {
		cmds = append(cmds, fmt.Sprintf("mkdir -p /data/$(dirname %s) && echo -n %q > /data/%s", path, content, path))
	}
	shellCmd := strings.Join(cmds, " && ")

	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "writer",
				Image:   "busybox:latest",
				Command: []string{"sh", "-c", shellCmd},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "vol",
					MountPath: "/data",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "vol",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			}},
		},
	}

	_, err := k8sClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("populatePVC: failed to create writer pod: %s", err)
	}

	waitForPodCompletion(t, namespace, podName, 120*time.Second)

	// Cleanup.
	_ = k8sClient.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// createUniqueNamespace creates a namespace with GenerateName for parallel-safe e2e tests.
func createUniqueNamespace(t *testing.T, prefix string) string {
	t.Helper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix + "-",
		},
	}
	created, err := k8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("createUniqueNamespace: failed to create namespace with prefix %q: %s", prefix, err)
	}
	t.Cleanup(func() {
		_ = k8sClient.CoreV1().Namespaces().Delete(context.Background(), created.Name, metav1.DeleteOptions{})
	})
	return created.Name
}

// createNamespace creates a Kubernetes namespace if it does not exist.
func createNamespace(t *testing.T, namespace string) {
	t.Helper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}
	_, err := k8sClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("createNamespace: failed to create namespace %s: %s", namespace, err)
	}
}

// jsonBody marshals v to a *bytes.Buffer suitable for use as an HTTP request body.
func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("jsonBody: marshal failed: %s", err)
	}
	return bytes.NewBuffer(b)
}
