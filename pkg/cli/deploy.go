package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/builder"
	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/randybias/tentacular/pkg/mcp"
	"github.com/randybias/tentacular/pkg/spec"
)

func NewDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [dir]",
		Short: "Deploy to Kubernetes",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDeploy,
	}
	cmd.Flags().String("image", "", "Base engine image (default: read from .tentacular/base-image.txt or use tentacular-engine:latest)")
	cmd.Flags().String("cluster-registry", "", "DEPRECATED: Use --image instead")
	cmd.Flags().String("runtime-class", "gvisor", "RuntimeClass name (empty to disable)")
	cmd.Flags().Bool("force", false, "Skip pre-deploy live test")
	cmd.Flags().Bool("skip-live-test", false, "Skip pre-deploy live test (alias for --force)")
	cmd.Flags().Bool("verify", false, "Run workflow once after deploy to verify")
	cmd.Flags().Bool("warn", false, "Audit mode: contract violations produce warnings instead of failures")
	cmd.Flags().String("group", "", "Set group ownership for the workflow (e.g. platform-team)")
	cmd.Flags().String("share", "", "Set permissions mode preset (e.g. group-edit, private, public-read)")
	return cmd
}

// InternalDeployOptions controls deployment behavior when called programmatically.
type InternalDeployOptions struct {
	StatusOut       io.Writer // writer for progress messages (nil defaults to os.Stdout)
	Namespace       string
	Image           string
	RuntimeClass    string
	ImagePullPolicy string
	Kubeconfig      string // explicit kubeconfig file path (bootstrap-only)
	Context         string // kubeconfig context override (bootstrap-only)
	Group           string // optional group ownership for authz annotation stamping
	Share           string // optional mode preset (e.g. "group-edit", "private")
}

// DeployResult holds the result of a deployment.
type DeployResult struct {
	MCPClient    *mcp.Client
	WorkflowName string
	Namespace    string
}

