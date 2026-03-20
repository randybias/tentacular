package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/cli"
	"github.com/randybias/tentacular/pkg/version"
)

func main() {
	root := &cobra.Command{
		Use:   "tntc",
		Short: "Durable workflow execution engine",
		Long:  "Tentacular -- build, test, and deploy TypeScript workflow DAGs on Kubernetes with Deno + gVisor.",
	}

	// Global flags
	root.PersistentFlags().StringP("registry", "r", "", "Container registry URL")
	root.PersistentFlags().StringP("output", "o", "text", "Output format: text|json")
	root.PersistentFlags().StringP("env", "e", "", "Target environment (overrides TENTACULAR_ENV and default_env)")

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
	root.AddCommand(cli.NewInitWorkspaceCmd())

	// Utility commands
	root.AddCommand(cli.NewVisualizeCmd())
	root.AddCommand(cli.NewAuditCommand())

	// Catalog commands
	root.AddCommand(cli.NewCatalogCmd())

	// Auth commands
	root.AddCommand(cli.NewLoginCmd())
	root.AddCommand(cli.NewLogoutCmd())
	root.AddCommand(cli.NewWhoamiCmd())

	// Permissions commands
	root.AddCommand(cli.NewPermissionsCmd())

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
