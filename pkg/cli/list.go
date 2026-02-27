package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List deployed workflows",
		Args:  cobra.NoArgs,
		RunE:  runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	namespace, _ := cmd.Flags().GetString("namespace")
	output, _ := cmd.Flags().GetString("output")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	workflows, err := mcpClient.WfList(cmd.Context(), namespace)
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("listing workflows: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("listing workflows: %w", err)
	}

	if output == "json" {
		data, err := json.MarshalIndent(workflows, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(workflows) == 0 {
		fmt.Printf("No workflows found in namespace %s\n", namespace)
		return nil
	}

	fmt.Printf("%-24s %-8s %-16s %-10s %-10s %s\n", "NAME", "VERSION", "NAMESPACE", "STATUS", "REPLICAS", "AGE")
	for _, w := range workflows {
		status := "not ready"
		if w.Ready {
			status = "ready"
		}
		age := ""
		if w.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, w.CreatedAt); err == nil {
				age = formatAge(time.Since(t))
			}
		}
		fmt.Printf("%-24s %-8s %-16s %-10s %d/%d        %s\n", w.Name, w.Version, w.Namespace, status, w.Available, w.Replicas, age)
	}

	return nil
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
