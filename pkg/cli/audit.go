package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/k8s"
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

			// Phase 2: Fetch - get actual deployed resources
			client, err := k8s.NewClient()
			if err != nil {
				return fmt.Errorf("creating k8s client: %w", err)
			}

			// Use deployment namespace from workflow config or flag
			if namespace == "" {
				namespace = wf.Deployment.Namespace
			}
			if namespace == "" {
				return fmt.Errorf("namespace required (use --namespace or set deployment.namespace in workflow)")
			}

			workflowName := wf.Name
			result := AuditResult{
				Overall: "pass",
			}

			// Fetch NetworkPolicy
			netpolName := workflowName + "-netpol"
			netpol, err := client.GetNetworkPolicy(namespace, netpolName)
			if err != nil {
				result.NetworkPolicy.Status = "missing"
				result.NetworkPolicy.Details = append(result.NetworkPolicy.Details, fmt.Sprintf("NetworkPolicy not found: %v", err))
				result.Overall = "fail"
			} else {
				// Compare NetworkPolicy (detailed comparison TBD)
				result.NetworkPolicy.Expected = map[string]interface{}{
					"egressRuleCount":  len(expectedEgress),
					"ingressRuleCount": len(expectedIngress),
				}
				result.NetworkPolicy.Actual = netpol.Object
				result.NetworkPolicy.Status = "found"
				// Detailed comparison logic to be added
			}

			// Fetch Secret
			secretName := workflowName + "-secrets"
			if len(expectedSecrets) == 0 {
				// No secrets expected — no K8s Secret needed
				result.Secrets.Status = "match"
				result.Secrets.ExpectedKeys = nil
				result.Secrets.ActualKeys = nil
			} else {
				// Convert expected dotted keys to service names for comparison
				expectedServiceNames := make([]string, 0, len(expectedSecrets))
				seen := make(map[string]bool)
				for _, key := range expectedSecrets {
					svcName := spec.GetSecretServiceName(key)
					if svcName != "" && !seen[svcName] {
						expectedServiceNames = append(expectedServiceNames, svcName)
						seen[svcName] = true
					}
				}

				secret, err := client.GetSecret(namespace, secretName)
				if err != nil {
					result.Secrets.Status = "missing"
					result.Secrets.ExpectedKeys = expectedSecrets
					result.Overall = "fail"
				} else {
					result.Secrets.ExpectedKeys = expectedSecrets
					var actualKeys []string
					for key := range secret.Data {
						actualKeys = append(actualKeys, key)
					}
					result.Secrets.ActualKeys = actualKeys

					// Compare at service-name level
					missing, extra := compareSecretKeys(expectedServiceNames, actualKeys)
					result.Secrets.Missing = missing
					result.Secrets.Extra = extra

					if len(missing) > 0 || len(extra) > 0 {
						result.Secrets.Status = "mismatch"
						result.Overall = "fail"
					} else {
						result.Secrets.Status = "match"
					}
				}
			}

			// Fetch CronJobs
			cronJobs, err := client.GetCronJobs(namespace, "app.kubernetes.io/name="+workflowName+",app.kubernetes.io/managed-by=tentacular")
			if err != nil {
				result.CronJobs.Details = append(result.CronJobs.Details, fmt.Sprintf("Error fetching CronJobs: %v", err))
				result.Overall = "fail"
			} else {
				// Count expected cron triggers
				expectedCronCount := 0
				for _, trigger := range wf.Triggers {
					if trigger.Type == "cron" {
						expectedCronCount++
					}
				}

				result.CronJobs.ExpectedCount = expectedCronCount
				result.CronJobs.ActualCount = len(cronJobs)

				if expectedCronCount != len(cronJobs) {
					result.CronJobs.Status = "mismatch"
					result.CronJobs.Details = append(result.CronJobs.Details,
						fmt.Sprintf("Expected %d CronJobs, found %d", expectedCronCount, len(cronJobs)))
					result.Overall = "fail"
				} else {
					result.CronJobs.Status = "match"
				}
			}

			// Phase 3: Output results
			if outputFormat == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Text output
			fmt.Fprintf(cmd.OutOrStdout(), "Audit Report: %s (namespace: %s)\n", workflowName, namespace)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNetworkPolicy: %s\n", result.NetworkPolicy.Status)
			if len(result.NetworkPolicy.Details) > 0 {
				for _, detail := range result.NetworkPolicy.Details {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", detail)
				}
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
			if len(result.CronJobs.Details) > 0 {
				for _, detail := range result.CronJobs.Details {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", detail)
				}
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

// compareSecretKeys returns missing and extra keys.
func compareSecretKeys(expected, actual []string) (missing, extra []string) {
	expectedSet := make(map[string]bool)
	for _, key := range expected {
		expectedSet[key] = true
	}

	actualSet := make(map[string]bool)
	for _, key := range actual {
		actualSet[key] = true
	}

	// Find missing keys (in expected but not in actual)
	for _, key := range expected {
		if !actualSet[key] {
			missing = append(missing, key)
		}
	}

	// Find extra keys (in actual but not in expected)
	for _, key := range actual {
		if !expectedSet[key] {
			extra = append(extra, key)
		}
	}

	return missing, extra
}
