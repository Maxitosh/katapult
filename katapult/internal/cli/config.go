package cli

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
)

// @cpt-dod:cpt-katapult-dod-api-cli-cli-tool:p1

func resolveServerFromKubeconfig(kubeconfigPath string) string {
	if kubeconfigPath == "" {
		kubeconfigPath = defaultKubeconfigPath()
	}
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil)
	cc, err := config.ClientConfig()
	if err != nil {
		return ""
	}
	return cc.Host
}

func resolveTokenFromKubeconfig(kubeconfigPath string) string {
	if kubeconfigPath == "" {
		kubeconfigPath = defaultKubeconfigPath()
	}
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil)
	cc, err := config.ClientConfig()
	if err != nil {
		return ""
	}
	return cc.BearerToken
}

func defaultKubeconfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}
