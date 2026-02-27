package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
)

// AuditResult holds the comparison results for all resources.
type AuditResult struct {
	NetworkPolicy NetworkPolicyAudit `json:"networkPolicy"`
	Secrets       SecretsAudit       `json:"secrets"`
	CronJobs      CronJobsAudit      `json:"cronJobs"`
	Overall       string             `json:"overall"` // "pass" or "fail"
}

// NetworkPolicyAudit holds NetworkPolicy comparison results.
type NetworkPolicyAudit struct {
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
	Status   string      `json:"status"` // "match", "mismatch", "missing"
	Details  []string    `json:"details,omitempty"`
}

// SecretsAudit holds Secrets comparison results.
type SecretsAudit struct {
	ExpectedKeys []string `json:"expectedKeys"`
	ActualKeys   []string `json:"actualKeys"`
	Missing      []string `json:"missing,omitempty"`
	Extra        []string `json:"extra,omitempty"`
	Status       string   `json:"status"` // "match", "mismatch", "missing"
}

// CronJobsAudit holds CronJob comparison results.
type CronJobsAudit struct {
	ExpectedCount int      `json:"expectedCount"`
	ActualCount   int      `json:"actualCount"`
	Status        string   `json:"status"` // "match", "mismatch"
	Details       []string `json:"details,omitempty"`
}

func NewAuditCommand() *cobra.Command {
	var (
		namespace    string
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "audit <workflow-dir>",
		Short: "Audit deployed resources against contract-derived expectations",
		Long: `Compares deployed Kubernetes resources (NetworkPolicy, Secrets, CronJobs)
against the expected resources derived from the workflow contract.

Reports discrepancies and validates that deployed state matches contract intent.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowDir := args[0]

			// Phase 1: Resolve - parse workflow and derive expected resources
			specPath := filepath.Join(workflowDir, "workflow.yaml")
			data, err := os.ReadFile(specPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", specPath, err)
			}

			wf, errs := spec.Parse(data)
			if len(errs) > 0 {
				return fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
			}

			if wf.Contract == nil {
				return fmt.Errorf("workflow has no contract section - audit requires contract")
			}

			expectedSecrets := spec.DeriveSecrets(wf.Contract)
			expectedEgress := spec.DeriveEgressRules(wf.Contract)
			expectedIngress := spec.DeriveIngressRules(wf)

			// Use deployment namespace from workflow config or flag
			if namespace == "" {
				namespace = wf.Deployment.Namespace
			}
			if namespace == "" {
				return fmt.Errorf("namespace required (use --namespace or set deployment.namespace in workflow)")
			}

			// Count expected cron triggers
			expectedCronCount := 0
			for _, trigger := range wf.Triggers {
				if trigger.Type == "cron" {
					expectedCronCount++
				}
			}

			// Phase 2: Fetch via MCP
			mcpClient, err := requireMCPClient(cmd)
			if err != nil {
				return err
			}

			expected := map[string]interface{}{
				"secrets":           expectedSecrets,
				"egressRuleCount":   len(expectedEgress),
				"ingressRuleCount":  len(expectedIngress),
				"cronJobCount":      expectedCronCount,
			}

			mcpResult, err := mcpClient.AuditResources(cmd.Context(), namespace, wf.Name, expected)
			if err != nil {
				if hint := mcpErrorHint(err); hint != "" {
					return fmt.Errorf("audit failed: %w\n  hint: %s", err, hint)
				}
				return fmt.Errorf("audit failed: %w", err)
			}

			// Map MCP result back to AuditResult for output compatibility
			result := AuditResult{
				Overall: mcpResult.Overall,
				NetworkPolicy: NetworkPolicyAudit{
					Status:  mcpResult.NetworkPolicy.Status,
					Details: mcpResult.NetworkPolicy.Details,
					Expected: map[string]interface{}{
						"egressRuleCount":  len(expectedEgress),
						"ingressRuleCount": len(expectedIngress),
					},
				},
				Secrets: SecretsAudit{
					Status:       mcpResult.Secrets.Status,
					ExpectedKeys: expectedSecrets,
					Missing:      mcpResult.Secrets.Missing,
					Extra:        mcpResult.Secrets.Extra,
				},
				CronJobs: CronJobsAudit{
					Status:        mcpResult.CronJobs.Status,
					ExpectedCount: expectedCronCount,
					Details:       mcpResult.CronJobs.Details,
				},
			}

			// Phase 3: Output results
			if outputFormat == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Text output
			fmt.Fprintf(cmd.OutOrStdout(), "Audit Report: %s (namespace: %s)\n", wf.Name, namespace)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNetworkPolicy: %s\n", result.NetworkPolicy.Status)
			for _, detail := range result.NetworkPolicy.Details {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", detail)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nSecrets: %s\n", result.Secrets.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "  Expected keys: %v\n", result.Secrets.ExpectedKeys)
			if len(result.Secrets.Missing) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Missing keys: %v\n", result.Secrets.Missing)
			}
			if len(result.Secrets.Extra) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Extra keys: %v\n", result.Secrets.Extra)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nCronJobs: %s\n", result.CronJobs.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "  Expected: %d, Actual: %d\n", result.CronJobs.ExpectedCount, result.CronJobs.ActualCount)
			for _, detail := range result.CronJobs.Details {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", detail)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nOverall: %s\n", result.Overall)

			if result.Overall == "fail" {
				return fmt.Errorf("audit failed - deployed resources do not match contract expectations")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (overrides workflow deployment.namespace)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text or json)")

	return cmd
}
