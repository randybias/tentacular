package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func NewUndeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "undeploy <name>",
		Short: "Remove a deployed workflow",
		Args:  cobra.ExactArgs(1),
		RunE:  runUndeploy,
	}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func runUndeploy(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace, _ := cmd.Flags().GetString("namespace")
	yes, _ := cmd.Flags().GetBool("yes")

	if !yes {
		fmt.Printf("Remove workflow %s from namespace %s? [y/N] ", name, namespace)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.WfRemove(cmd.Context(), namespace, name)
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("removing workflow: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("removing workflow: %w", err)
	}

	if len(result.Deleted) == 0 {
		fmt.Printf("No resources found for %s in %s\n", name, namespace)
	} else {
		for _, d := range result.Deleted {
			fmt.Printf("  deleted %s\n", d)
		}
	}

	return nil
}
