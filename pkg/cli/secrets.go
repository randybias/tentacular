package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewSecretsCmd creates the "secrets" subcommand with check and init subcommands.
func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage workflow secrets",
	}
	cmd.AddCommand(newSecretsCheckCmd())
	cmd.AddCommand(newSecretsInitCmd())
	return cmd
}

func newSecretsCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [dir]",
		Short: "Check secrets provisioning against node requirements",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSecretsCheck,
	}
}

func newSecretsInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Initialize secrets from .secrets.yaml.example",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSecretsInit,
	}
	cmd.Flags().Bool("force", false, "Overwrite existing .secrets.yaml")
	cmd.Flags().Bool("shared", false, "Initialize shared .secrets/ directory from .secrets.yaml.example")
	return cmd
}

// secretsPattern matches ctx.secrets?.XXX?.YYY or ctx.secrets.XXX
var secretsPattern = regexp.MustCompile(`ctx\.secrets\??\.\s*(\w+)`)

func runSecretsCheck(cmd *cobra.Command, args []string) error {
	dir := resolveDir(args)

	// Scan node source files for secret references
	required, err := scanRequiredSecrets(dir)
	if err != nil {
		return err
	}

	if len(required) == 0 {
		fmt.Println("No secret references found in node source files.")
		return nil
	}

	// Read provisioned secrets (local + shared)
	provisioned, provSource := readProvisionedSecrets(dir)

	// Also check shared secrets for any $shared.<name> references in .secrets.yaml
	sharedStatus := checkSharedSecrets(dir)

	// Compare
	wfName := filepath.Base(dir)
	fmt.Printf("Secrets check for %s:\n", wfName)

	allProvisioned := true
	missing := 0
	keys := make([]string, 0, len(required))
	for k := range required {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		if status, ok := sharedStatus[name]; ok {
			// Secret is referenced via $shared
			if status == "ok" {
				fmt.Printf("  %s  provisioned (shared)\n", name)
			} else {
				fmt.Printf("  %s  missing (%s)\n", name, status)
				allProvisioned = false
				missing++
			}
		} else if _, ok := provisioned[name]; ok {
			fmt.Printf("  %s  provisioned (%s)\n", name, provSource)
		} else {
			fmt.Printf("  %s  missing\n", name)
			allProvisioned = false
			missing++
		}
	}

	if allProvisioned {
		fmt.Printf("  All %d required secret(s) provisioned.\n", len(required))
	} else {
		fmt.Printf("  %d of %d required secret(s) missing.\n", missing, len(required))
		fmt.Printf("  Run: tntc secrets init %s\n", dir)
	}

	return nil
}

// checkSharedSecrets reads .secrets.yaml for $shared.<name> references and
// verifies each referenced shared secret exists and is non-empty.
// Returns a map of service name -> "ok" | "empty" | "missing".
func checkSharedSecrets(workflowDir string) map[string]string {
	result := make(map[string]string)

	secretsFile := filepath.Join(workflowDir, ".secrets.yaml")
	data, err := os.ReadFile(secretsFile)
	if err != nil {
		return result
	}

	var secrets map[string]interface{}
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		return result
	}

	cfg := LoadConfig()
	var repoRoot string
	if cfg.Workspace != "" {
		repoRoot = expandHome(cfg.Workspace)
	} else {
		repoRoot = findRepoRoot(workflowDir)
		if repoRoot == "" {
			return result
		}
	}
	sharedDir := filepath.Join(repoRoot, ".secrets")

	for serviceName, val := range secrets {
		strVal, ok := val.(string)
		if !ok || !strings.HasPrefix(strVal, "$shared.") {
			continue
		}
		sharedName := strings.TrimPrefix(strVal, "$shared.")
		sharedPath := filepath.Clean(filepath.Join(sharedDir, sharedName))
		// Path traversal guard
		if !strings.HasPrefix(sharedPath, filepath.Clean(sharedDir)+string(filepath.Separator)) {
			result[serviceName] = "invalid path"
			continue
		}
		info, err := os.Stat(sharedPath)
		if err != nil {
			result[serviceName] = "missing from shared"
			continue
		}
		if info.Size() == 0 {
			result[serviceName] = "empty in shared (edit " + sharedPath + ")"
			continue
		}
		result[serviceName] = "ok"
	}

	return result
}

