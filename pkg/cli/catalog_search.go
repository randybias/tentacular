package cli

import (
	"github.com/spf13/cobra"
)

// newCatalogSearchCmd is a deprecated wrapper around 'tntc scaffold search'.
func newCatalogSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "search <query>",
		Short:      "Search workflow templates (deprecated: use 'tntc scaffold search')",
		Args:       cobra.ExactArgs(1),
		Deprecated: "use 'tntc scaffold search' instead",
		RunE:       runScaffoldSearch,
	}
	cmd.Flags().Bool("no-cache", false, "Force re-fetch (ignored, use 'tntc scaffold sync')")
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().String("source", "public", "Filter by source")
	return cmd
}
