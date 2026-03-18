package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/catalog"
)

func newCatalogListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available workflow templates",
		Args:  cobra.NoArgs,
		RunE:  runCatalogList,
	}
	cmd.Flags().String("category", "", "Filter by category")
	cmd.Flags().String("tag", "", "Filter by tag")
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().Bool("no-cache", false, "Force re-fetch of catalog index")
	return cmd
}

func runCatalogList(cmd *cobra.Command, _ []string) error {
	categoryFilter, _ := cmd.Flags().GetString("category")
	tagFilter, _ := cmd.Flags().GetString("tag")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	noCache, _ := cmd.Flags().GetBool("no-cache")

	cfg := LoadConfig()
	client := catalog.NewClient(cfg.Catalog)

	idx, err := client.FetchIndex(noCache)
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	templates := filterTemplates(idx.Templates, categoryFilter, tagFilter)

	if jsonOutput {
		data, err := json.MarshalIndent(templates, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(templates) == 0 {
		fmt.Println("No templates found.")
		return nil
	}

	fmt.Printf("%-24s %-16s %-12s %s\n", "NAME", "CATEGORY", "COMPLEXITY", "DESCRIPTION")
	for _, t := range templates {
		desc := t.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Printf("%-24s %-16s %-12s %s\n", t.Name, t.Category, t.Complexity, desc)
	}

	return nil
}

func filterTemplates(templates []catalog.TemplateEntry, category, tag string) []catalog.TemplateEntry {
	if category == "" && tag == "" {
		return templates
	}

	var result []catalog.TemplateEntry
	for _, t := range templates {
		if category != "" && !strings.EqualFold(t.Category, category) {
			continue
		}
		if tag != "" && !hasTag(t.Tags, tag) {
			continue
		}
		result = append(result, t)
	}
	return result
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}
