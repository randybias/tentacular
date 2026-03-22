package cli

import (
	"github.com/spf13/cobra"
)

// newCatalogListCmd is a deprecated wrapper around 'tntc scaffold list'.
func newCatalogListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "list",
		Short:      "List available workflow templates (deprecated: use 'tntc scaffold list')",
		Args:       cobra.NoArgs,
		Deprecated: "use 'tntc scaffold list' instead",
		RunE:       runScaffoldList,
	}
	cmd.Flags().String("category", "", "Filter by category")
	cmd.Flags().String("tag", "", "Filter by tag")
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().Bool("no-cache", false, "Force re-fetch (ignored, use 'tntc scaffold sync')")
	// source defaults to public-only for backwards compatibility with old catalog behavior
	cmd.Flags().String("source", "public", "Filter by source")
	return cmd
}
