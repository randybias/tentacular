package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/spec"
)

// AuditResult holds the comparison results for all resources.
type AuditResult struct {
	Overall       string             `json:"overall"` // "pass" or "fail"
	NetworkPolicy NetworkPolicyAudit `json:"networkPolicy"`
	Secrets       SecretsAudit       `json:"secrets"`
	CronJobs      CronJobsAudit      `json:"cronJobs"`
}

// NetworkPolicyAudit holds NetworkPolicy comparison results.
type NetworkPolicyAudit struct {
	Expected any      `json:"expected"`
	Actual   any      `json:"actual"`
	Status   string   `json:"status"` // "match", "mismatch", "missing"
	Details  []string `json:"details,omitempty"`
}

// SecretsAudit holds Secrets comparison results.
type SecretsAudit struct {
	Status       string   `json:"status"`
	ExpectedKeys []string `json:"expectedKeys"`
	ActualKeys   []string `json:"actualKeys"`
	Missing      []string `json:"missing,omitempty"`
	Extra        []string `json:"extra,omitempty"`
}

// CronJobsAudit holds CronJob comparison results.
type CronJobsAudit struct {
	Status        string   `json:"status"` // "match", "mismatch"
	Details       []string `json:"details,omitempty"`
	ExpectedCount int      `json:"expectedCount"`
	ActualCount   int      `json:"actualCount"`
}

func NewAuditCommand() *cobra.Command {
	var outputFormat string

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
			data, err := os.ReadFile(specPath) //nolint:gosec // reading user-specified workflow file
			if err != nil {
				return fmt.Errorf("reading %s: %w", specPath, err)
			}

			wf, errs := spec.Parse(data)
			if len(errs) > 0 {
				return fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
			}

			if wf.Contract == nil {
				return errors.New("workflow has no contract section - audit requires contract")
			}

			expectedSecrets := spec.DeriveSecrets(wf.Contract)
			expectedEgress := spec.DeriveEgressRules(wf.Contract)
			expectedIngress := spec.DeriveIngressRules(wf)

			// Namespace cascade: workflow.yaml > env config > global config > "default"
			namespace := resolveNamespace(cmd, workflowDir)
			if namespace == "default" && wf.Deployment.Namespace == "" {
				return errors.New("namespace required: set deployment.namespace in workflow.yaml or configure an environment")
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

			expected := map[string]any{
				"secrets":          expectedSecrets,
				"egressRuleCount":  len(expectedEgress),
				"ingressRuleCount": len(expectedIngress),
				"cronJobCount":     expectedCronCount,
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
					Expected: map[string]any{
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Audit Report: %s (namespace: %s)\n", wf.Name, namespace)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nNetworkPolicy: %s\n", result.NetworkPolicy.Status)
			for _, detail := range result.NetworkPolicy.Details {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", detail)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nSecrets: %s\n", result.Secrets.Status)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Expected keys: %v\n", result.Secrets.ExpectedKeys)
			if len(result.Secrets.Missing) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Missing keys: %v\n", result.Secrets.Missing)
			}
			if len(result.Secrets.Extra) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Extra keys: %v\n", result.Secrets.Extra)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nCronJobs: %s\n", result.CronJobs.Status)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Expected: %d, Actual: %d\n", result.CronJobs.ExpectedCount, result.CronJobs.ActualCount)
			for _, detail := range result.CronJobs.Details {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", detail)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nOverall: %s\n", result.Overall)

			if result.Overall == "fail" {
				return errors.New("audit failed - deployed resources do not match contract expectations")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format (text or json)")

	return cmd
}
