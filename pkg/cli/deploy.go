package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/randybias/tentacular/pkg/builder"
	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [dir]",
		Short: "Deploy to Kubernetes",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDeploy,
	}
	cmd.Flags().String("env", "", "Target environment from config (resolves context, namespace, runtime-class)")
	cmd.Flags().String("image", "", "Base engine image (default: read from .tentacular/base-image.txt or use tentacular-engine:latest)")
	cmd.Flags().String("cluster-registry", "", "DEPRECATED: Use --image instead")
	cmd.Flags().String("runtime-class", "gvisor", "RuntimeClass name (empty to disable)")
	cmd.Flags().Bool("force", false, "Skip pre-deploy live test")
	cmd.Flags().Bool("skip-live-test", false, "Skip pre-deploy live test (alias for --force)")
	cmd.Flags().Bool("verify", false, "Run workflow once after deploy to verify")
	cmd.Flags().Bool("warn", false, "Audit mode: contract violations produce warnings instead of failures")
	return cmd
}

// InternalDeployOptions controls deployment behavior when called programmatically.
type InternalDeployOptions struct {
	Namespace       string
	Image           string
	RuntimeClass    string
	ImagePullPolicy string
	Kubeconfig      string    // explicit kubeconfig file path
	Context         string    // kubeconfig context override
	StatusOut       io.Writer // writer for progress messages (nil defaults to os.Stdout)
}

