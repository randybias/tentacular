package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randyb/pipedreamer2/pkg/k8s"
	"github.com/randyb/pipedreamer2/pkg/spec"
	"github.com/spf13/cobra"
)

func NewClusterCmd() *cobra.Command {
	cluster := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster management commands",
	}

	check := &cobra.Command{
		Use:   "check",
		Short: "Validate cluster readiness",
		RunE:  runClusterCheck,
	}
	check.Flags().Bool("fix", false, "Auto-create namespace and apply basic RBAC")

	cluster.AddCommand(check)
	return cluster
}

func runClusterCheck(cmd *cobra.Command, args []string) error {
	namespace, _ := cmd.Flags().GetString("namespace")
	fix, _ := cmd.Flags().GetBool("fix")
	output, _ := cmd.Flags().GetString("output")

	// Parse workflow spec from current directory to extract secret references
	secretNames := extractSecretNames()

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	results, err := client.PreflightCheck(namespace, fix, secretNames)
	if err != nil {
		return fmt.Errorf("preflight check failed: %w", err)
	}

	// JSON output
	if output == "json" {
		fmt.Println(k8s.CheckResultsJSON(results))
		for _, r := range results {
			if !r.Passed {
				return fmt.Errorf("cluster check failed")
			}
		}
		return nil
	}

	// Text output
	allPassed := true
	for _, r := range results {
		icon := "\u2713"
		if !r.Passed {
			icon = "\u2717"
			allPassed = false
		}
		fmt.Printf("  %s %s\n", icon, r.Name)
		if r.Warning != "" {
			fmt.Printf("    ⚠ %s\n", r.Warning)
		}
		if !r.Passed && r.Remediation != "" {
			fmt.Printf("    \u2192 %s\n", r.Remediation)
		}
	}

	if !allPassed {
		return fmt.Errorf("cluster check failed \u2014 see above for details")
	}

	fmt.Println("\n\u2713 Cluster is ready for deployment")
	return nil
}

// extractSecretNames attempts to parse a workflow spec from the current directory
// and returns secret names referenced by the workflow. Returns nil if no spec found.
func extractSecretNames() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	specPath := filepath.Join(cwd, "workflow.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil
	}

	wf, errs := spec.Parse(data)
	if len(errs) > 0 || wf == nil {
		return nil
	}

	// The convention is {workflow-name}-secrets (see pkg/builder/k8s.go)
	return []string{fmt.Sprintf("%s-secrets", wf.Name)}
}
