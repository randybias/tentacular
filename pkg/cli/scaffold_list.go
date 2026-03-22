package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/scaffold"
)

func newScaffoldListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available scaffolds (private and public)",
		Args:  cobra.NoArgs,
		RunE:  runScaffoldList,
	}
	cmd.Flags().String("source", "all", "Filter by source: private, public, or all")
	cmd.Flags().String("category", "", "Filter by category")
	cmd.Flags().String("tag", "", "Filter by tag")
	cmd.Flags().Bool("json", false, "Output as JSON")
	return cmd
}

func runScaffoldList(cmd *cobra.Command, _ []string) error {
	source, _ := cmd.Flags().GetString("source")
	category, _ := cmd.Flags().GetString("category")
	tag, _ := cmd.Flags().GetString("tag")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	cfg := LoadConfig()
	client := scaffold.NewClient(cfg.Scaffold)

	scaffolds, err := scaffold.ListScaffolds(source, category, tag, client.CachedIndexPath())
	if err != nil {
		return fmt.Errorf("listing scaffolds: %w", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(scaffolds, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(scaffolds) == 0 {
		fmt.Println("No scaffolds found.")
		fmt.Println("Run 'tntc scaffold sync' to fetch the public quickstarts index.")
		return nil
	}

	fmt.Printf("%-9s %-28s %-16s %-12s %s\n", "SOURCE", "NAME", "CATEGORY", "COMPLEXITY", "DESCRIPTION")
	for _, s := range scaffolds {
		desc := s.Description
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}
		fmt.Printf("%-9s %-28s %-16s %-12s %s\n", s.Source, s.Name, s.Category, s.Complexity, desc)
	}
	return nil
}
