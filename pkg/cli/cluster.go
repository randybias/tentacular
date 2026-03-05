package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func NewClusterCmd() *cobra.Command {
	cluster := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster management commands",
	}

	check := &cobra.Command{
		Use:   "check",
		Short: "Validate cluster readiness via MCP server",
		RunE:  runClusterCheck,
	}

	cluster.AddCommand(check)
	cluster.AddCommand(NewProfileCmd())
	return cluster
}

func runClusterCheck(cmd *cobra.Command, args []string) error {
	namespace := resolveNamespace(cmd, "")
	output, _ := cmd.Flags().GetString("output")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	preflightResult, err := mcpClient.ClusterPreflight(cmd.Context(), namespace)
	if err != nil {
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("preflight check failed: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("preflight check failed: %w", err)
	}

	// JSON output
	if output == "json" {
		data, jsonErr := json.Marshal(preflightResult)
		if jsonErr != nil {
			return fmt.Errorf("marshaling JSON: %w", jsonErr)
		}
		fmt.Println(string(data))
		if !preflightResult.AllPass {
			return fmt.Errorf("cluster check failed")
		}
		return nil
	}

	// Text output
	for _, r := range preflightResult.Results {
		icon := "\u2713"
		if !r.Passed {
			icon = "\u2717"
		}
		fmt.Printf("  %s %s\n", icon, r.Name)
		if r.Warning != "" {
			fmt.Printf("    \u26a0 %s\n", r.Warning)
		}
		if !r.Passed && r.Remediation != "" {
			fmt.Printf("    \u2192 %s\n", r.Remediation)
		}
	}

	if !preflightResult.AllPass {
		return fmt.Errorf("cluster check failed \u2014 see above for details")
	}

	fmt.Println("\n\u2713 Cluster is ready for deployment")
	return nil
}


