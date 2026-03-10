package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/mcp"
)

func NewUndeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "undeploy <name>",
		Short: "Remove a deployed workflow",
		Args:  cobra.ExactArgs(1),
		RunE:  runUndeploy,
	}
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().Bool("force", false, "Skip exoskeleton cleanup confirmation")
	return cmd
}

func runUndeploy(cmd *cobra.Command, args []string) error {
	return runUndeployWith(cmd, args, os.Stdin)
}

// runUndeployWith is the testable core of undeploy, accepting a reader for stdin.
func runUndeployWith(cmd *cobra.Command, args []string, stdin io.Reader) error {
	name := args[0]
	namespace := resolveNamespace(cmd, ".")
	yes, _ := cmd.Flags().GetBool("yes")
	force, _ := cmd.Flags().GetBool("force")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	// Check exoskeleton status before confirming removal.
	exoWarning := checkExoskeletonCleanup(cmd, mcpClient, namespace, name)

	if !yes {
		fmt.Printf("Remove workflow %s from namespace %s? [y/N] ", name, namespace)
		reader := bufio.NewReader(stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// If exoskeleton cleanup is pending and --force was not given, show warning and confirm.
	if exoWarning != "" && !force {
		fmt.Print(exoWarning)
		fmt.Print("This action cannot be undone. Proceed? [y/N] ")
		reader := bufio.NewReader(stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	result, err := mcpClient.WfRemove(cmd.Context(), namespace, name)
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("removing workflow: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("removing workflow: %w", err)
	}

	if len(result.Deleted) > 0 {
		for _, d := range result.Deleted {
			fmt.Printf("  deleted %s\n", d)
		}
	} else if result.DeletedCount > 0 {
		fmt.Printf("Removed %d resource(s) for %s in %s\n", result.DeletedCount, name, namespace)
	} else {
		fmt.Printf("No resources found for %s in %s\n", name, namespace)
	}

	// Show exoskeleton cleanup results if applicable.
	if result.ExoCleanedUp {
		fmt.Printf("Exoskeleton cleanup: %s\n", result.ExoCleanupDetails)
	}

	return nil
}

// checkExoskeletonCleanup queries the MCP server for exoskeleton status and
// registration. If cleanup_on_undeploy is enabled and the workflow has an
// exoskeleton registration, it returns a warning string to display. Otherwise
// it returns empty string. Errors are silently ignored (best-effort check).
func checkExoskeletonCleanup(cmd *cobra.Command, mcpClient *mcp.Client, namespace, name string) string {
	ctx := cmd.Context()

	exoStatus, err := mcpClient.ExoStatus(ctx)
	if err != nil || !exoStatus.Enabled || !exoStatus.CleanupOnUndeploy {
		return ""
	}

	exoReg, err := mcpClient.ExoRegistration(ctx, namespace, name)
	if err != nil || !exoReg.Found {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nWARNING: Exoskeleton cleanup is enabled. Undeploying will permanently delete:\n")
	if exoStatus.PostgresAvailable {
		sb.WriteString("  - Postgres schema and role for this workflow\n")
	}
	if exoStatus.RustFSAvailable {
		sb.WriteString("  - RustFS objects, IAM user, and access policy\n")
	}
	if exoStatus.NATSAvailable {
		sb.WriteString("  - NATS authorization entries and credentials\n")
	}
	sb.WriteString("\n")
	return sb.String()
}
