package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/scaffold"
)

func newScaffoldSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Refresh the public quickstarts index from the remote repo",
		Args:  cobra.NoArgs,
		RunE:  runScaffoldSync,
	}
}

func runScaffoldSync(_ *cobra.Command, _ []string) error {
	cfg := LoadConfig()
	client := scaffold.NewClient(cfg.Scaffold)

	fmt.Println("Fetching scaffolds index...")
	idx, err := client.Sync()
	if err != nil {
		return fmt.Errorf("syncing scaffolds: %w", err)
	}

	fmt.Printf("Updated scaffolds index: %d public scaffolds available.\n", len(idx.Scaffolds))
	fmt.Printf("Run 'tntc scaffold list' to browse them.\n")
	return nil
}
