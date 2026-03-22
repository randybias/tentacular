package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/scaffold"
)

func newScaffoldSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search scaffolds by keyword",
		Args:  cobra.ExactArgs(1),
		RunE:  runScaffoldSearch,
	}
	cmd.Flags().String("source", "all", "Filter by source: private, public, or all")
	cmd.Flags().Bool("json", false, "Output as JSON")
	return cmd
}

func runScaffoldSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	source, _ := cmd.Flags().GetString("source")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	cfg := LoadConfig()
	client := scaffold.NewClient(cfg.Scaffold)

	matches, err := scaffold.SearchScaffolds(query, source, client.CachedIndexPath())
	if err != nil {
		return fmt.Errorf("searching scaffolds: %w", err)
	}

	if jsonOutput {
		data, err := json.MarshalIndent(matches, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(matches) == 0 {
		fmt.Printf("No scaffolds matching '%s'.\n", query)
		return nil
	}

	fmt.Printf("%-9s %-28s %-16s %-12s %s\n", "SOURCE", "NAME", "CATEGORY", "COMPLEXITY", "DESCRIPTION")
	for _, s := range matches {
		desc := s.Description
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}
		fmt.Printf("%-9s %-28s %-16s %-12s %s\n", s.Source, s.Name, s.Category, s.Complexity, desc)
	}
	return nil
}
