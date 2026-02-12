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
	"gopkg.in/yaml.v3"
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
	return cmd
}

func runDeploy(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
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

	namespace, _ := cmd.Flags().GetString("namespace")
	imageFlagValue, _ := cmd.Flags().GetString("image")
	clusterRegistry, _ := cmd.Flags().GetString("cluster-registry")
	runtimeClass, _ := cmd.Flags().GetString("runtime-class")

	// Apply config defaults: CLI flag > workflow.yaml > config file > cobra default
	cfg := LoadConfig()
	if !cmd.Flags().Changed("namespace") {
		if wf.Deployment.Namespace != "" {
			namespace = wf.Deployment.Namespace
		} else if cfg.Namespace != "" {
			namespace = cfg.Namespace
		}
	}
	if !cmd.Flags().Changed("runtime-class") && cfg.RuntimeClass != "" {
		runtimeClass = cfg.RuntimeClass
	}

	// Check for deprecated --cluster-registry flag
	if clusterRegistry != "" {
		return fmt.Errorf("--cluster-registry is deprecated; use --image instead")
	}

	// Image resolution cascade: --image flag > .tentacular/base-image.txt > tentacular-engine:latest
	imageTag := imageFlagValue
	if imageTag == "" {
		// Try reading from .tentacular/base-image.txt
		tagFilePath := ".tentacular/base-image.txt"
		if tagData, err := os.ReadFile(tagFilePath); err == nil {
			imageTag = strings.TrimSpace(string(tagData))
		}
	}
	if imageTag == "" {
		// Fallback to default
		imageTag = "tentacular-engine:latest"
	}

	// Generate ConfigMap for workflow code
	configMap, err := builder.GenerateCodeConfigMap(wf, absDir, namespace)
	if err != nil {
		return fmt.Errorf("generating ConfigMap: %w", err)
	}

	// Generate K8s manifests
	opts := builder.DeployOptions{
		RuntimeClassName: runtimeClass,
	}
	manifests := builder.GenerateK8sManifests(wf, imageTag, namespace, opts)

	// Prepend ConfigMap to manifest list
	manifests = append([]builder.Manifest{configMap}, manifests...)

	fmt.Printf("Deploying %s to namespace %s...\n", wf.Name, namespace)

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	// Build secret manifest first to validate local secrets before preflight
	secretManifest, err := buildSecretManifest(absDir, wf.Name, namespace)
	if err != nil {
		return fmt.Errorf("building secret manifest: %w", err)
	}
	hasLocalSecrets := secretManifest != nil

	// Auto-preflight checks before applying manifests
	// If local secrets exist, we'll create them — pass empty secretNames so preflight
	// doesn't check K8s for a secret we're about to provision. If no local secrets,
	// check K8s for a pre-existing secret the Deployment may reference.
	var secretNames []string
	if hasLocalSecrets {
		fmt.Printf("  ℹ Found local secrets — will provision %s-secrets\n", wf.Name)
	} else {
		secretNames = []string{wf.Name + "-secrets"}
	}
	results, err := client.PreflightCheck(namespace, false, secretNames)
	if err != nil {
		return fmt.Errorf("preflight check failed: %w", err)
	}
	if failed := evaluatePreflightResults(results, hasLocalSecrets); failed {
		return fmt.Errorf("preflight checks failed — fix the issues above and retry")
	}
	fmt.Println("  ✓ Preflight checks passed")

	if secretManifest != nil {
		manifests = append(manifests, *secretManifest)
	}

	if err := client.Apply(namespace, manifests); err != nil {
		return fmt.Errorf("applying manifests: %w", err)
	}

	// Trigger rollout restart to pick up new ConfigMap
	if err := client.RolloutRestart(namespace, wf.Name); err != nil {
		return fmt.Errorf("triggering rollout restart: %w", err)
	}
	fmt.Println("  ✓ Triggered rollout restart")

	fmt.Printf("✓ Deployed %s to %s\n", wf.Name, namespace)
	return nil
}

// evaluatePreflightResults displays preflight check results and returns true if
// any check failed. When hasLocalSecrets is false, secret-reference failures are
// downgraded to warnings since the Deployment mounts secrets with optional: true.
func evaluatePreflightResults(results []k8s.CheckResult, hasLocalSecrets bool) bool {
	failed := false
	for _, r := range results {
		if r.Passed {
			fmt.Printf("  ✓ %s\n", r.Name)
			if r.Warning != "" {
				fmt.Printf("    ⚠ %s\n", r.Warning)
			}
			continue
		}
		// When no local secrets, downgrade secret-reference failures to warnings
		if !hasLocalSecrets && r.Name == "Secret references" {
			fmt.Printf("  ⚠ %s (no local secrets — secret volume is optional)\n", r.Name)
			if r.Remediation != "" {
				fmt.Printf("    ℹ %s\n", r.Remediation)
			}
			continue
		}
		fmt.Printf("  ✗ %s\n", r.Name)
		if r.Warning != "" {
			fmt.Printf("    ⚠ %s\n", r.Warning)
		}
		if r.Remediation != "" {
			fmt.Printf("    → %s\n", r.Remediation)
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
