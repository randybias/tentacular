package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/params"
	"github.com/randybias/tentacular/pkg/scaffold"
)

func newScaffoldInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <scaffold-name>",
		Short: "Show scaffold details and parameters",
		Args:  cobra.ExactArgs(1),
		RunE:  runScaffoldInfo,
	}
	cmd.Flags().String("source", "", "Disambiguate source: private or public")
	return cmd
}

func runScaffoldInfo(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := scaffold.ValidateScaffoldName(name); err != nil {
		return fmt.Errorf("invalid scaffold name: %w", err)
	}
	source, _ := cmd.Flags().GetString("source")

	cfg := LoadConfig()
	client := scaffold.NewClient(cfg.Scaffold)

	entry, err := scaffold.FindScaffold(name, source, client.CachedIndexPath())
	if err != nil {
		return err
	}

	fmt.Printf("Name:          %s\n", entry.Name)
	fmt.Printf("Display Name:  %s\n", entry.DisplayName)
	fmt.Printf("Source:        %s\n", entry.Source)
	fmt.Printf("Category:      %s\n", entry.Category)
	fmt.Printf("Complexity:    %s\n", entry.Complexity)
	fmt.Printf("Version:       %s\n", entry.Version)
	if entry.Author != "" {
		fmt.Printf("Author:        %s\n", entry.Author)
	}
	fmt.Printf("Description:   %s\n", entry.Description)
	if len(entry.Tags) > 0 {
		fmt.Printf("Tags:          %s\n", strings.Join(entry.Tags, ", "))
	}

	// Show params.schema.yaml summary if it exists
	schemaPath := scaffoldSchemaPath(entry)
	if schemaPath != "" {
		if schema, err := params.LoadSchema(schemaPath); err == nil {
			printParamsSummary(schema)
		}
	}

	// List files
	if len(entry.Files) > 0 {
		fmt.Printf("\nFiles:\n  %s\n", strings.Join(entry.Files, ", "))
	} else if entry.Path != "" {
		// Private scaffold: list actual files
		printScaffoldFiles(entry.Path)
	}

	fmt.Printf("\nUse: tntc scaffold init %s <tentacle-name>\n", entry.Name)
	return nil
}

// scaffoldSchemaPath returns the path to params.schema.yaml for a scaffold entry, or "".
func scaffoldSchemaPath(entry *scaffold.ScaffoldEntry) string {
	if entry.Path == "" {
		return ""
	}
	p := filepath.Join(entry.Path, "params.schema.yaml")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func printParamsSummary(schema *params.Schema) {
	if len(schema.Parameters) == 0 {
		return
	}
	fmt.Printf("\nParameters:\n")
	for name, def := range schema.Parameters {
		reqStr := "optional"
		if def.Required {
			reqStr = "required"
		}
		typeStr := def.Type
		if def.Default != nil {
			typeStr = fmt.Sprintf("%s, optional, default: %v", def.Type, def.Default)
		}
		fmt.Printf("  %s (%s, %s):\n    %s\n", name, typeStr, reqStr, def.Description)
	}
}

func printScaffoldFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	if len(files) > 0 {
		fmt.Printf("\nFiles:\n  %s\n", strings.Join(files, ", "))
	}
}
