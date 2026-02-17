package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
)

func NewVisualizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "visualize [dir]",
		Short: "Generate Mermaid diagram",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runVisualize,
	}
	cmd.Flags().Bool("rich", false, "Include contract dependencies in visualization")
	return cmd
}

func runVisualize(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	rich, _ := cmd.Flags().GetBool("rich")

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

	// Render workflow nodes
	for name := range wf.Nodes {
		fmt.Printf("    %s[%s]\n", name, name)
	}

	// Render workflow edges
	for _, edge := range wf.Edges {
		fmt.Printf("    %s --> %s\n", edge.From, edge.To)
	}

	// Render dependencies if --rich flag is set
	if rich && wf.Contract != nil && len(wf.Contract.Dependencies) > 0 {
		fmt.Println()
		fmt.Println("    %% External Dependencies")
		for name, dep := range wf.Contract.Dependencies {
			// Style dependency nodes differently
			fmt.Printf("    dep_%s[(%s<br/>%s:%d)]\n", name, name, dep.Host, getPortWithDefault(dep))
			fmt.Printf("    style dep_%s fill:#e1f5ff,stroke:#0066cc,stroke-width:2px\n", name)
		}

		// Connect nodes to their dependencies (simplified - would need actual usage tracking)
		fmt.Println()
		fmt.Println("    %% Dependency connections")
		for depName := range wf.Contract.Dependencies {
			// For now, show dependencies as separate nodes
			// In a real implementation, we'd track which nodes use which dependencies
			fmt.Printf("    dep_%s -.->|external| %s\n", depName, depName)
		}
	}

	fmt.Println("```")

	return nil
}

// getPortWithDefault returns the port with protocol defaults applied
func getPortWithDefault(dep spec.Dependency) int {
	if dep.Port != 0 {
		return dep.Port
	}
	defaults := map[string]int{
		"https":      443,
		"postgresql": 5432,
		"nats":       4222,
	}
	if port, ok := defaults[dep.Protocol]; ok {
		return port
	}
	return 443
}
