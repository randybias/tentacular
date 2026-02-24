package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/randybias/tentacular/pkg/spec"
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

	install := &cobra.Command{
		Use:   "install",
		Short: "Install cluster-level Tentacular components (module proxy, etc.)",
		RunE:  runClusterInstall,
	}
	install.Flags().Bool("module-proxy", true, "Install esm.sh module proxy for jsr/npm dep resolution")
	install.Flags().String("proxy-namespace", "", "Namespace for module proxy (default: tentacular-system)")
	install.Flags().String("proxy-storage", "", "Module proxy cache storage: emptydir (default) or pvc")
	install.Flags().String("proxy-pvc-size", "", "PVC size when storage=pvc (default: 5Gi)")
	install.Flags().String("proxy-image", "", "Module proxy image (default: ghcr.io/esm-dev/esm.sh:v136)")

	cluster.AddCommand(check)
	cluster.AddCommand(install)
	cluster.AddCommand(NewProfileCmd())
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

	// Append module proxy check (informational: not installed = warning, not failure)
	cfg := LoadConfig()
	proxyNS := cfg.ModuleProxy.Namespace
	if proxyNS == "" {
		proxyNS = "tentacular-system"
	}
	results = append(results, client.CheckModuleProxy(proxyNS))

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

func runClusterInstall(cmd *cobra.Command, args []string) error {
	installProxy, _ := cmd.Flags().GetBool("module-proxy")
	if !installProxy {
		fmt.Println("Nothing to install (use --module-proxy to install the module proxy).")
		return nil
	}

	cfg := LoadConfig()

	// Flags override config
	proxyNamespace, _ := cmd.Flags().GetString("proxy-namespace")
	if proxyNamespace == "" {
		proxyNamespace = cfg.ModuleProxy.Namespace
	}
	if proxyNamespace == "" {
		proxyNamespace = "tentacular-system"
	}

	storage, _ := cmd.Flags().GetString("proxy-storage")
	if storage == "" {
		storage = cfg.ModuleProxy.Storage
	}

	pvcSize, _ := cmd.Flags().GetString("proxy-pvc-size")
	if pvcSize == "" {
		pvcSize = cfg.ModuleProxy.PVCSize
	}

	image, _ := cmd.Flags().GetString("proxy-image")
	if image == "" {
		image = cfg.ModuleProxy.Image
	}

	namespace, _ := cmd.Flags().GetString("namespace")
	if namespace == "" || namespace == "default" {
		namespace = proxyNamespace
	}

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	// Ensure namespace exists
	if err := client.EnsureNamespace(proxyNamespace); err != nil {
		return fmt.Errorf("ensuring namespace %s: %w", proxyNamespace, err)
	}

	manifests := k8s.GenerateModuleProxyManifests(image, proxyNamespace, storage, pvcSize)

	fmt.Printf("Installing module proxy (esm.sh) in namespace %s...\n", proxyNamespace)
	if storage == "pvc" {
		fmt.Printf("  Storage: PVC (%s)\n", pvcSize)
	} else {
		fmt.Println("  Storage: emptyDir (cache lost on pod restart — use --proxy-storage=pvc for production)")
	}

	if err := client.Apply(proxyNamespace, manifests); err != nil {
		return fmt.Errorf("applying module proxy manifests: %w", err)
	}

	fmt.Printf("\n\u2713 Module proxy installed: http://esm-sh.%s.svc.cluster.local:8080\n", proxyNamespace)
	fmt.Println("  Workflow pods with jsr/npm deps will automatically route through it.")
	fmt.Println("  Run `tntc cluster check` to verify readiness.")
	return nil
}