func runDeploy(cmd *cobra.Command, args []string) error {
	startedAt := time.Now().UTC()

	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	envName := flagString(cmd, "env")
	imageFlagValue, _ := cmd.Flags().GetString("image")
	clusterRegistry, _ := cmd.Flags().GetString("cluster-registry")
	runtimeClass, _ := cmd.Flags().GetString("runtime-class")
	force, _ := cmd.Flags().GetBool("force")
	skipLiveTest, _ := cmd.Flags().GetBool("skip-live-test")
	verify, _ := cmd.Flags().GetBool("verify")
	group, _ := cmd.Flags().GetString("group")
	share, _ := cmd.Flags().GetString("share")

	// --skip-live-test is an alias for --force
	if skipLiveTest {
		force = true
	}

	// Check for deprecated --cluster-registry flag
	if clusterRegistry != "" {
		return errors.New("--cluster-registry is deprecated; use --image instead")
	}

	// Apply config defaults: workflow.yaml > env config > config file
	cfg := LoadConfig()

	// Resolve --env: environment config provides namespace, runtime-class defaults.
	if envName != "" {
		env, envErr := cfg.LoadEnvironment(envName)
		if envErr != nil {
			return fmt.Errorf("loading environment %q: %w", envName, envErr)
		}
		if !cmd.Flags().Changed("runtime-class") && env.RuntimeClass != "" {
			runtimeClass = env.RuntimeClass
		}
		if !cmd.Flags().Changed("runtime-class") && env.RuntimeClass == "" {
			runtimeClass = ""
		}
		if !cmd.Flags().Changed("image") && env.Image != "" {
			imageFlagValue = env.Image
		}
	}

	specPath := filepath.Join(absDir, "workflow.yaml")
	data, err := os.ReadFile(specPath) //nolint:gosec // specPath is derived from user's workflow directory
	if err != nil {
		return fmt.Errorf("reading workflow spec: %w", err)
	}
	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		return fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
	}

	// Contract preflight gate: validate contract before deploy
	warnMode, _ := cmd.Flags().GetBool("warn")
	if wf.Contract != nil {
		contractErrs := spec.ValidateContract(wf.Contract)
		if len(contractErrs) > 0 {
			if !warnMode {
				fmt.Fprintf(os.Stderr, "Contract validation failed with %d error(s):\n", len(contractErrs))
				for _, e := range contractErrs {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
				return errors.New("deploy aborted: contract validation failed (use --warn for audit mode)")
			}
			fmt.Fprintf(os.Stderr, "WARNING: Contract validation failed with %d error(s):\n", len(contractErrs))
			for _, e := range contractErrs {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
			fmt.Fprintf(os.Stderr, "Proceeding with deploy in audit mode (use strict mode in production)\n\n")
		}
	}

	// Namespace cascade: workflow.yaml > env config > global config > "default"
	namespace := resolveNamespace(cmd, absDir)
	if !cmd.Flags().Changed("runtime-class") && envName == "" && cfg.RuntimeClass != "" {
		runtimeClass = cfg.RuntimeClass
	}

	// Image resolution cascade: --image flag > env.Image > <workflow>/.tentacular/base-image.txt > tentacular-engine:latest
	imageTag := imageFlagValue
	if imageTag == "" {
		tagFilePath := filepath.Join(absDir, ".tentacular", "base-image.txt")
		if tagData, readErr := os.ReadFile(tagFilePath); readErr == nil { //nolint:gosec // tagFilePath is derived from workflow directory
			imageTag = strings.TrimSpace(string(tagData))
		}
	}
	if imageTag == "" {
		imageTag = "tentacular-engine:latest"
	}

	// Determine status output writer (stderr when -o json)
	w := StatusWriter(cmd)

	// Resolve MCP client once; it's used for apply, pre-deploy test, and verify.
	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return emitDeployResult(cmd, "fail", err.Error(), nil, startedAt)
	}

	// Pre-deploy live test gate: if dev environment is configured and --force is not set,
	// run a live test first to catch issues before deploying.
	if !force {
		devEnv, envErr := cfg.LoadEnvironment("dev")
		if envErr == nil && devEnv.Namespace != "" {
			_, _ = fmt.Fprintln(w, "Running pre-deploy live test in dev environment...")
			liveOpts := InternalDeployOptions{
				Namespace:    devEnv.Namespace,
				Image:        imageTag,
				RuntimeClass: devEnv.RuntimeClass,
				StatusOut:    w,
			}
			liveResult, liveErr := deployWorkflow(absDir, liveOpts, mcpClient)
			if liveErr != nil {
				return fmt.Errorf("pre-deploy live test failed (use --force to skip): %w", liveErr)
			}

			runResult, runErr := liveResult.MCPClient.WfRun(cmd.Context(), liveResult.Namespace, liveResult.WorkflowName, nil, 120)
			// Clean up the dev deployment regardless of run outcome
			_, _ = liveResult.MCPClient.WfRemove(cmd.Context(), liveResult.Namespace, liveResult.WorkflowName)

			if runErr != nil {
				return fmt.Errorf("pre-deploy live test: workflow run failed (use --force to skip): %w", runErr)
			}

			var execResult map[string]any
			if json.Unmarshal(runResult.Output, &execResult) == nil {
				if success, ok := execResult["success"].(bool); ok && !success {
					return errors.New("pre-deploy live test: workflow returned success=false (use --force to skip)")
				}
			}
			_, _ = fmt.Fprintln(w, "  Pre-deploy live test passed")
		}
	}

	// Deploy
	deployOpts := InternalDeployOptions{
		Namespace:    namespace,
		Image:        imageTag,
		RuntimeClass: runtimeClass,
		StatusOut:    w,
		Group:        group,
		Share:        share,
	}

	deployResult, err := deployWorkflow(absDir, deployOpts, mcpClient)
	if err != nil {
		return emitDeployResult(cmd, "fail", "deploy failed: "+err.Error(), nil, startedAt)
	}

	// Post-deploy verification
	if verify {
		_, _ = fmt.Fprintln(w, "Verifying deployment...")
		runResult, runErr := deployResult.MCPClient.WfRun(cmd.Context(), deployResult.Namespace, deployResult.WorkflowName, nil, 120)
		if runErr != nil {
			return emitDeployResult(cmd, "fail", "verification: workflow run failed: "+runErr.Error(), nil, startedAt)
		}

		var execResult map[string]any
		if json.Unmarshal(runResult.Output, &execResult) == nil {
			if success, ok := execResult["success"].(bool); ok && !success {
				return emitDeployResult(cmd, "fail", "verification: workflow returned success=false", execResult, startedAt)
			}
		}
		_, _ = fmt.Fprintln(w, "  Verification passed")
	}

	return emitDeployResult(cmd, "pass", fmt.Sprintf("deployed %s to %s", deployResult.WorkflowName, deployResult.Namespace), nil, startedAt)
}

