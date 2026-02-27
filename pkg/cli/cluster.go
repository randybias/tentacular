package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/randybias/tentacular/pkg/mcp"
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

	install := &cobra.Command{
		Use:   "install",
		Short: "Bootstrap Tentacular: deploy the MCP server and module proxy",
		Long: `Install the tentacular-mcp server into the cluster.

This is the bootstrap command and the ONLY tntc command that communicates
directly with the Kubernetes API. All other commands route through the MCP
server once it is installed.

After install, the MCP endpoint and token are saved to ~/.tentacular/config.yaml
so subsequent tntc commands automatically find and use the MCP server.`,
		RunE: runClusterInstall,
	}
	install.Flags().String("namespace", "tentacular-system", "Namespace to install MCP server into")
	install.Flags().String("proxy-namespace", k8s.DefaultProxyNamespace, "Namespace to install module proxy into")
	install.Flags().String("image", "", "MCP server image (default: "+k8s.DefaultMCPImage+")")
	install.Flags().Bool("module-proxy", true, "Install esm.sh module proxy for jsr/npm dep resolution")
	install.Flags().String("proxy-storage", "", "Module proxy cache storage: emptydir (default) or pvc")
	install.Flags().String("proxy-pvc-size", "", "PVC size when storage=pvc (default: 5Gi)")
	install.Flags().String("proxy-image", "", "Module proxy image (default: ghcr.io/esm-dev/esm.sh:v136)")
	install.Flags().Duration("wait", 120*time.Second, "Timeout for MCP server to become ready")

	cluster.AddCommand(check)
	cluster.AddCommand(install)
	cluster.AddCommand(NewProfileCmd())
	return cluster
}

