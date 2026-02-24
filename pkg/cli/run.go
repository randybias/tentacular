package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Trigger a deployed workflow",
		Args:  cobra.ExactArgs(1),
		RunE:  runRun,
	}
	cmd.Flags().Duration("timeout", 120*time.Second, "Maximum time to wait for readiness + result")
	cmd.Flags().Bool("no-wait", false, "Skip readiness check and run immediately")
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace, _ := cmd.Flags().GetString("namespace")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	noWait, _ := cmd.Flags().GetBool("no-wait")

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait for the deployment to be ready before triggering.
	// Without this, the runner curl pod fires immediately; if the workflow pod
	// hasn't passed its readiness probe yet the Service has no endpoints and
	// the run fails with a connection-refused error (the port-forward race).
	if !noWait {
		fmt.Fprintf(os.Stderr, "Waiting for %s to be ready...\n", name)
		if err := client.WaitForReady(ctx, namespace, name); err != nil {
			return fmt.Errorf("workflow not ready: %w (use --no-wait to skip readiness check)", err)
		}
	}

	fmt.Fprintf(os.Stderr, "Running workflow %s in %s...\n", name, namespace)

	result, err := client.RunWorkflow(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("running workflow: %w", err)
	}

	fmt.Fprint(os.Stdout, result)
	return nil
}