// emitDeployResult outputs the deploy result in the appropriate format.
func emitDeployResult(cmd *cobra.Command, status, summary string, execution any, startedAt time.Time) error {
	result := CommandResult{
		Version: "1",
		Command: "deploy",
		Status:  status,
		Summary: summary,
		Hints:   []string{},
		Timing: TimingInfo{
			StartedAt:  startedAt.Format(time.RFC3339),
			DurationMs: time.Since(startedAt).Milliseconds(),
		},
		Execution: execution,
	}

	if status == "fail" {
		result.Hints = append(result.Hints, "use --force to skip pre-deploy live test")
		result.Hints = append(result.Hints, "check deployment logs with: tntc logs <workflow-name>")
	}

	if err := EmitResult(cmd, result, os.Stdout); err != nil {
		return err
	}

	if status == "fail" {
		return fmt.Errorf("%s", summary)
	}
	return nil
}

// buildManifests generates all K8s manifests locally from the workflow spec.
// This is the local/pure-Go phase of deployment — no K8s or MCP calls.
func buildManifests(workflowDir string, wf *spec.Workflow, opts InternalDeployOptions) ([]builder.Manifest, error) {
	w := opts.StatusOut
	if w == nil {
		w = os.Stdout
	}

	namespace := opts.Namespace
	imageTag := opts.Image
	runtimeClass := opts.RuntimeClass
	imagePullPolicy := opts.ImagePullPolicy

	if imageTag == "" {
		imageTag = "tentacular-engine:latest"
	}

	// Scan TypeScript node files for jsr:/npm: imports and auto-wire the module proxy.
	nodesDir := filepath.Join(workflowDir, "nodes")
	if scanned, scanErr := k8s.ScanNodeImports(nodesDir); scanErr == nil && len(scanned) > 0 {
		if wf.Contract == nil {
			wf.Contract = &spec.Contract{Dependencies: make(map[string]spec.Dependency)}
		}
		if wf.Contract.Dependencies == nil {
			wf.Contract.Dependencies = make(map[string]spec.Dependency)
		}
		for _, sd := range scanned {
			alreadyDeclared := false
			for _, d := range wf.Contract.Dependencies {
				if d.Protocol == sd.Protocol && d.Host == sd.Host {
					alreadyDeclared = true
					break
				}
			}
			if !alreadyDeclared {
				key := fmt.Sprintf("__scanned__%s__%s", sd.Protocol, strings.ReplaceAll(sd.Host, "/", "_"))
				wf.Contract.Dependencies[key] = sd
				versionHint := ""
				if sd.Version != "" {
					versionHint = "@" + sd.Version
				}
				_, _ = fmt.Fprintf(w, "  Module proxy: auto-detected %s:%s%s from TypeScript (declare in contract to pin version)\n",
					sd.Protocol, sd.Host, versionHint)
			}
		}
	}

	// Generate ConfigMap for workflow code
	configMap, err := builder.GenerateCodeConfigMap(wf, workflowDir, namespace)
	if err != nil {
		return nil, fmt.Errorf("generating ConfigMap: %w", err)
	}

	// Detect kind cluster and adjust defaults
	kindInfo, _ := k8s.DetectKindCluster()
	if kindInfo != nil && kindInfo.IsKind {
		_, _ = fmt.Fprintf(w, "  Detected kind cluster '%s', adjusted: no gVisor, imagePullPolicy=IfNotPresent\n", kindInfo.ClusterName)
		runtimeClass = ""
		if imagePullPolicy == "" {
			imagePullPolicy = "IfNotPresent"
		}
	}

	// Resolve module proxy URL
	cfg := LoadConfig()
	proxyURL := k8s.DefaultModuleProxyURL
	if cfg.ModuleProxy.Namespace != "" {
		proxyURL = fmt.Sprintf("http://esm-sh.%s.svc.cluster.local:8080", cfg.ModuleProxy.Namespace)
	}

	// Generate K8s manifests
	buildOpts := builder.DeployOptions{
		RuntimeClassName: runtimeClass,
		ImagePullPolicy:  imagePullPolicy,
		ModuleProxyURL:   proxyURL,
	}
	manifests := builder.GenerateK8sManifests(wf, imageTag, namespace, buildOpts)
	manifests = append([]builder.Manifest{configMap}, manifests...)

	// Add NetworkPolicy if contract present
	proxyNamespace := cfg.ModuleProxy.Namespace
	if netpol := k8s.GenerateNetworkPolicy(wf, namespace, proxyNamespace); netpol != nil {
		manifests = append(manifests, *netpol)
	}

	// Always generate import map — engine jsr: deps must route through the module proxy.
	// Workflow namespaces cannot reach external registries directly.
	if importMap := k8s.GenerateImportMapWithNamespace(wf, namespace, proxyURL); importMap != nil {
		manifests = append(manifests, *importMap)
		wfDeps := countModuleProxyDeps(wf)
		if wfDeps > 0 {
			_, _ = fmt.Fprintf(w, "  Module proxy: import map generated (%d jsr/npm workflow deps + engine deps)\n", wfDeps)
		} else {
			_, _ = fmt.Fprintf(w, "  Module proxy: import map generated (engine deps)\n")
		}
	}

	// Build secret manifest from local secrets
	secretManifest, err := buildSecretManifest(workflowDir, wf.Name, namespace)
	if err != nil {
		return nil, fmt.Errorf("building secret manifest: %w", err)
	}
	if secretManifest != nil {
		_, _ = fmt.Fprintf(w, "  Found local secrets -- will provision %s-secrets\n", wf.Name)
		manifests = append(manifests, *secretManifest)
	}

	return manifests, nil
}

