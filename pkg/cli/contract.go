package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/randybias/tentacular/pkg/builder"
	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// bootstrapHosts lists Deno/npm registries used to fetch dependencies at runtime.
// These should be removed from the NetworkPolicy once dependencies are cached locally.
var bootstrapHosts = map[string]bool{
	"jsr.io":             true,
	"deno.land":          true,
	"cdn.deno.land":      true,
	"registry.npmjs.org": true,
}

// isBootstrapHost returns true if the host is a known Deno/npm bootstrap registry.
func isBootstrapHost(host string) bool {
	return bootstrapHosts[strings.ToLower(strings.TrimSpace(host))]
}

// NewContractCmd returns the `tntc contract` subcommand group.
func NewContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "Manage workflow contract and NetworkPolicy",
	}
	cmd.AddCommand(NewContractStatusCmd())
	cmd.AddCommand(NewContractLockCmd())
	return cmd
}

// NewContractStatusCmd returns the `tntc contract status` subcommand.
func NewContractStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [dir]",
		Short: "Show live NetworkPolicy egress vs contract definition",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runContractStatus,
	}
	cmd.Flags().String("env", "", "Target environment from config (resolves kubeconfig, namespace)")
	return cmd
}

// NewContractLockCmd returns the `tntc contract lock` subcommand.
func NewContractLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock [dir]",
		Short: "Remove bootstrap egress rules from live NetworkPolicy (no pod restart)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runContractLock,
	}
	cmd.Flags().String("env", "", "Target environment from config (resolves kubeconfig, namespace)")
	cmd.Flags().Bool("dry-run", false, "Show what would change without applying")
	return cmd
}

func runContractStatus(cmd *cobra.Command, args []string) error {
	dir, envName, namespace, _, err := contractCmdArgs(cmd, args)
	if err != nil {
		return err
	}

	wf, err := loadWorkflow(dir)
	if err != nil {
		return err
	}
	if wf.Contract == nil {
		return fmt.Errorf("workflow %q has no contract — nothing to check", wf.Name)
	}

	if namespace == "" {
		namespace = resolveContractNamespace(wf, envName)
	}

	client, err := contractK8sClient(envName)
	if err != nil {
		return err
	}

	netpolName := wf.Name + "-netpol"
	netpol, err := client.GetNetworkPolicy(namespace, netpolName)
	if err != nil {
		return fmt.Errorf("fetching NetworkPolicy %s/%s: %w", namespace, netpolName, err)
	}

	contractHosts := contractEgressHosts(wf)
	liveHosts := liveEgressHosts(netpol)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return printContractStatusJSON(wf.Name, namespace, contractHosts, liveHosts)
	}
	return printContractStatusText(wf.Name, namespace, netpolName, contractHosts, liveHosts)
}

func runContractLock(cmd *cobra.Command, args []string) error {
	dir, envName, namespace, dryRun, err := contractCmdArgs(cmd, args)
	if err != nil {
		return err
	}

	wf, err := loadWorkflow(dir)
	if err != nil {
		return err
	}
	if wf.Contract == nil {
		return fmt.Errorf("workflow %q has no contract — nothing to lock", wf.Name)
	}

	if namespace == "" {
		namespace = resolveContractNamespace(wf, envName)
	}

	client, err := contractK8sClient(envName)
	if err != nil {
		return err
	}

	netpolName := wf.Name + "-netpol"
	netpol, err := client.GetNetworkPolicy(namespace, netpolName)
	if err != nil {
		return fmt.Errorf("fetching NetworkPolicy %s/%s: %w", namespace, netpolName, err)
	}

	liveHosts := liveEgressHosts(netpol)
	var toRemove []string
	for _, h := range liveHosts {
		if isBootstrapHost(h) {
			toRemove = append(toRemove, h)
		}
	}

	if len(toRemove) == 0 {
		fmt.Printf("%s/%s: already clean — no bootstrap egress rules present.\n", namespace, netpolName)
		return nil
	}

	fmt.Printf("Bootstrap egress rules found in %s/%s:\n", namespace, netpolName)
	for _, h := range toRemove {
		fmt.Printf("  - %s\n", h)
	}

	if dryRun {
		fmt.Println("\n--dry-run: no changes applied.")
		return nil
	}

	// Build a clean workflow copy with bootstrap dependencies filtered out
	cleanWf := filterBootstrapDeps(wf)
	manifest := k8s.GenerateNetworkPolicy(cleanWf, namespace)
	if manifest == nil {
		return fmt.Errorf("failed to generate clean NetworkPolicy")
	}

	if err := client.Apply(namespace, []builder.Manifest{*manifest}); err != nil {
		return fmt.Errorf("applying NetworkPolicy: %w", err)
	}

	fmt.Printf("\nRemoved %d bootstrap egress rule(s). %s/%s updated.\n",
		len(toRemove), namespace, netpolName)
	fmt.Println("Pods are unaffected — no restart required.")
	return nil
}

// loadWorkflow reads and parses workflow.yaml from the given directory.
func loadWorkflow(dir string) (*spec.Workflow, error) {
	specPath := filepath.Join(dir, "workflow.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading workflow spec: %w", err)
	}
	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		return nil, fmt.Errorf("workflow spec has %d validation error(s): %s", len(errs), errs[0])
	}
	return wf, nil
}

