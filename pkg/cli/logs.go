package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "View workflow pod logs",
		Args:  cobra.ExactArgs(1),
		RunE:  runLogs,
	}
	cmd.Flags().Int64("tail", 100, "Number of recent log lines to show")
	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace := resolveNamespace(cmd, ".")
	tailLines, _ := cmd.Flags().GetInt64("tail")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	// Resolve pod name from workflow name via wf_pods
	pods, err := mcpClient.WfPods(cmd.Context(), namespace)
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	var podName string
	for _, p := range pods.Pods {
		if strings.HasPrefix(p.Name, name+"-") {
			podName = p.Name
			break
		}
	}
	if podName == "" {
		return fmt.Errorf("no pod found for %s in namespace %s", name, namespace)
	}

	result, err := mcpClient.WfLogs(cmd.Context(), namespace, podName, tailLines)
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("getting logs: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("getting logs: %w", err)
	}

	fmt.Println(result.LogText())
	return nil
}
