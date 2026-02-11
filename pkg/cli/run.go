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
	cmd.Flags().Duration("timeout", 30*time.Second, "Maximum time to wait for result")
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace, _ := cmd.Flags().GetString("namespace")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "Running workflow %s in %s...\n", name, namespace)

	result, err := client.RunWorkflow(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("running workflow: %w", err)
	}

	fmt.Fprint(os.Stdout, result)
	return nil
}
