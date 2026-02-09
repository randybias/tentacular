package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/randyb/pipedreamer2/pkg/builder"
	"github.com/randyb/pipedreamer2/pkg/k8s"
	"github.com/randyb/pipedreamer2/pkg/spec"
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
	cmd.Flags().String("cluster-registry", "", "In-cluster registry URL for manifest image references")
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
	registry, _ := cmd.Flags().GetString("registry")
	clusterRegistry, _ := cmd.Flags().GetString("cluster-registry")
	runtimeClass, _ := cmd.Flags().GetString("runtime-class")

	// Image tag for the manifest uses cluster-registry if provided
	imageTag := wf.Name + ":" + strings.ReplaceAll(wf.Version, ".", "-")
	if clusterRegistry != "" {
		imageTag = clusterRegistry + "/" + imageTag
	} else if registry != "" {
		imageTag = registry + "/" + imageTag
	}

	// Generate K8s manifests
	opts := builder.DeployOptions{
		RuntimeClassName: runtimeClass,
	}
	manifests := builder.GenerateK8sManifests(wf, imageTag, namespace, opts)

	fmt.Printf("Deploying %s to namespace %s...\n", wf.Name, namespace)

	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("creating k8s client: %w", err)
	}

	// Auto-provision secrets from local .secrets.yaml or .secrets/ directory
	secretManifest, err := buildSecretManifest(absDir, wf.Name, namespace)
	if err != nil {
		return fmt.Errorf("building secret manifest: %w", err)
	}
	if secretManifest != nil {
		manifests = append(manifests, *secretManifest)
	}

	if err := client.Apply(namespace, manifests); err != nil {
		return fmt.Errorf("applying manifests: %w", err)
	}

	fmt.Printf("✓ Deployed %s to %s\n", wf.Name, namespace)
	return nil
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
func buildSecretFromYAML(filePath, secretName, namespace string) (*builder.Manifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading secrets file: %w", err)
	}

	var secrets map[string]string
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("parsing secrets YAML: %w", err)
	}

	if len(secrets) == 0 {
		return nil, nil
	}

	var dataEntries []string
	for k, v := range secrets {
		dataEntries = append(dataEntries, fmt.Sprintf("  %s: %q", k, v))
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
