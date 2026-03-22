package cli

import (
	"github.com/spf13/cobra"
)

// NewScaffoldCmd returns the top-level scaffold command group.
func NewScaffoldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaffold",
		Short: "Browse, use, and create workflow scaffolds",
	}
	cmd.AddCommand(newScaffoldListCmd())
	cmd.AddCommand(newScaffoldSearchCmd())
	cmd.AddCommand(newScaffoldInfoCmd())
	cmd.AddCommand(newScaffoldInitCmd())
	cmd.AddCommand(newScaffoldExtractCmd())
	cmd.AddCommand(newScaffoldSyncCmd())
	cmd.AddCommand(newScaffoldParamsCmd())
	return cmd
}
