package main

import (
	"fmt"
	"os"

	"github.com/randybias/tentacular/pkg/cli"
	"github.com/randybias/tentacular/pkg/version"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "tntc",
		Short: "Durable workflow execution engine",
		Long:  "Tentacular -- build, test, and deploy TypeScript workflow DAGs on Kubernetes with Deno + gVisor.",
	}

	// Global flags
	root.PersistentFlags().StringP("namespace", "n", "default", "Kubernetes namespace")
	root.PersistentFlags().StringP("registry", "r", "", "Container registry URL")
	root.PersistentFlags().StringP("output", "o", "text", "Output format: text|json")

	// Workflow commands
	root.AddCommand(cli.NewInitCmd())
	root.AddCommand(cli.NewValidateCmd())
	root.AddCommand(cli.NewDevCmd())
	root.AddCommand(cli.NewTestCmd())
	root.AddCommand(cli.NewBuildCmd())
	root.AddCommand(cli.NewDeployCmd())
	root.AddCommand(cli.NewStatusCmd())

	// Operations commands
	root.AddCommand(cli.NewRunCmd())
	root.AddCommand(cli.NewLogsCmd())
	root.AddCommand(cli.NewListCmd())
	root.AddCommand(cli.NewUndeployCmd())

	// Cluster commands
	root.AddCommand(cli.NewClusterCmd())

	// Configuration commands
	root.AddCommand(cli.NewConfigureCmd())
	root.AddCommand(cli.NewSecretsCmd())

	// Utility commands
	root.AddCommand(cli.NewVisualizeCmd())
	root.AddCommand(cli.NewAuditCommand())

	// Version
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print tntc version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.String())
		},
	})

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