func runSecretsInit(cmd *cobra.Command, args []string) error {
	dir := resolveDir(args)
	force, _ := cmd.Flags().GetBool("force")
	shared, _ := cmd.Flags().GetBool("shared")

	if shared {
		return runSecretsInitShared(dir, force)
	}

	src := filepath.Join(dir, ".secrets.yaml.example")
	dst := filepath.Join(dir, ".secrets.yaml")

	if !force {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf(".secrets.yaml already exists (use --force to overwrite)")
		}
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("no .secrets.yaml.example found -- create one first")
	}

	// Uncomment the example (remove leading "# " from each line)
	lines := strings.Split(string(data), "\n")
	var uncommented []string
	for _, line := range lines {
		uncommented = append(uncommented, strings.TrimPrefix(line, "# "))
	}

	if err := os.WriteFile(dst, []byte(strings.Join(uncommented, "\n")), 0o644); err != nil {
		return fmt.Errorf("writing .secrets.yaml: %w", err)
	}

	fmt.Printf("Created %s from example template.\n", dst)
	fmt.Println("Edit the file to add your actual secret values.")
	return nil
}

// runSecretsInitShared creates the .secrets/ directory with placeholder files
// for each key in .secrets.yaml.example at the given directory.
func runSecretsInitShared(dir string, force bool) error {
	src := filepath.Join(dir, ".secrets.yaml.example")
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("no .secrets.yaml.example found at %s -- create one first", dir)
	}

	// Parse the example to get secret names (keys are service names)
	var example map[string]interface{}
	if err := yaml.Unmarshal(data, &example); err != nil {
		return fmt.Errorf("parsing .secrets.yaml.example: %w", err)
	}

	sharedDir := filepath.Join(dir, ".secrets")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return fmt.Errorf("creating .secrets/ directory: %w", err)
	}

	var created, skipped []string
	for name := range example {
		dest := filepath.Join(sharedDir, name)
		if !force {
			if _, err := os.Stat(dest); err == nil {
				skipped = append(skipped, name)
				continue
			}
		}
		if err := os.WriteFile(dest, []byte(""), 0o600); err != nil {
			return fmt.Errorf("creating .secrets/%s: %w", name, err)
		}
		created = append(created, name)
	}

	sort.Strings(created)
	sort.Strings(skipped)

	for _, name := range created {
		fmt.Printf("  created .secrets/%s\n", name)
	}
	for _, name := range skipped {
		fmt.Printf("  skipped .secrets/%s (already exists; use --force to overwrite)\n", name)
	}

	if len(created) > 0 {
		fmt.Printf("\nEdit the files in %s with actual secret values.\n", sharedDir)
	}
	return nil
}

func resolveDir(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "."
}

// scanRequiredSecrets reads all nodes/*.ts files and workflow.yaml contract
// dependencies to extract required secret service names.
func scanRequiredSecrets(workflowDir string) (map[string]bool, error) {
	required := make(map[string]bool)

	// Scan node TypeScript files for ctx.secrets?.XXX patterns
	nodesDir := filepath.Join(workflowDir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading nodes directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ts") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(nodesDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		matches := secretsPattern.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			required[m[1]] = true
		}
	}

	// Also scan workflow.yaml contract dependencies for auth.secret fields.
	wfPath := filepath.Join(workflowDir, "workflow.yaml")
	if data, err := os.ReadFile(wfPath); err == nil {
		contractSecrets, scanErr := scanContractSecrets(data)
		if scanErr == nil {
			for k := range contractSecrets {
				required[k] = true
			}
		}
	}

	return required, nil
}

// contractAuthStub is a minimal struct for YAML-parsing auth.secret from workflow.yaml.
// Uses a lenient parse (not spec.Parse) so partial/invalid workflow YAML is tolerated.
type contractAuthStub struct {
	Contract *struct {
		Dependencies map[string]*struct {
			Auth *struct {
				Secret string `yaml:"secret"`
			} `yaml:"auth"`
		} `yaml:"dependencies"`
	} `yaml:"contract"`
}

