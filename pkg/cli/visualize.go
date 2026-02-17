package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	cmd.Flags().Bool("write", false, "Write visualization artifacts to workflow directory")
	return cmd
}

func runVisualize(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	rich, _ := cmd.Flags().GetBool("rich")
	write, _ := cmd.Flags().GetBool("write")

	specPath := filepath.Join(dir, "workflow.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", specPath, err)
	}

	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		return fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
	}

	// Generate Mermaid diagram
	mermaidContent := generateMermaidDiagram(wf, rich)

	// Generate contract summary if rich mode
	var contractSummary string
	if rich && wf.Contract != nil {
		contractSummary = generateContractSummary(wf)
	}

	// Write mode: save to files
	if write {
		// Write Mermaid diagram
		mermaidPath := filepath.Join(dir, "workflow-diagram.md")
		if err := os.WriteFile(mermaidPath, []byte(mermaidContent), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", mermaidPath, err)
		}
		fmt.Printf("✓ Wrote Mermaid diagram to %s\n", mermaidPath)

		// Write contract summary if available
		if contractSummary != "" {
			summaryPath := filepath.Join(dir, "contract-summary.md")
			if err := os.WriteFile(summaryPath, []byte(contractSummary), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", summaryPath, err)
			}
			fmt.Printf("✓ Wrote contract summary to %s\n", summaryPath)
		}
	} else {
		// Print mode: output to stdout
		fmt.Print(mermaidContent)
		if contractSummary != "" {
			fmt.Print(contractSummary)
		}
	}

	return nil
}

// generateMermaidDiagram generates Mermaid diagram content
func generateMermaidDiagram(wf *spec.Workflow, rich bool) string {
	var buf bytes.Buffer

	buf.WriteString("```mermaid\n")
	buf.WriteString("graph TD\n")

	// Render workflow nodes in sorted order for deterministic output
	nodeNames := make([]string, 0, len(wf.Nodes))
	for name := range wf.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	for _, name := range nodeNames {
		fmt.Fprintf(&buf, "    %s[%s]\n", name, name)
	}

	// Render workflow edges
	for _, edge := range wf.Edges {
		fmt.Fprintf(&buf, "    %s --> %s\n", edge.From, edge.To)
	}

	// Render dependencies if --rich flag is set
	if rich && wf.Contract != nil && len(wf.Contract.Dependencies) > 0 {
		buf.WriteString("\n")
		buf.WriteString("    %% External Dependencies\n")

		// Sort dependency names for deterministic output
		depNames := make([]string, 0, len(wf.Contract.Dependencies))
		for name := range wf.Contract.Dependencies {
			depNames = append(depNames, name)
		}
		sort.Strings(depNames)

		for _, name := range depNames {
			dep := wf.Contract.Dependencies[name]
			fmt.Fprintf(&buf, "    dep_%s[(%s<br/>%s:%d)]\n", name, name, dep.Host, getPortWithDefault(dep))
			fmt.Fprintf(&buf, "    style dep_%s fill:#e1f5ff,stroke:#0066cc,stroke-width:2px\n", name)
		}

		// Connect nodes to their dependencies
		buf.WriteString("\n")
		buf.WriteString("    %% Dependency connections\n")
		for _, depName := range depNames {
			fmt.Fprintf(&buf, "    dep_%s -.->|external| %s\n", depName, depName)
		}
	}

	buf.WriteString("```\n")
	return buf.String()
}

// generateContractSummary generates contract summary content
func generateContractSummary(wf *spec.Workflow) string {
	if wf.Contract == nil {
		return ""
	}

	var buf bytes.Buffer

	buf.WriteString("\n## Derived Artifacts\n\n")

	// Derived secrets
	secrets := spec.DeriveSecrets(wf.Contract)
	if len(secrets) > 0 {
		buf.WriteString("### Secrets\n\n")
		for _, secretKey := range secrets {
			serviceName := spec.GetSecretServiceName(secretKey)
			keyName := spec.GetSecretKeyName(secretKey)
			fmt.Fprintf(&buf, "- `%s` → service=%s, key=%s\n", secretKey, serviceName, keyName)
		}
		buf.WriteString("\n")
	}

	// Derived egress rules
	egressRules := spec.DeriveEgressRules(wf.Contract)
	if len(egressRules) > 0 {
		buf.WriteString("### Egress Rules (NetworkPolicy)\n\n")
		buf.WriteString("| Host | Port | Protocol |\n")
		buf.WriteString("|------|------|----------|\n")
		for _, rule := range egressRules {
			portStr := fmt.Sprintf("%d", rule.Port)
			if rule.Port == 0 {
				portStr = "any"
			}
			fmt.Fprintf(&buf, "| %s | %s | %s |\n", rule.Host, portStr, rule.Protocol)
		}
		buf.WriteString("\n")
	}

	// Derived ingress rules
	ingressRules := spec.DeriveIngressRules(wf)
	if len(ingressRules) > 0 {
		buf.WriteString("### Ingress Rules (NetworkPolicy)\n\n")
		buf.WriteString("| Port | Protocol | Trigger |\n")
		buf.WriteString("|------|----------|---------|\n")
		for _, rule := range ingressRules {
			fmt.Fprintf(&buf, "| %d | %s | webhook |\n", rule.Port, rule.Protocol)
		}
		buf.WriteString("\n")
	}

	return buf.String()
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
