package cli

import (
	"github.com/spf13/cobra"
)

// newCatalogInfoCmd is a deprecated wrapper around 'tntc scaffold info'.
func newCatalogInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "info <template-name>",
		Short:      "Show details for a workflow template (deprecated: use 'tntc scaffold info')",
		Args:       cobra.ExactArgs(1),
		Deprecated: "use 'tntc scaffold info' instead",
		RunE:       runScaffoldInfo,
	}
	cmd.Flags().Bool("no-cache", false, "Force re-fetch (ignored)")
	cmd.Flags().String("source", "public", "Filter by source")
	return cmd
}
