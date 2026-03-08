package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"sigs.k8s.io/yaml"
)

// @cpt-dod:cpt-katapult-dod-api-cli-cli-tool:p1

// FormatOutput writes data in the specified format.
// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-format-output
func FormatOutput(w io.Writer, format string, data []byte, tableFunc func(io.Writer, []byte) error) error {
	switch format {
	case "json":
		var pretty json.RawMessage
		if err := json.Unmarshal(data, &pretty); err != nil {
			return err
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(pretty)
	case "yaml":
		yamlData, err := yaml.JSONToYAML(data)
		if err != nil {
			return err
		}
		_, err = w.Write(yamlData)
		return err
	case "table":
		if tableFunc != nil {
			return tableFunc(w, data)
		}
		return FormatOutput(w, "json", data, nil)
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}
}

// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-cli:p1:inst-format-output

// NewTabWriter creates a standard tabwriter for table output.
func NewTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}