// DeployResult holds the result of a deployment.
type DeployResult struct {
	WorkflowName string
	Namespace    string
	Client       *k8s.Client
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

	envName, _ := cmd.Flags().GetString("env")
	namespace, _ := cmd.Flags().GetString("namespace")
	imageFlagValue, _ := cmd.Flags().GetString("image")
	clusterRegistry, _ := cmd.Flags().GetString("cluster-registry")
	runtimeClass, _ := cmd.Flags().GetString("runtime-class")
	force, _ := cmd.Flags().GetBool("force")
	skipLiveTest, _ := cmd.Flags().GetBool("skip-live-test")
	verify, _ := cmd.Flags().GetBool("verify")

	// --skip-live-test is an alias for --force
	if skipLiveTest {
		force = true
	}

	// Check for deprecated --cluster-registry flag
	if clusterRegistry != "" {
		return fmt.Errorf("--cluster-registry is deprecated; use --image instead")
	}

	// Apply config defaults: CLI flag > workflow.yaml > config file > cobra default
	cfg := LoadConfig()

	// Resolve --env: environment config provides context, namespace, runtime-class defaults.
	// CLI flags still override environment values.
	var envContext string
	var envKubeconfig string
	if envName != "" {
		env, envErr := cfg.LoadEnvironment(envName)
		if envErr != nil {
			return fmt.Errorf("loading environment %q: %w", envName, envErr)
		}
		envContext = env.Context
		envKubeconfig = env.Kubeconfig
		if !cmd.Flags().Changed("namespace") && env.Namespace != "" {
			namespace = env.Namespace
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
	data, err := os.ReadFile(specPath)
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
			if warnMode {
				fmt.Fprintf(os.Stderr, "⚠️  WARNING: Contract validation failed with %d error(s):\n", len(contractErrs))
				for _, e := range contractErrs {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
				fmt.Fprintf(os.Stderr, "Proceeding with deploy in audit mode (use strict mode in production)\n\n")
			} else {
				fmt.Fprintf(os.Stderr, "❌ Contract validation failed with %d error(s):\n", len(contractErrs))
				for _, e := range contractErrs {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
				return fmt.Errorf("deploy aborted: contract validation failed (use --warn for audit mode)")
			}
		}
	}

	// Namespace cascade: CLI -n > --env > workflow.yaml > config > default
	if !cmd.Flags().Changed("namespace") && envName == "" {
		if wf.Deployment.Namespace != "" {
			namespace = wf.Deployment.Namespace
		} else if cfg.Namespace != "" {
			namespace = cfg.Namespace
		}
	}
	if !cmd.Flags().Changed("runtime-class") && envName == "" && cfg.RuntimeClass != "" {
		runtimeClass = cfg.RuntimeClass
	}

	// Image resolution cascade: --image flag > env.Image > <workflow>/.tentacular/base-image.txt > tentacular-engine:latest
	imageTag := imageFlagValue
	if imageTag == "" {
		tagFilePath := filepath.Join(absDir, ".tentacular", "base-image.txt")
		if tagData, err := os.ReadFile(tagFilePath); err == nil {
			imageTag = strings.TrimSpace(string(tagData))
		}
	}
	if imageTag == "" {
		imageTag = "tentacular-engine:latest"
	}

	// Determine status output writer (stderr when -o json)
	w := StatusWriter(cmd)

	// Pre-deploy live test gate: if dev environment is configured and --force is not set,
	// run a live test first to catch issues before deploying.
	if !force {
		devEnv, envErr := cfg.LoadEnvironment("dev")
		if envErr == nil && devEnv.Namespace != "" {
			fmt.Fprintln(w, "Running pre-deploy live test in dev environment...")
			liveOpts := InternalDeployOptions{
				Namespace:    devEnv.Namespace,
				Image:        imageTag,
				RuntimeClass: devEnv.RuntimeClass,
				Kubeconfig:   devEnv.Kubeconfig,
				Context:      devEnv.Context,
				StatusOut:    w,
			}
			liveResult, liveErr := deployWorkflow(absDir, liveOpts)
			if liveErr != nil {
				return fmt.Errorf("pre-deploy live test failed (use --force to skip): %w", liveErr)
			}

			// Wait for ready and run once
			liveCtx, liveCancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer liveCancel()

			if waitErr := liveResult.Client.WaitForReady(liveCtx, liveResult.Namespace, liveResult.WorkflowName); waitErr != nil {
				// Clean up before failing
				liveResult.Client.DeleteResources(liveResult.Namespace, liveResult.WorkflowName)
				return fmt.Errorf("pre-deploy live test: deployment not ready (use --force to skip): %w", waitErr)
			}

			runOutput, runErr := liveResult.Client.RunWorkflow(liveCtx, liveResult.Namespace, liveResult.WorkflowName)
			// Clean up the dev deployment
			liveResult.Client.DeleteResources(liveResult.Namespace, liveResult.WorkflowName)

			if runErr != nil {
				return fmt.Errorf("pre-deploy live test: workflow run failed (use --force to skip): %w", runErr)
			}

			var execResult map[string]interface{}
			if json.Unmarshal([]byte(runOutput), &execResult) == nil {
				if success, ok := execResult["success"].(bool); ok && !success {
					return fmt.Errorf("pre-deploy live test: workflow returned success=false (use --force to skip)")
				}
			}
			fmt.Fprintln(w, "  Pre-deploy live test passed")
		}
	}

	// Deploy
	deployOpts := InternalDeployOptions{
		Namespace:    namespace,
		Image:        imageTag,
		RuntimeClass: runtimeClass,
		Kubeconfig:   envKubeconfig,
		Context:      envContext,
		StatusOut:    w,
	}

	deployResult, err := deployWorkflow(absDir, deployOpts)
	if err != nil {
		return emitDeployResult(cmd, "fail", "deploy failed: "+err.Error(), nil, startedAt)
	}

	// Post-deploy verification
	if verify {
		fmt.Fprintln(w, "Verifying deployment...")
		verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer verifyCancel()

		if waitErr := deployResult.Client.WaitForReady(verifyCtx, deployResult.Namespace, deployResult.WorkflowName); waitErr != nil {
			return emitDeployResult(cmd, "fail", "verification: deployment not ready: "+waitErr.Error(), nil, startedAt)
		}

		runOutput, runErr := deployResult.Client.RunWorkflow(verifyCtx, deployResult.Namespace, deployResult.WorkflowName)
		if runErr != nil {
			return emitDeployResult(cmd, "fail", "verification: workflow run failed: "+runErr.Error(), nil, startedAt)
		}

		var execResult map[string]interface{}
		if json.Unmarshal([]byte(runOutput), &execResult) == nil {
			if success, ok := execResult["success"].(bool); ok && !success {
				return emitDeployResult(cmd, "fail", "verification: workflow returned success=false", execResult, startedAt)
			}
		}
		fmt.Fprintln(w, "  Verification passed")
	}

	return emitDeployResult(cmd, "pass", fmt.Sprintf("deployed %s to %s", deployResult.WorkflowName, deployResult.Namespace), nil, startedAt)
}

// emitDeployResult outputs the deploy result in the appropriate format.
func emitDeployResult(cmd *cobra.Command, status, summary string, execution interface{}, startedAt time.Time) error {
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

// deployWorkflow is the internal deployment function used by both `tntc deploy` and `tntc test --live`.
func deployWorkflow(workflowDir string, opts InternalDeployOptions) (*DeployResult, error) {
	w := opts.StatusOut
	if w == nil {
		w = os.Stdout
	}

	specPath := filepath.Join(workflowDir, "workflow.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading workflow spec: %w", err)
	}

	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		return nil, fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
	}

	// Contract preflight gate: validate contract before deploy (strict mode for internal calls)
	if wf.Contract != nil {
		contractErrs := spec.ValidateContract(wf.Contract)
		if len(contractErrs) > 0 {
			return nil, fmt.Errorf("deploy aborted: contract validation failed with %d error(s)", len(contractErrs))
		}
	}

	namespace := opts.Namespace
	imageTag := opts.Image
	runtimeClass := opts.RuntimeClass
	imagePullPolicy := opts.ImagePullPolicy

	if imageTag == "" {
		imageTag = "tentacular-engine:latest"
	}

	// Scan TypeScript node files for jsr:/npm: imports and auto-wire the module proxy.
	// This catches jsr/npm usage in code even when not yet declared in the contract.
	// Scanned deps are merged into the contract so that DeriveDenoFlags,
	// GenerateNetworkPolicy, and GenerateImportMap all pick them up automatically.
	nodesDir := filepath.Join(workflowDir, "nodes")
	if scanned, scanErr := k8s.ScanNodeImports(nodesDir); scanErr == nil && len(scanned) > 0 {
		if wf.Contract == nil {
			wf.Contract = &spec.Contract{Dependencies: make(map[string]spec.Dependency)}
		}
		if wf.Contract.Dependencies == nil {
			wf.Contract.Dependencies = make(map[string]spec.Dependency)
		}
		for _, sd := range scanned {
			// Check if already in contract by protocol+host
			alreadyDeclared := false
			for _, d := range wf.Contract.Dependencies {
				if d.Protocol == sd.Protocol && d.Host == sd.Host {
					alreadyDeclared = true
					break
				}
			}
			if !alreadyDeclared {
				// Synthetic key: won't collide with user-defined keys
				key := fmt.Sprintf("__scanned__%s__%s", sd.Protocol, strings.ReplaceAll(sd.Host, "/", "_"))
				wf.Contract.Dependencies[key] = sd
				versionHint := ""
				if sd.Version != "" {
					versionHint = "@" + sd.Version
				}
				fmt.Fprintf(w, "  Module proxy: auto-detected %s:%s%s from TypeScript (declare in contract to pin version)\n",
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
		fmt.Fprintf(w, "  Detected kind cluster '%s', adjusted: no gVisor, imagePullPolicy=IfNotPresent\n", kindInfo.ClusterName)
		runtimeClass = ""
		if imagePullPolicy == "" {
			imagePullPolicy = "IfNotPresent"
		}
	}

	// Resolve module proxy URL before manifest generation so the Deployment's
	// pre-warm initContainer and --allow-import flags are wired correctly.
	cfg := LoadConfig()
	proxyURL := k8s.DefaultModuleProxyURL
	if cfg.ModuleProxy.Namespace != "" {
		proxyURL = fmt.Sprintf("http://esm-sh.%s.svc.cluster.local:8080", cfg.ModuleProxy.Namespace)
	}

	// Generate K8s manifests
	buildOpts := builder.DeployOptions{
		RuntimeClassName: runtimeClass,
		ImagePullPolicy:  imagePullPolicy,
		ModuleProxyURL:   proxyURL, // triggers pre-warm initContainer when jsr/npm deps present
	}
	manifests := builder.GenerateK8sManifests(wf, imageTag, namespace, buildOpts)

	// Prepend ConfigMap to manifest list
	manifests = append([]builder.Manifest{configMap}, manifests...)

	// Add NetworkPolicy if contract present
	proxyNamespace := cfg.ModuleProxy.Namespace
	if netpol := k8s.GenerateNetworkPolicy(wf, namespace, proxyNamespace); netpol != nil {
		manifests = append(manifests, *netpol)
	}

	// Add import map ConfigMap when workflow has jsr/npm module proxy dependencies
	if k8s.HasModuleProxyDeps(wf) {
		if importMap := k8s.GenerateImportMapWithNamespace(wf, namespace, proxyURL); importMap != nil {
			manifests = append(manifests, *importMap)
			fmt.Fprintf(w, "  Module proxy: import map generated (%d jsr/npm deps)\n", countModuleProxyDeps(wf))
		}
	}

	fmt.Fprintf(w, "Deploying %s to namespace %s...\n", wf.Name, namespace)

	// Create K8s client (with optional kubeconfig file and/or context override)
	var client *k8s.Client
	if opts.Kubeconfig != "" {
		client, err = k8s.NewClientFromConfig(opts.Kubeconfig, opts.Context)
	} else if opts.Context != "" {
		client, err = k8s.NewClientWithContext(opts.Context)
	} else {
		client, err = k8s.NewClient()
	}
	if err != nil {
		return nil, fmt.Errorf("creating k8s client: %w", err)
	}

	// Build secret manifest first to validate local secrets before preflight
	secretManifest, err := buildSecretManifest(workflowDir, wf.Name, namespace)
	if err != nil {
		return nil, fmt.Errorf("building secret manifest: %w", err)
	}
	hasLocalSecrets := secretManifest != nil

	// Auto-preflight checks
	var secretNames []string
	if hasLocalSecrets {
		fmt.Fprintf(w, "  Found local secrets -- will provision %s-secrets\n", wf.Name)
	} else {
		secretNames = []string{wf.Name + "-secrets"}
	}
	results, err := client.PreflightCheck(namespace, false, secretNames)
	if err != nil {
		return nil, fmt.Errorf("preflight check failed: %w", err)
	}
	if failed := evaluatePreflightResults(w, results, hasLocalSecrets); failed {
		return nil, fmt.Errorf("preflight checks failed -- fix the issues above and retry")
	}
	fmt.Fprintln(w, "  Preflight checks passed")

	if secretManifest != nil {
		manifests = append(manifests, *secretManifest)
	}

	if err := client.ApplyWithStatus(w, namespace, manifests); err != nil {
		return nil, fmt.Errorf("applying manifests: %w", err)
	}

	// Only rollout restart on updates (skip for fresh deployments to avoid
	// double-pod creation and the associated readiness race condition).
	if client.LastApplyHadUpdates() {
		if err := client.RolloutRestart(namespace, wf.Name); err != nil {
			return nil, fmt.Errorf("triggering rollout restart: %w", err)
		}
		fmt.Fprintln(w, "  Triggered rollout restart")
	}

	fmt.Fprintf(w, "Deployed %s to %s\n", wf.Name, namespace)
	return &DeployResult{
		WorkflowName: wf.Name,
		Namespace:    namespace,
		Client:       client,
	}, nil
}

// evaluatePreflightResults displays preflight check results and returns true if
// any check failed. When hasLocalSecrets is false, secret-reference failures are
// downgraded to warnings since the Deployment mounts secrets with optional: true.
func evaluatePreflightResults(w io.Writer, results []k8s.CheckResult, hasLocalSecrets bool) bool {
	failed := false
	for _, r := range results {
		if r.Passed {
			fmt.Fprintf(w, "  ✓ %s\n", r.Name)
			if r.Warning != "" {
				fmt.Fprintf(w, "    ⚠ %s\n", r.Warning)
			}
			continue
		}
		// When no local secrets, downgrade secret-reference failures to warnings
		if !hasLocalSecrets && r.Name == "Secret references" {
			fmt.Fprintf(w, "  ⚠ %s (no local secrets — secret volume is optional)\n", r.Name)
			if r.Remediation != "" {
				fmt.Fprintf(w, "    ℹ %s\n", r.Remediation)
			}
			continue
		}
		fmt.Fprintf(w, "  ✗ %s\n", r.Name)
		if r.Warning != "" {
			fmt.Fprintf(w, "    ⚠ %s\n", r.Warning)
		}
		if r.Remediation != "" {
			fmt.Fprintf(w, "    → %s\n", r.Remediation)
		}
		failed = true
	}
	return failed
}

// buildSecretManifest creates a K8s Secret manifest from local secrets.
// Cascade: .secrets/ directory → .secrets.yaml file
// Returns nil if no local secrets found.
func buildSecretManifest(workflowDir, name, namespace string) (*builder.Manifest, error) {
	secretName := name + "-secrets"

	// Try .secrets/ directory first
	secretsDir := filepath.Join(workflowDir, ".secrets")
	if info, err := os.Stat(secretsDir); err == nil && info.IsDir() {
		return buildSecretFromDir(secretsDir, secretName, namespace)
	}

	// Fall back to .secrets.yaml
	secretsFile := filepath.Join(workflowDir, ".secrets.yaml")
	if _, err := os.Stat(secretsFile); err == nil {
		return buildSecretFromYAML(secretsFile, secretName, namespace)
	}

	// No local secrets found — skip
	return nil, nil
}

// buildSecretFromDir creates a K8s Secret from a directory of files.
func buildSecretFromDir(dirPath, secretName, namespace string) (*builder.Manifest, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading secrets directory: %w", err)
	}

	var dataEntries []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dirPath, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading secret file %s: %w", entry.Name(), err)
		}
		// Use stringData so we don't need to base64-encode
		dataEntries = append(dataEntries, fmt.Sprintf("  %s: %q", entry.Name(), strings.TrimSpace(string(content))))
	}

	if len(dataEntries) == 0 {
		return nil, nil
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

// buildSecretFromYAML creates a K8s Secret from a .secrets.yaml file.
// Supports nested YAML maps: string values are used as-is, nested maps are
// JSON-serialized (matching the engine's loadSecretsFromDir JSON parsing).
func buildSecretFromYAML(filePath, secretName, namespace string) (*builder.Manifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading secrets file: %w", err)
	}

	var secrets map[string]interface{}
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("parsing secrets YAML: %w", err)
	}

	if len(secrets) == 0 {
		return nil, nil
	}

	// Resolve $shared.<name> references
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
			jsonBytes, err := json.Marshal(val)
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
