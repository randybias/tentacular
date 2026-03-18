package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/catalog"
)

func newCatalogSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search workflow templates",
		Args:  cobra.ExactArgs(1),
		RunE:  runCatalogSearch,
	}
	cmd.Flags().Bool("no-cache", false, "Force re-fetch of catalog index")
	return cmd
}

func runCatalogSearch(cmd *cobra.Command, args []string) error {
	query := strings.ToLower(args[0])
	noCache, _ := cmd.Flags().GetBool("no-cache")

	cfg := LoadConfig()
	client := catalog.NewClient(cfg.Catalog)

	idx, err := client.FetchIndex(noCache)
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	var matches []catalog.TemplateEntry
	for _, t := range idx.Templates {
		if matchesQuery(t, query) {
			matches = append(matches, t)
		}
	}

	if len(matches) == 0 {
		fmt.Printf("No templates matching '%s'.\n", args[0])
		return nil
	}

	fmt.Printf("%-24s %-16s %-12s %s\n", "NAME", "CATEGORY", "COMPLEXITY", "DESCRIPTION")
	for _, t := range matches {
		desc := t.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Printf("%-24s %-16s %-12s %s\n", t.Name, t.Category, t.Complexity, desc)
	}

	return nil
}

func matchesQuery(t catalog.TemplateEntry, query string) bool {
	if strings.Contains(strings.ToLower(t.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(t.DisplayName), query) {
		return true
	}
	if strings.Contains(strings.ToLower(t.Description), query) {
		return true
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
