package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// @cpt-dod:cpt-katapult-dod-api-cli-cli-tool:p1
// @cpt-algo:cpt-katapult-algo-api-cli-resolve-cli-command:p1

var (
	serverFlag     string
	tokenFlag      string
	outputFlag     string
	kubeconfigFlag string
)

func newRootCmd() *cobra.Command {
	// @cpt-begin:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-parse-command
	cmd := &cobra.Command{
		Use:   "katapult",
		Short: "Katapult CLI — manage PVC transfers across Kubernetes clusters",
		SilenceUsage: true,
	}
	// @cpt-end:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-parse-command

	// @cpt-begin:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-parse-flags
	cmd.PersistentFlags().StringVar(&serverFlag, "server", "", "API server address (overrides KATAPULT_SERVER)")
	cmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "Bearer token (overrides KATAPULT_TOKEN)")
	cmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "table", "Output format: table, json, yaml")
	cmd.PersistentFlags().StringVar(&kubeconfigFlag, "kubeconfig", "", "Path to kubeconfig file")
	// @cpt-end:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-parse-flags

	cmd.AddCommand(NewTransferCommand())
	cmd.AddCommand(NewAgentCommand())

	return cmd
}

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

// resolveServerAndToken resolves the API server address and token from flags/env.
// @cpt-begin:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-resolve-address
// @cpt-begin:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-resolve-auth
func resolveServerAndToken() (string, string, error) {
	server := serverFlag
	if server == "" {
		server = os.Getenv("KATAPULT_SERVER")
	}
	if server == "" {
		server = resolveServerFromKubeconfig(kubeconfigFlag)
	}
	if server == "" {
		return "", "", fmt.Errorf("API server address not set; use --server, KATAPULT_SERVER, or kubeconfig")
	}

	token := tokenFlag
	if token == "" {
		token = os.Getenv("KATAPULT_TOKEN")
	}
	if token == "" {
		token = resolveTokenFromKubeconfig(kubeconfigFlag)
	}

	return server, token, nil
}

// @cpt-end:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-resolve-auth
// @cpt-end:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-resolve-address