// deployWorkflow builds manifests locally and applies them via MCP.
// Used by both `tntc deploy` and `tntc test --live`.
func deployWorkflow(workflowDir string, opts InternalDeployOptions, mcpClient *mcp.Client) (*DeployResult, error) {
	w := opts.StatusOut
	if w == nil {
		w = os.Stdout
	}

	specPath := filepath.Join(workflowDir, "workflow.yaml")
	data, err := os.ReadFile(specPath) //nolint:gosec // specPath is derived from workflow directory
	if err != nil {
		return nil, fmt.Errorf("reading workflow spec: %w", err)
	}

	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		return nil, fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
	}

	// Contract preflight gate (strict mode for internal calls)
	if wf.Contract != nil {
		contractErrs := spec.ValidateContract(wf.Contract)
		if len(contractErrs) > 0 {
			return nil, fmt.Errorf("deploy aborted: contract validation failed with %d error(s)", len(contractErrs))
		}
	}

	// Phase 1: Build manifests locally
	manifests, err := buildManifests(workflowDir, wf, opts)
	if err != nil {
		return nil, err
	}

	_, _ = fmt.Fprintf(w, "Deploying %s to namespace %s...\n", wf.Name, opts.Namespace)

	// Phase 2: Convert manifests to map[string]any for MCP transport
	mcpManifests := make([]map[string]any, 0, len(manifests))
	for _, m := range manifests {
		var obj map[string]any
		if unmarshalErr := yaml.Unmarshal([]byte(m.Content), &obj); unmarshalErr != nil {
			return nil, fmt.Errorf("serializing manifest %s/%s: %w", m.Kind, m.Name, unmarshalErr)
		}
		mcpManifests = append(mcpManifests, obj)
	}

	// Phase 3: Apply via MCP
	applyResult, err := mcpClient.WfApplyWithAuthz(context.Background(), opts.Namespace, wf.Name, mcpManifests, opts.Group, opts.Share)
	if err != nil {
		if isAuthzError(err) {
			return nil, fmt.Errorf("permission denied: %w\n  hint: use 'tntc permissions get %s %s' to view current ownership", err, opts.Namespace, wf.Name)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return nil, fmt.Errorf("applying via MCP: %w\n  hint: %s", err, hint)
		}
		return nil, fmt.Errorf("applying via MCP: %w", err)
	}

	for _, applied := range applyResult.Applied {
		_, _ = fmt.Fprintf(w, "  applied %s\n", applied)
	}

	if applyResult.Updated > 0 {
		_, _ = fmt.Fprintln(w, "  Triggered rollout restart")
	}

	_, _ = fmt.Fprintf(w, "Deployed %s to %s\n", wf.Name, opts.Namespace)
	return &DeployResult{
		WorkflowName: wf.Name,
		Namespace:    opts.Namespace,
		MCPClient:    mcpClient,
	}, nil
}

