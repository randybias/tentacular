package cli

import (
	"fmt"

	"github.com/randybias/tentacular/pkg/k8s"
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

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	if detail {
		ds, err := client.GetDetailedStatus(namespace, name)
		if err != nil {
			return fmt.Errorf("getting detailed status: %w", err)
		}
		if output == "json" {
			fmt.Println(ds.JSON())
		} else {
			fmt.Print(ds.Text())
		}
		return nil
	}

	status, err := client.GetStatus(namespace, name)
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	if output == "json" {
		fmt.Println(status.JSON())
	} else {
		fmt.Println(status.Text())
	}

	return nil
}
