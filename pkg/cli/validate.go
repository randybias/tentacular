package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
)

func NewValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [dir]",
		Short: "Validate workflow spec",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runValidate,
	}
}

func runValidate(cmd *cobra.Command, args []string) error {
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
		fmt.Fprintf(os.Stderr, "Validation errors in %s:\n", specPath)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		return fmt.Errorf("workflow spec has %d error(s)", len(errs))
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		fmt.Printf("Workflow: %s (v%s)\n", wf.Name, wf.Version)
		fmt.Printf("Nodes:    %d\n", len(wf.Nodes))
		fmt.Printf("Edges:    %d\n", len(wf.Edges))
		fmt.Printf("Triggers: %d\n", len(wf.Triggers))
	}

	fmt.Printf("✓ %s is valid\n", specPath)
	return nil
}
