package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
)

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [dir]",
		Short: "Validate workflow spec",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runValidate,
	}
	cmd.Flags().BoolP("verbose", "v", false, "Show derived artifacts")
	cmd.Flags().StringP("output", "o", "", "Output format (json)")
	return cmd
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
	outputFormat, _ := cmd.Flags().GetString("output")
	out := cmd.OutOrStdout()

	// JSON output mode
	if outputFormat == "json" {
		return outputValidateJSON(wf, out)
	}

	// Text output mode
	if verbose {
		fmt.Fprintf(out, "Workflow: %s (v%s)\n", wf.Name, wf.Version)
		fmt.Fprintf(out, "Nodes:    %d\n", len(wf.Nodes))
		fmt.Fprintf(out, "Edges:    %d\n", len(wf.Edges))
		fmt.Fprintf(out, "Triggers: %d\n", len(wf.Triggers))

		// Show derived artifacts if contract exists
		if wf.Contract != nil {
			fmt.Fprintf(out, "\nDerived Artifacts:\n")

			// Secrets
			secrets := spec.DeriveSecrets(wf.Contract)
			if len(secrets) > 0 {
				fmt.Fprintf(out, "  Secrets: %v\n", secrets)
			}

			// Egress Rules
			egressRules := spec.DeriveEgressRules(wf.Contract)
			if len(egressRules) > 0 {
				fmt.Fprintf(out, "  Egress Rules:\n")
				for _, rule := range egressRules {
					fmt.Fprintf(out, "    %s:%d/%s\n", rule.Host, rule.Port, rule.Protocol)
				}
			}

			// Ingress Rules
			ingressRules := spec.DeriveIngressRules(wf)
			if len(ingressRules) > 0 {
				fmt.Fprintf(out, "  Ingress Rules:\n")
				for _, rule := range ingressRules {
					if rule.FromLabels != nil {
						var labels string
						for k, v := range rule.FromLabels {
							if labels != "" {
								labels += ", "
							}
							labels += fmt.Sprintf("%s=%s", k, v)
						}
						fmt.Fprintf(out, "    %d/%s (from: %s)\n", rule.Port, rule.Protocol, labels)
					} else {
						fmt.Fprintf(out, "    %d/%s\n", rule.Port, rule.Protocol)
					}
				}
			}
		}
		fmt.Fprintf(out, "\n")
	}

	fmt.Fprintf(out, "✓ %s is valid\n", specPath)
	return nil
}

// ValidateResult is the JSON output structure for validate command.
type ValidateResult struct {
	Workflow     string            `json:"workflow"`
	Version      string            `json:"version"`
	Nodes        int               `json:"nodes"`
	Edges        int               `json:"edges"`
	Triggers     int               `json:"triggers"`
	HasContract  bool              `json:"hasContract"`
	Secrets      []string          `json:"secrets,omitempty"`
	EgressRules  []EgressRuleJSON  `json:"egressRules,omitempty"`
	IngressRules []IngressRuleJSON `json:"ingressRules,omitempty"`
}

// EgressRuleJSON is the JSON representation of an egress rule.
type EgressRuleJSON struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// IngressRuleJSON is the JSON representation of an ingress rule.
type IngressRuleJSON struct {
	Port       int               `json:"port"`
	Protocol   string            `json:"protocol"`
	FromLabels map[string]string `json:"fromLabels,omitempty"`
}

// outputValidateJSON outputs validation results in JSON format.
func outputValidateJSON(wf *spec.Workflow, out io.Writer) error {
	result := ValidateResult{
		Workflow:    wf.Name,
		Version:     wf.Version,
		Nodes:       len(wf.Nodes),
		Edges:       len(wf.Edges),
		Triggers:    len(wf.Triggers),
		HasContract: wf.Contract != nil,
	}

	if wf.Contract != nil {
		// Derive secrets
		result.Secrets = spec.DeriveSecrets(wf.Contract)

		// Derive egress rules
		egressRules := spec.DeriveEgressRules(wf.Contract)
		for _, rule := range egressRules {
			result.EgressRules = append(result.EgressRules, EgressRuleJSON{
				Host:     rule.Host,
				Port:     rule.Port,
				Protocol: rule.Protocol,
			})
		}

		// Derive ingress rules
		ingressRules := spec.DeriveIngressRules(wf)
		for _, rule := range ingressRules {
			result.IngressRules = append(result.IngressRules, IngressRuleJSON{
				Port:       rule.Port,
				Protocol:   rule.Protocol,
				FromLabels: rule.FromLabels,
			})
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Fprintln(out, string(data))
	return nil
}
