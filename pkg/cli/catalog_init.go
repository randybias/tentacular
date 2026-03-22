package cli

import (
	"github.com/spf13/cobra"
)

// newCatalogInitCmd is a deprecated wrapper around 'tntc scaffold init'.
func newCatalogInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "init <template-name> [workflow-name]",
		Short:      "Scaffold a workflow from a catalog template (deprecated: use 'tntc scaffold init')",
		Args:       cobra.RangeArgs(1, 2),
		Deprecated: "use 'tntc scaffold init' instead",
		RunE:       runCatalogInitCompat,
	}
	cmd.Flags().String("namespace", "", "Set deployment.namespace in workflow.yaml")
	cmd.Flags().Bool("no-cache", false, "Force re-fetch (ignored, use 'tntc scaffold sync')")
	return cmd
}

// runCatalogInitCompat adapts the old catalog init argument style (1 or 2 args) to scaffold init (requires 2 args).
func runCatalogInitCompat(cmd *cobra.Command, args []string) error {
	scaffoldName := args[0]
	tentacleName := scaffoldName
	if len(args) > 1 {
		tentacleName = args[1]
	}
	// Delegate to scaffold init with public-only source and optional namespace
	namespace, _ := cmd.Flags().GetString("namespace")
	fakeCmd := newScaffoldInitCmd()
	fakeArgs := []string{scaffoldName, tentacleName}
	if namespace != "" {
		if err := fakeCmd.Flags().Set("namespace", namespace); err != nil {
			return err
		}
	}
	if err := fakeCmd.Flags().Set("source", "public"); err != nil {
		return err
	}
	return fakeCmd.RunE(fakeCmd, fakeArgs)
}
