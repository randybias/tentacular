package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Check deployment status",
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
	cmd.Flags().Bool("detail", false, "Show detailed status including pods, events, and resource limits")
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace, _ := cmd.Flags().GetString("namespace")
	output, _ := cmd.Flags().GetString("output")
	detail, _ := cmd.Flags().GetBool("detail")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	status, err := mcpClient.WfStatus(cmd.Context(), namespace, name, detail)
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("getting status: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("getting status: %w", err)
	}

	if output == "json" {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Text output
	readyStr := "not ready"
	if status.Ready {
		readyStr = "ready"
	}
	fmt.Printf("Name:      %s\n", status.Name)
	fmt.Printf("Namespace: %s\n", status.Namespace)
	if status.Version != "" {
		fmt.Printf("Version:   %s\n", status.Version)
	}
	fmt.Printf("Status:    %s\n", readyStr)
	fmt.Printf("Replicas:  %d/%d\n", status.Available, status.Replicas)

	if detail && len(status.Pods) > 0 {
		fmt.Println("\nPods:")
		for _, pod := range status.Pods {
			podReady := "not ready"
			if pod.Ready {
				podReady = "ready"
			}
			fmt.Printf("  %-40s %-12s %s\n", pod.Name, pod.Phase, podReady)
		}
	}

	if detail && len(status.Events) > 0 {
		fmt.Println("\nEvents:")
		for _, evt := range status.Events {
			fmt.Printf("  [%s] %s: %s (x%d)\n", evt.Type, evt.Reason, evt.Message, evt.Count)
		}
	}

	return nil
}
