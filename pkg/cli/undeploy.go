package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/randybias/tentacular/pkg/k8s"
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

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	deleted, err := client.DeleteResources(namespace, name)
	if err != nil {
		return fmt.Errorf("deleting resources: %w", err)
	}

	if len(deleted) == 0 {
		fmt.Printf("No resources found for %s in %s\n", name, namespace)
	} else {
		for _, d := range deleted {
			fmt.Printf("  deleted %s\n", d)
		}
	}

	return nil
}