// scanContractSecrets parses workflow YAML and extracts the service name from each
// contract dependency's auth.secret field. Secret names are in "service.key" format;
// only the service name (the part before the first dot) is returned, since that is the
// top-level key in .secrets.yaml.
func scanContractSecrets(yamlContent []byte) (map[string]bool, error) {
	required := make(map[string]bool)

	var stub contractAuthStub
	if err := yaml.Unmarshal(yamlContent, &stub); err != nil {
		return nil, fmt.Errorf("parsing workflow YAML: %w", err)
	}

	if stub.Contract == nil {
		return required, nil
	}

	for _, dep := range stub.Contract.Dependencies {
		if dep == nil || dep.Auth == nil || dep.Auth.Secret == "" {
			continue
		}
		parts := strings.SplitN(dep.Auth.Secret, ".", 2)
		required[parts[0]] = true
	}

	return required, nil
}

// readProvisionedSecrets reads local secrets from .secrets.yaml or .secrets/ directory.
// Also checks shared secrets at repo root.
func readProvisionedSecrets(workflowDir string) (map[string]bool, string) {
	provisioned := make(map[string]bool)

	// Check .secrets.yaml
	yamlFile := filepath.Join(workflowDir, ".secrets.yaml")
	if data, err := os.ReadFile(yamlFile); err == nil {
		var secrets map[string]interface{}
		if err := parseYAMLMap(data, &secrets); err == nil {
			for k := range secrets {
				provisioned[k] = true
			}
		}
		return provisioned, ".secrets.yaml"
	}

	// Check .secrets/ directory
	secretsDir := filepath.Join(workflowDir, ".secrets")
	if entries, err := os.ReadDir(secretsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				provisioned[entry.Name()] = true
			}
		}
		return provisioned, ".secrets/"
	}

	return provisioned, ""
}

// parseYAMLMap parses YAML data into a map.
func parseYAMLMap(data []byte, out *map[string]interface{}) error {
	return yaml.Unmarshal(data, out)
}

// resolveSharedSecrets resolves $shared.<name> references in secrets map.
// Shared secrets live at <workspace>/.secrets/<name> where workspace is
// read from config, or falls back to git root detection.
func resolveSharedSecrets(secrets map[string]interface{}, workflowDir string) error {
	// Check if any secrets actually use $shared references
	hasShared := false
	for _, v := range secrets {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "$shared.") {
			hasShared = true
			break
		}
	}
	if !hasShared {
		return nil
	}

	// Try workspace config first
	cfg := LoadConfig()
	var repoRoot string
	if cfg.Workspace != "" {
		repoRoot = expandHome(cfg.Workspace)
	} else {
		repoRoot = findRepoRoot(workflowDir)
		if repoRoot == "" {
			return fmt.Errorf("cannot resolve $shared secrets: no workspace configured and no git repository found; set 'workspace' in ~/.tentacular/config.yaml or run 'tntc init-workspace'")
		}
		fmt.Fprintf(os.Stderr, "Warning: falling back to git root for $shared secrets. Set 'workspace' in config.\n")
	}
	sharedDir := filepath.Join(repoRoot, ".secrets")

	for k, v := range secrets {
		strVal, ok := v.(string)
		if !ok || !strings.HasPrefix(strVal, "$shared.") {
			continue
		}
		sharedName := strings.TrimPrefix(strVal, "$shared.")
		// Prevent path traversal: resolve and verify the path stays within sharedDir
		resolvedPath := filepath.Clean(filepath.Join(sharedDir, sharedName))
		if !strings.HasPrefix(resolvedPath, filepath.Clean(sharedDir)+string(filepath.Separator)) {
			return fmt.Errorf("shared secret name %q contains invalid path components", sharedName)
		}
		content, err := os.ReadFile(resolvedPath)
		if err != nil {
			return fmt.Errorf("shared secret %q referenced but not found at %s/.secrets/%s", strVal, repoRoot, sharedName)
		}
		// Try JSON parse first, fall back to plain string
		var parsed interface{}
		if err := json.Unmarshal(content, &parsed); err == nil {
			secrets[k] = parsed
		} else {
			secrets[k] = strings.TrimSpace(string(content))
		}
	}
	return nil
}

// findRepoRoot walks up from dir looking for .git/ or go.mod.
func findRepoRoot(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(absDir, ".git")); err == nil {
			return absDir
		}
		if _, err := os.Stat(filepath.Join(absDir, "go.mod")); err == nil {
			return absDir
		}
		parent := filepath.Dir(absDir)
		if parent == absDir {
			return ""
		}
		absDir = parent
	}
}