// buildSecretManifest creates a K8s Secret manifest from a .secrets.yaml file.
// All secret values must use $shared.<name> references pointing to the repo root
// .secrets/ directory. Direct secret values are not supported.
// Returns nil if no .secrets.yaml found.
func buildSecretManifest(workflowDir, name, namespace string) (*builder.Manifest, error) {
	secretName := name + "-secrets"

	secretsFile := filepath.Join(workflowDir, ".secrets.yaml")
	if _, err := os.Stat(secretsFile); err != nil {
		return nil, nil //nolint:nilerr // no .secrets.yaml file means no secret manifest needed
	}

	return buildSecretFromYAML(secretsFile, secretName, namespace)
}

// buildSecretFromYAML creates a K8s Secret from a .secrets.yaml file.
// All values must be $shared.<name> references -- direct values are rejected.
func buildSecretFromYAML(filePath, secretName, namespace string) (*builder.Manifest, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is derived from workflow directory
	if err != nil {
		return nil, fmt.Errorf("reading secrets file: %w", err)
	}

	var secrets map[string]any
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("parsing secrets YAML: %w", err)
	}

	if len(secrets) == 0 {
		return nil, nil
	}

	// Validate all values are $shared.<name> references
	for k, v := range secrets {
		s, ok := v.(string)
		if !ok || !strings.HasPrefix(s, "$shared.") {
			return nil, fmt.Errorf("secret %q has a direct value; all secrets must use $shared.<name> references (e.g. $shared.%s)", k, k)
		}
	}

	// Resolve $shared.<name> references from repo root .secrets/ directory
	workflowDir := filepath.Dir(filePath)
	if err := resolveSharedSecrets(secrets, workflowDir); err != nil {
		return nil, fmt.Errorf("resolving shared secrets: %w", err)
	}

	var dataEntries []string
	for k, v := range secrets {
		switch val := v.(type) {
		case string:
			dataEntries = append(dataEntries, fmt.Sprintf("  %s: %q", k, val))
		default:
			_ = val
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("serializing secret %q to JSON: %w", k, err)
			}
			dataEntries = append(dataEntries, fmt.Sprintf("  %s: %q", k, string(jsonBytes)))
		}
	}

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
%s
`, secretName, namespace, strings.Join(dataEntries, "\n"))

	return &builder.Manifest{
		Kind: "Secret", Name: secretName, Content: manifest,
	}, nil
}

// countModuleProxyDeps returns the number of jsr/npm dependencies in the workflow contract.
func countModuleProxyDeps(wf *spec.Workflow) int {
	if wf.Contract == nil {
		return 0
	}
	n := 0
	for _, dep := range wf.Contract.Dependencies {
		if dep.Protocol == "jsr" || dep.Protocol == "npm" {
			n++
		}
	}
	return n
}
