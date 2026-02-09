package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randyb/pipedreamer2/pkg/spec"
	"github.com/spf13/cobra"
)

func NewVisualizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "visualize [dir]",
		Short: "Generate Mermaid diagram",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runVisualize,
	}
}

func runVisualize(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	specPath := filepath.Join(dir, "workflow.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", specPath, err)
	}

	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		return fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
	}

	fmt.Println("```mermaid")
	fmt.Println("graph TD")
	for name := range wf.Nodes {
		fmt.Printf("    %s[%s]\n", name, name)
	}
	for _, edge := range wf.Edges {
		fmt.Printf("    %s --> %s\n", edge.From, edge.To)
	}
	fmt.Println("```")

	return nil
}
