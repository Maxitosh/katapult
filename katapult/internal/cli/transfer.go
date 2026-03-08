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
// @cpt-flow:cpt-katapult-flow-api-cli-create-transfer-cli:p1

// NewTransferCommand creates the transfer parent command and its subcommands.
func NewTransferCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Manage PVC transfers",
	}

	cmd.AddCommand(newTransferCreateCmd())
	cmd.AddCommand(newTransferListCmd())
	cmd.AddCommand(newTransferGetCmd())
	cmd.AddCommand(newTransferCancelCmd())

	return cmd
}

func newTransferCreateCmd() *cobra.Command {
	var (
		sourceCluster string
		sourcePVC     string
		destCluster   string
		destPVC       string
		strategy      string
		allowOverwrite bool
	)

	// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-cli-invoke
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new transfer",
		RunE: func(cmd *cobra.Command, args []string) error {
			// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-resolve-server
			server, token, err := resolveServerAndToken()
			if err != nil {
				return err
			}
			// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-resolve-server

			// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-construct-request
			client := NewAPIClient(server, token)
			input := map[string]any{
				"source_cluster":      sourceCluster,
				"source_pvc":          sourcePVC,
				"destination_cluster": destCluster,
				"destination_pvc":     destPVC,
				"allow_overwrite":     allowOverwrite,
			}
			if strategy != "" {
				input["strategy"] = strategy
			}
			// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-construct-request

			// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-send-request
			data, status, err := client.CreateTransfer(input)
			if err != nil {
				return err
			}
			// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-send-request

			// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-handle-error
			if status >= 400 {
				return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
			}
			// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-handle-error

			return FormatOutput(os.Stdout, outputFlag, data, formatTransferDetail)
		},
	}
	// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-cli-invoke

	cmd.Flags().StringVar(&sourceCluster, "source-cluster", "", "Source cluster ID (required)")
	cmd.Flags().StringVar(&sourcePVC, "source-pvc", "", "Source PVC reference (required)")
	cmd.Flags().StringVar(&destCluster, "dest-cluster", "", "Destination cluster ID (required)")
	cmd.Flags().StringVar(&destPVC, "dest-pvc", "", "Destination PVC reference (required)")
	cmd.Flags().StringVar(&strategy, "strategy", "", "Transfer strategy: stream, s3, direct")
	cmd.Flags().BoolVar(&allowOverwrite, "allow-overwrite", false, "Allow overwriting destination PVC data")

	_ = cmd.MarkFlagRequired("source-cluster")
	_ = cmd.MarkFlagRequired("source-pvc")
	_ = cmd.MarkFlagRequired("dest-cluster")
	_ = cmd.MarkFlagRequired("dest-pvc")

	return cmd
}

func newTransferListCmd() *cobra.Command {
	var (
		statusFilter  string
		clusterFilter string
		limit         int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transfers",
		RunE: func(cmd *cobra.Command, args []string) error {
			server, token, err := resolveServerAndToken()
			if err != nil {
				return err
			}

			params := url.Values{}
			if statusFilter != "" {
				params.Set("status", statusFilter)
			}
			if clusterFilter != "" {
				params.Set("cluster", clusterFilter)
			}
			if limit > 0 {
				params.Set("limit", fmt.Sprintf("%d", limit))
			}

			client := NewAPIClient(server, token)
			data, status, err := client.ListTransfers(params.Encode())
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
			}

			return FormatOutput(os.Stdout, outputFlag, data, formatTransferList)
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by transfer status")
	cmd.Flags().StringVar(&clusterFilter, "cluster", "", "Filter by cluster")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results")

	return cmd
}

func newTransferGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <transfer-id>",
		Short: "Get transfer details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, token, err := resolveServerAndToken()
			if err != nil {
				return err
			}

			client := NewAPIClient(server, token)
			data, status, err := client.GetTransfer(args[0])
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
			}

			return FormatOutput(os.Stdout, outputFlag, data, formatTransferDetail)
		},
	}
}

func newTransferCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <transfer-id>",
		Short: "Cancel an active transfer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			server, token, err := resolveServerAndToken()
			if err != nil {
				return err
			}

			client := NewAPIClient(server, token)
			data, status, err := client.CancelTransfer(args[0])
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("API error (HTTP %d): %s", status, string(data))
			}

			return FormatOutput(os.Stdout, outputFlag, data, formatTransferDetail)
		},
	}
}

func formatTransferList(w io.Writer, data []byte) error {
	var resp struct {
		Items []struct {
			ID                 string `json:"id"`
			State              string `json:"state"`
			SourceCluster      string `json:"source_cluster"`
			DestinationCluster string `json:"destination_cluster"`
			CreatedAt          string `json:"created_at"`
		} `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}

	tw := NewTabWriter(w)
	fmt.Fprintln(tw, "ID\tSTATE\tSOURCE\tDESTINATION\tCREATED")
	for _, t := range resp.Items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", t.ID, t.State, t.SourceCluster, t.DestinationCluster, t.CreatedAt)
	}
	_ = tw.Flush()
	fmt.Fprintf(w, "\nTotal: %d\n", resp.Pagination.Total)
	return nil
}

func formatTransferDetail(w io.Writer, data []byte) error {
	var t map[string]any
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}

	tw := NewTabWriter(w)
	for _, key := range []string{"id", "state", "source_cluster", "source_pvc", "destination_cluster", "destination_pvc", "strategy", "created_at"} {
		if v, ok := t[key]; ok {
			fmt.Fprintf(tw, "%s:\t%v\n", key, v)
		}
	}
	return tw.Flush()
}
