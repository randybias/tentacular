package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewCatalogCmd returns the top-level catalog command group.
// Deprecated: use 'tntc scaffold' instead.
func NewCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "catalog",
		Short:      "Browse and use workflow templates (deprecated: use 'tntc scaffold')",
		Deprecated: "use 'tntc scaffold' instead",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			fmt.Fprintln(os.Stderr, "Warning: 'tntc catalog' is deprecated. Use 'tntc scaffold' instead.")
		},
	}
	cmd.AddCommand(newCatalogListCmd())
	cmd.AddCommand(newCatalogSearchCmd())
	cmd.AddCommand(newCatalogInfoCmd())
	cmd.AddCommand(newCatalogInitCmd())
	return cmd
}
