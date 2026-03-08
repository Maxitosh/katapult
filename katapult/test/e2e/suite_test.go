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
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	clusterName = "katapult-e2e-test"
	kubeconfig  string
	httpBaseURL string
	k8sClient   kubernetes.Interface
)

// TestMain orchestrates the full e2e lifecycle: cluster creation, deployment,
// port-forwarding, test execution, and teardown.
func TestMain(m *testing.M) {
	code := runSuite(m)
	os.Exit(code)
}

func runSuite(m *testing.M) int {
	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-create-cluster
	fmt.Println("e2e: creating Kind cluster", clusterName)
	out, err := exec.Command("kind", "create", "cluster", "--name", clusterName, "--wait", "60s").CombinedOutput()
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

	// @cpt-begin:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-port-forward
	localPort := portForwardSetup(kubeconfig, "katapult-system", "svc/katapult-controlplane", 8080)
	if localPort == 0 {
		fmt.Fprintln(os.Stderr, "e2e: port-forward failed")
		return 1
	}
	httpBaseURL = fmt.Sprintf("http://127.0.0.1:%d", localPort)
	fmt.Println("e2e: controlplane available at", httpBaseURL)
	// @cpt-end:cpt-katapult-algo-integration-tests-kind-lifecycle:p2:inst-port-forward

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

// portForwardSetup starts kubectl port-forward in the background and returns the
// local port. Returns 0 on failure.
func portForwardSetup(kc, namespace, target string, remotePort int) int {
	localPort, err := freePort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: failed to find free port: %s\n", err)
		return 0
	}

	cmd := exec.Command(
		"kubectl", "port-forward", target,
		fmt.Sprintf("%d:%d", localPort, remotePort),
		"-n", namespace, "--kubeconfig", kc,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: port-forward start failed: %s\n", err)
		return 0
	}

	// Wait briefly for port-forward to establish.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, connErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 500*time.Millisecond)
		if connErr == nil {
			conn.Close()
			return localPort
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "e2e: port-forward did not become ready in time")
	return 0
}

// freePort asks the OS for a free TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// computePVCChecksum creates a temporary pod that mounts the given PVC and
// computes sha256 checksums of all regular files under /data.
//
// @cpt-begin:cpt-katapult-algo-integration-tests-data-integrity-check:p2:inst-compute-checksum
func computePVCChecksum(t *testing.T, namespace, pvcName string) string {
	t.Helper()

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

	waitForPodCompletion(t, namespace, podName, 120*time.Second)

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

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := httpDo(t, http.MethodGet, "/api/v1alpha1/transfers/"+transferID, nil)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("waitForTransferComplete: failed to read response: %s", err)
		}

		var result struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("waitForTransferComplete: failed to parse response: %s\n%s", err, body)
		}

		switch result.State {
		case "completed", "failed", "cancelled":
			return result.State
		}

		time.Sleep(3 * time.Second)
	}

	t.Fatalf("waitForTransferComplete: transfer %s did not reach terminal state within %s", transferID, timeout)
	return ""
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

// portForward starts kubectl port-forward in the background and returns the
// allocated local port.
func portForward(t *testing.T, kc, namespace, svcName string, remotePort int) int {
	t.Helper()

	localPort, err := freePort()
	if err != nil {
		t.Fatalf("portForward: failed to find free port: %s", err)
	}

	cmd := exec.Command(
		"kubectl", "port-forward", "svc/"+svcName,
		fmt.Sprintf("%d:%d", localPort, remotePort),
		"-n", namespace, "--kubeconfig", kc,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("portForward: failed to start: %s", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
	})

	// Wait for port-forward to be reachable.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, connErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 500*time.Millisecond)
		if connErr == nil {
			conn.Close()
			return localPort
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("portForward: port-forward to %s/%s did not become ready", namespace, svcName)
	return 0
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
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			t.Logf("waitForPodCompletion: error getting pod %s: %s", podName, err)
			time.Sleep(2 * time.Second)
			continue
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return
		case corev1.PodFailed:
			t.Fatalf("waitForPodCompletion: pod %s failed", podName)
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("waitForPodCompletion: pod %s did not complete within %s", podName, timeout)
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
