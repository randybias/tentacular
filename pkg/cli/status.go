package cli

import (
	"fmt"

	"github.com/randyb/pipedreamer2/pkg/k8s"
	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Check deployment status",
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	name := args[0]
	namespace, _ := cmd.Flags().GetString("namespace")

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	status, err := client.GetStatus(namespace, name)
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		fmt.Println(status.JSON())
	} else {
		fmt.Println(status.Text())
	}

	return nil
}
