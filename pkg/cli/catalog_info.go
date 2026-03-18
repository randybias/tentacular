package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/catalog"
)

func newCatalogInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <template-name>",
		Short: "Show details for a workflow template",
		Args:  cobra.ExactArgs(1),
		RunE:  runCatalogInfo,
	}
	cmd.Flags().Bool("no-cache", false, "Force re-fetch of catalog index")
	return cmd
}

func runCatalogInfo(cmd *cobra.Command, args []string) error {
	name := args[0]
	noCache, _ := cmd.Flags().GetBool("no-cache")

	cfg := LoadConfig()
	client := catalog.NewClient(cfg.Catalog)

	idx, err := client.FetchIndex(noCache)
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	tmpl, err := findTemplate(idx.Templates, name)
	if err != nil {
		return err
	}

	fmt.Printf("Name:                 %s\n", tmpl.Name)
	fmt.Printf("Display Name:         %s\n", tmpl.DisplayName)
	fmt.Printf("Description:          %s\n", tmpl.Description)
	fmt.Printf("Category:             %s\n", tmpl.Category)
	fmt.Printf("Complexity:           %s\n", tmpl.Complexity)
	fmt.Printf("Author:               %s\n", tmpl.Author)
	fmt.Printf("Tags:                 %s\n", strings.Join(tmpl.Tags, ", "))
	fmt.Printf("Min tntc Version:     %s\n", tmpl.MinTentacularVersion)
	fmt.Printf("Files:\n")
	for _, f := range tmpl.Files {
		fmt.Printf("  %s\n", f)
	}
	fmt.Printf("\nTo use this template:\n")
	fmt.Printf("  tntc catalog init %s [your-workflow-name]\n", tmpl.Name)

	return nil
}

func findTemplate(templates []catalog.TemplateEntry, name string) (*catalog.TemplateEntry, error) {
	for i := range templates {
		if templates[i].Name == name {
			return &templates[i], nil
		}
	}
	return nil, fmt.Errorf("template '%s' not found in catalog", name)
}