// contractCmdArgs parses common arguments for contract subcommands.
func contractCmdArgs(cmd *cobra.Command, args []string) (dir, envName, namespace string, dryRun bool, err error) {
	dir = "."
	if len(args) > 0 {
		dir, err = filepath.Abs(args[0])
		if err != nil {
			return "", "", "", false, fmt.Errorf("resolving path: %w", err)
		}
	}
	envName, _ = cmd.Flags().GetString("env")
	namespace, _ = cmd.Flags().GetString("namespace")
	dryRun, _ = cmd.Flags().GetBool("dry-run")
	return dir, envName, namespace, dryRun, nil
}

// resolveContractNamespace picks namespace from env config, workflow deployment config, or defaults.
func resolveContractNamespace(wf *spec.Workflow, envName string) string {
	if envName != "" {
		env, err := LoadEnvironment(envName)
		if err == nil && env.Namespace != "" {
			return env.Namespace
		}
	}
	if wf.Deployment.Namespace != "" {
		return wf.Deployment.Namespace
	}
	cfg := LoadConfig()
	if cfg.Namespace != "" {
		return cfg.Namespace
	}
	return "default"
}

// contractK8sClient creates a K8s client respecting the --env flag for kubeconfig/context.
func contractK8sClient(envName string) (*k8s.Client, error) {
	if envName != "" {
		env, err := LoadEnvironment(envName)
		if err != nil {
			return nil, fmt.Errorf("loading environment %q: %w", envName, err)
		}
		if env.Kubeconfig != "" {
			return k8s.NewClientFromConfig(env.Kubeconfig, env.Context)
		}
		if env.Context != "" {
			return k8s.NewClientWithContext(env.Context)
		}
	}
	return k8s.NewClient()
}

// contractEgressHosts returns external hosts (host:port) declared in the workflow contract.
// DNS and cluster-internal hosts are excluded.
func contractEgressHosts(wf *spec.Workflow) []string {
	rules := spec.DeriveEgressRules(wf.Contract)
	seen := make(map[string]bool)
	var hosts []string
	for _, r := range rules {
		if r.Port == 53 {
			continue // skip DNS
		}
		if strings.HasSuffix(r.Host, ".svc.cluster.local") {
			continue // skip cluster-internal
		}
		key := fmt.Sprintf("%s:%d", r.Host, r.Port)
		if !seen[key] {
			seen[key] = true
			hosts = append(hosts, key)
		}
	}
	return hosts
}

// liveEgressHosts parses the tentacular.dev/intended-hosts annotation from a live NetworkPolicy.
func liveEgressHosts(netpol *unstructured.Unstructured) []string {
	if netpol == nil {
		return nil
	}
	annotations := netpol.GetAnnotations()
	raw, ok := annotations["tentacular.dev/intended-hosts"]
	if !ok || raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var hosts []string
	for _, p := range parts {
		if h := strings.TrimSpace(p); h != "" {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// filterBootstrapDeps returns a deep copy of the workflow with bootstrap dependency hosts removed.
func filterBootstrapDeps(wf *spec.Workflow) *spec.Workflow {
	clean := *wf
	cleanDeps := make(map[string]spec.Dependency)
	for name, dep := range wf.Contract.Dependencies {
		if !isBootstrapHost(dep.Host) {
			cleanDeps[name] = dep
		}
	}
	cleanContract := *wf.Contract
	cleanContract.Dependencies = cleanDeps
	clean.Contract = &cleanContract
	return &clean
}

// printContractStatusText renders the contract status in human-readable format.
func printContractStatusText(workflowName, namespace, netpolName string, contractHosts, liveHosts []string) error {
	fmt.Printf("Workflow:  %s\n", workflowName)
	fmt.Printf("Namespace: %s\n\n", namespace)

	if len(contractHosts) > 0 {
		fmt.Println("CONTRACT EGRESS (from workflow.yaml):")
		for _, h := range contractHosts {
			fmt.Printf("  ✓ %s\n", h)
		}
		fmt.Println()
	}

	fmt.Printf("LIVE NETWORK POLICY EGRESS (%s/%s):\n", namespace, netpolName)
	bootstrapCount := 0
	if len(liveHosts) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, h := range liveHosts {
			if isBootstrapHost(h) {
				fmt.Printf("  ⚠ %s  [bootstrap — removable with: tntc contract lock]\n", h)
				bootstrapCount++
			} else {
				fmt.Printf("  ✓ %s\n", h)
			}
		}
	}
	fmt.Println()

	if bootstrapCount == 0 {
		fmt.Println("STATUS: Clean — no bootstrap egress rules present.")
	} else {
		fmt.Printf("STATUS: %d bootstrap egress rule(s) present. Run `tntc contract lock` to remove.\n", bootstrapCount)
	}
	return nil
}

// printContractStatusJSON renders the contract status as JSON.
func printContractStatusJSON(workflowName, namespace string, contractHosts, liveHosts []string) error {
	type hostEntry struct {
		Host      string `json:"host"`
		Bootstrap bool   `json:"bootstrap,omitempty"`
	}

	live := make([]hostEntry, 0, len(liveHosts))
	bootstrapCount := 0
	for _, h := range liveHosts {
		entry := hostEntry{Host: h, Bootstrap: isBootstrapHost(h)}
		if entry.Bootstrap {
			bootstrapCount++
		}
		live = append(live, entry)
	}

	contract := make([]hostEntry, 0, len(contractHosts))
	for _, h := range contractHosts {
		contract = append(contract, hostEntry{Host: h})
	}

	out := map[string]interface{}{
		"workflow":       workflowName,
		"namespace":      namespace,
		"contract":       contract,
		"live":           live,
		"bootstrapCount": bootstrapCount,
		"clean":          bootstrapCount == 0,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
