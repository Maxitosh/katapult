package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

// @cpt-dod:cpt-katapult-dod-api-cli-cli-tool:p1

// NewAgentCommand creates the agent parent command and its subcommands.
func NewAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
	}

	cmd.AddCommand(newAgentListCmd())
	cmd.AddCommand(newAgentGetCmd())

	return cmd
}

func newAgentListCmd() *cobra.Command {
	var (
		clusterFilter string
		stateFilter   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			server, token, err := resolveServerAndToken()
			if err != nil {
				return err
			}

			params := url.Values{}
			if clusterFilter != "" {
				params.Set("cluster", clusterFilter)
			}
			if stateFilter != "" {
				params.Set("state", stateFilter)
			}

			client := NewAPIClient(server, token)
			data, status, err := client.ListAgents(params.Encode())
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
			}

			return FormatOutput(os.Stdout, outputFlag, data, formatAgentList)
		},
	}

	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "Filter by cluster ID")
	cmd.Flags().StringVar(&stateFilter, "state", "", "Filter by agent state")

	return cmd
}

func newAgentGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <agent-id>",
		Short: "Get agent details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, token, err := resolveServerAndToken()
			if err != nil {
				return err
			}

			client := NewAPIClient(server, token)
			data, status, err := client.GetAgent(args[0])
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
			}

			return FormatOutput(os.Stdout, outputFlag, data, formatAgentDetail)
		},
	}
}

func formatAgentList(w io.Writer, data []byte) error {
	var resp struct {
		Items []struct {
			ID            string `json:"id"`
			ClusterID     string `json:"cluster_id"`
			NodeName      string `json:"node_name"`
			State         string `json:"state"`
			LastHeartbeat string `json:"last_heartbeat"`
			PVCs          []any  `json:"pvcs"`
		} `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	tw := NewTabWriter(w)
	fmt.Fprintln(tw, "ID\tCLUSTER\tNODE\tSTATE\tPVCS\tLAST HEARTBEAT")
	for _, a := range resp.Items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n", a.ID, a.ClusterID, a.NodeName, a.State, len(a.PVCs), a.LastHeartbeat)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\nTotal: %d\n", resp.Pagination.Total)
	return nil
}

func formatAgentDetail(w io.Writer, data []byte) error {
	var a map[string]any
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	tw := NewTabWriter(w)
	for _, key := range []string{"id", "cluster_id", "node_name", "state", "healthy", "last_heartbeat", "registered_at"} {
		if v, ok := a[key]; ok {
			fmt.Fprintf(tw, "%s:\t%v\n", key, v)
		}
	}
	if pvcs, ok := a["pvcs"].([]any); ok {
		fmt.Fprintf(tw, "pvcs:\t%d\n", len(pvcs))
	}
	return tw.Flush()
}