func runClusterCheck(cmd *cobra.Command, args []string) error {
	namespace, _ := cmd.Flags().GetString("namespace")
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

// runClusterInstall is the bootstrap command that deploys the MCP server and
// optionally the esm.sh module proxy. This is the ONLY tntc command that talks
// directly to the Kubernetes API — all other commands route through MCP.
func runClusterInstall(cmd *cobra.Command, args []string) error {
	namespace, _ := cmd.Flags().GetString("namespace")
	if namespace == "" {
		namespace = k8s.DefaultMCPNamespace
	}
	proxyNamespace, _ := cmd.Flags().GetString("proxy-namespace")
	if proxyNamespace == "" {
		proxyNamespace = k8s.DefaultProxyNamespace
	}
	mcpImage, _ := cmd.Flags().GetString("image")
	installProxy, _ := cmd.Flags().GetBool("module-proxy")
	proxyStorage, _ := cmd.Flags().GetString("proxy-storage")
	pvcSize, _ := cmd.Flags().GetString("proxy-pvc-size")
	proxyImage, _ := cmd.Flags().GetString("proxy-image")
	waitTimeout, _ := cmd.Flags().GetDuration("wait")

	cfg := LoadConfig()
	if proxyStorage == "" {
		proxyStorage = cfg.ModuleProxy.Storage
	}
	if pvcSize == "" {
		pvcSize = cfg.ModuleProxy.PVCSize
	}
	if proxyImage == "" {
		proxyImage = cfg.ModuleProxy.Image
	}

	// Step 1: Create K8s client (bootstrap — direct K8s access only here)
	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	// Step 2: Ensure tentacular-system namespace
	fmt.Printf("Ensuring namespace %s...\n", namespace)
	if err := client.EnsureNamespace(namespace); err != nil {
		return fmt.Errorf("ensuring namespace %s: %w", namespace, err)
	}
	fmt.Printf("  \u2713 Namespace %s ready\n", namespace)

	// Step 3: Generate MCP auth token
	fmt.Println("Generating MCP auth token...")
	token, err := k8s.GenerateMCPToken()
	if err != nil {
		return fmt.Errorf("generating MCP token: %w", err)
	}
	fmt.Println("  \u2713 Token generated")

	// Step 4: Deploy MCP server
	fmt.Printf("Deploying MCP server (%s) to namespace %s...\n", mcpImage, namespace)
	if mcpImage == "" {
		mcpImage = k8s.DefaultMCPImage
	}
	mcpManifests := k8s.GenerateMCPServerManifests(namespace, mcpImage, token)
	if err := client.Apply(namespace, mcpManifests); err != nil {
		return fmt.Errorf("applying MCP server manifests: %w", err)
	}
	fmt.Println("  \u2713 MCP server manifests applied")

	// Step 5: Optionally install module proxy
	if installProxy {
		fmt.Printf("Installing module proxy (esm.sh) in namespace %s...\n", proxyNamespace)
		fmt.Printf("Ensuring namespace %s...\n", proxyNamespace)
		if err := client.EnsureNamespace(proxyNamespace); err != nil {
			return fmt.Errorf("ensuring namespace %s: %w", proxyNamespace, err)
		}
		fmt.Printf("  \u2713 Namespace %s ready\n", proxyNamespace)
		if proxyStorage == "pvc" {
			fmt.Printf("  Storage: PVC (%s)\n", pvcSize)
		} else {
			fmt.Println("  Storage: emptyDir (cache lost on pod restart -- use --proxy-storage=pvc for production)")
		}
		proxyManifests := k8s.GenerateModuleProxyManifests(proxyImage, proxyNamespace, proxyStorage, pvcSize)
		if err := client.Apply(proxyNamespace, proxyManifests); err != nil {
			return fmt.Errorf("applying module proxy manifests: %w", err)
		}
		fmt.Printf("  \u2713 Module proxy installed: http://esm-sh.%s.svc.cluster.local:8080\n", proxyNamespace)
	}

	// Step 6: Wait for MCP server to be ready
	mcpEndpoint := k8s.MCPEndpointInCluster(namespace)
	fmt.Printf("Waiting for MCP server at %s (timeout: %s)...\n", mcpEndpoint, waitTimeout)
	if err := waitForMCPReady(mcpEndpoint, token, waitTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "  WARNING: MCP server health check timed out: %v\n", err)
		fmt.Fprintln(os.Stderr, "  The server may still be starting. Check with: kubectl get deploy -n "+namespace)
	} else {
		fmt.Println("  \u2713 MCP server is healthy")
	}

	// Step 7: Save endpoint + token to config
	tokenPath, err := saveMCPToken(token, namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  WARNING: Could not save token to file: %v\n", err)
		fmt.Fprintf(os.Stderr, "  Set manually: TNTC_MCP_ENDPOINT=%s TNTC_MCP_TOKEN=<check token file at ~/.tentacular/mcp-token>\n", mcpEndpoint)
	} else {
		if err := mcp.SaveConfig(mcpEndpoint, tokenPath); err != nil {
			fmt.Fprintf(os.Stderr, "  WARNING: Could not save MCP config: %v\n", err)
		} else {
			fmt.Printf("  \u2713 MCP config saved to ~/.tentacular/config.yaml\n")
		}
	}

	fmt.Printf("\n\u2713 Tentacular MCP server installed successfully\n")
	fmt.Printf("  Endpoint: %s\n", mcpEndpoint)
	fmt.Printf("  Token:    %s\n", tokenPath)
	fmt.Println("\nSubsequent tntc commands will automatically route through the MCP server.")
	fmt.Println("Run `tntc cluster check` to verify cluster readiness.")
	return nil
}

// waitForMCPReady polls the MCP server's /healthz endpoint until it responds OK
// or the timeout is reached.
func waitForMCPReady(endpoint, token string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	httpClient := &http.Client{Timeout: 5 * time.Second}

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint+"/healthz", nil)
		if err != nil {
			return err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("MCP server did not become ready within %s", timeout)
}

// saveMCPToken writes the bearer token to ~/.tentacular/mcp-token and returns the path.
func saveMCPToken(token, namespace string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}

	dir := filepath.Join(home, ".tentacular")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}

	tokenPath := filepath.Join(dir, "mcp-token")
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("writing token file: %w", err)
	}
	return tokenPath, nil
}

