package cli

import (
	"fmt"

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
	namespace, _ := cmd.Flags().GetString("namespace")
	tailLines, _ := cmd.Flags().GetInt64("tail")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.WfLogs(cmd.Context(), namespace, name, tailLines)
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("getting logs: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("getting logs: %w", err)
	}

	fmt.Print(result.Logs)
	return nil
}
