package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Trigger a deployed workflow",
		Args:  cobra.ExactArgs(1),
		RunE:  runRun,
	}
	cmd.Flags().Duration("timeout", 120*time.Second, "Maximum time to wait for result")
	cmd.Flags().Bool("no-wait", false, "Deprecated: MCP server handles readiness internally")
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace, _ := cmd.Flags().GetString("namespace")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Running workflow %s in %s...\n", name, namespace)

	result, err := mcpClient.WfRun(cmd.Context(), namespace, name, nil, int(timeout.Seconds()))
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("running workflow: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("running workflow: %w", err)
	}

	fmt.Fprint(os.Stdout, string(result.Output))
	return nil
}
