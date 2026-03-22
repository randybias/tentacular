package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/params"
	"github.com/randybias/tentacular/pkg/scaffold"
)

func newScaffoldInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <scaffold-name> <tentacle-name>",
		Short: "Create a tentacle from a scaffold",
		Args:  cobra.ExactArgs(2),
		RunE:  runScaffoldInit,
	}
	cmd.Flags().String("source", "", "Scaffold source: private or public (default: private first)")
	cmd.Flags().String("params-file", "", "Apply parameter values from YAML file to workflow.yaml")
	cmd.Flags().Bool("no-params", false, "Copy scaffold as-is without printing parameter prompts")
	cmd.Flags().String("namespace", "", "Set deployment.namespace in workflow.yaml")
	cmd.Flags().String("dir", "", "Override output directory (default: ~/tentacles/<tentacle-name>/)")
	return cmd
}

func runScaffoldInit(cmd *cobra.Command, args []string) error {
	scaffoldName := args[0]
	tentacleName := args[1]

	if err := scaffold.ValidateScaffoldName(scaffoldName); err != nil {
		return fmt.Errorf("invalid scaffold name: %w", err)
	}
	if !kebabCaseRe.MatchString(tentacleName) {
		return fmt.Errorf("tentacle name must be kebab-case (e.g., my-tentacle), got: %s", tentacleName)
	}

	source, _ := cmd.Flags().GetString("source")
	paramsFile, _ := cmd.Flags().GetString("params-file")
	noParams, _ := cmd.Flags().GetBool("no-params")
	namespace, _ := cmd.Flags().GetString("namespace")
	dirOverride, _ := cmd.Flags().GetString("dir")

	cfg := LoadConfig()
	client := scaffold.NewClient(cfg.Scaffold)

	entry, findErr := scaffold.FindScaffold(scaffoldName, source, client.CachedIndexPath())
	if findErr != nil {
		return findErr
	}

	// Check min version
	checkMinVersion(entry.MinTentacularVersion)

	// Determine output directory
	outDir, dirErr := resolveOutDir(dirOverride, tentacleName)
	if dirErr != nil {
		return dirErr
	}

	if _, statErr := os.Stat(outDir); statErr == nil {
		return fmt.Errorf("directory already exists: %s", outDir)
	}

	// Copy scaffold files
	if copyErr := copyScaffoldFiles(entry, outDir); copyErr != nil {
		return copyErr
	}

	// Rename workflow name field
	workflowPath := filepath.Join(outDir, "workflow.yaml")
	if renameErr := renameWorkflowName(workflowPath, tentacleName); renameErr != nil {
		return fmt.Errorf("updating workflow name: %w", renameErr)
	}

	// Apply namespace if provided
	if namespace != "" {
		if nsErr := applyNamespace(workflowPath, namespace); nsErr != nil {
			return fmt.Errorf("setting namespace: %w", nsErr)
		}
	}

	// Write tentacle.yaml
	if tentErr := writeTentacleYAML(outDir, tentacleName, entry); tentErr != nil {
		return fmt.Errorf("writing tentacle.yaml: %w", tentErr)
	}

	// Handle params
	schemaPath := filepath.Join(outDir, "params.schema.yaml")
	schemaExists := fileExists(schemaPath)

	var schema *params.Schema
	var schemaErr error
	if schemaExists {
		schema, schemaErr = params.LoadSchema(schemaPath)
		if schemaErr != nil {
			return fmt.Errorf("loading params schema: %w", schemaErr)
		}
	}

	fmt.Printf("Scaffolded tentacle '%s' from '%s' (%s) in %s\n\n", tentacleName, scaffoldName, entry.Source, outDir)

	switch {
	case paramsFile != "" && schema != nil:
		if err := applyParamsFile(workflowPath, schema, paramsFile); err != nil {
			return err
		}
	case schemaExists && schema != nil && !noParams:
		printScaffoldParams(schema)
		writeParamsExample(outDir, schema)
		fmt.Printf("Edit workflow.yaml to set your values, then:\n")
		fmt.Printf("  tntc validate\n")
		fmt.Printf("  tntc dev\n")
	default:
		fmt.Printf("Next steps:\n")
		fmt.Printf("  tntc validate\n")
		fmt.Printf("  tntc dev\n")
	}

	return nil
}

func resolveOutDir(dirOverride, tentacleName string) (string, error) {
	if dirOverride != "" {
		return dirOverride, nil
	}
	tentaclesDir, err := scaffold.TentaclesDir()
	if err != nil {
		return "", fmt.Errorf("resolving tentacles directory: %w", err)
	}
	return filepath.Join(tentaclesDir, tentacleName), nil
}

// copyScaffoldFiles copies all files from entry's source directory to outDir.
// For public scaffolds (no local Path), it returns an error directing users to sync.
func copyScaffoldFiles(entry *scaffold.ScaffoldEntry, outDir string) error {
	if entry.Path == "" {
		return fmt.Errorf("scaffold '%s' has no local path; run 'tntc scaffold sync' first", entry.Name)
	}
	return copyScaffoldDir(entry.Path, outDir)
}

// copyScaffoldDir recursively copies src into dst, preserving directory structure.
// Skips symlinks and never copies .secrets.yaml.
func copyScaffoldDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks — do not follow to prevent path traversal via symlinks.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}

		// Hard-exclude .secrets.yaml; only .secrets.yaml.example is safe to copy.
		if filepath.Base(rel) == ".secrets.yaml" {
			return nil
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}

		data, readErr := os.ReadFile(path) //nolint:gosec // reading scaffold source file
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o700); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(target, data, 0o600) //nolint:gosec // scaffold file under validated dst
	})
}

func renameWorkflowName(workflowPath, name string) error {
	data, readErr := os.ReadFile(workflowPath) //nolint:gosec // path controlled by caller
	if readErr != nil {
		// workflow.yaml might not exist in a scaffold directory -- not fatal
		return nil //nolint:nilerr // intentional: missing workflow.yaml is not an error
	}
	// Replace the top-level name: field
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "name: ") {
			lines[i] = "name: " + name
			break
		}
	}
	return os.WriteFile(workflowPath, []byte(strings.Join(lines, "\n")), 0o644) //nolint:gosec // workflow file
}

func applyNamespace(workflowPath, namespace string) error {
	data, err := os.ReadFile(workflowPath) //nolint:gosec // path controlled by caller
	if err != nil {
		return err
	}
	content := string(data)
	if strings.Contains(content, "namespace:") {
		lines := strings.Split(content, "\n")
		inDeployment := false
		for i, line := range lines {
			if strings.TrimSpace(line) == "deployment:" || strings.HasPrefix(line, "deployment:") {
				inDeployment = true
			}
			if inDeployment && strings.Contains(line, "namespace:") {
				trimmed := strings.TrimLeft(line, " \t")
				indent := line[:len(line)-len(trimmed)]
				lines[i] = indent + "namespace: " + namespace
				break
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		content += "\ndeployment:\n  namespace: " + namespace + "\n"
	}
	return os.WriteFile(workflowPath, []byte(content), 0o644) //nolint:gosec // workflow file
}

func writeTentacleYAML(dir, tentacleName string, entry *scaffold.ScaffoldEntry) error {
	t := scaffold.TentacleYAML{
		Name:    tentacleName,
		Created: time.Now().UTC().Format(time.RFC3339),
		Scaffold: &scaffold.TentacleScaffold{
			Name:     entry.Name,
			Version:  entry.Version,
			Source:   entry.Source,
			Modified: false,
		},
	}

	data, err := yaml.Marshal(&t)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tentacle.yaml"), data, 0o644) //nolint:gosec // tentacle metadata file
}

func applyParamsFile(workflowPath string, schema *params.Schema, paramsFilePath string) error {
	values, err := params.LoadParamsFile(paramsFilePath)
	if err != nil {
		return err
	}

	errs := params.Validate(schema, values)
	if len(errs) > 0 {
		return fmt.Errorf("parameter validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	if err := params.ApplyToFile(workflowPath, schema, values); err != nil {
		return err
	}

	fmt.Printf("Applied %d parameters from %s:\n", len(values), paramsFilePath)
	for k, v := range values {
		switch val := v.(type) {
		case []any:
			fmt.Printf("  %s: %d entries\n", k, len(val))
		default:
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  tntc validate\n")
	fmt.Printf("  tntc dev\n")
	return nil
}

func printScaffoldParams(schema *params.Schema) {
	fmt.Printf("This scaffold has configurable parameters:\n\n")
	for name, def := range schema.Parameters {
		reqStr := "optional"
		if def.Required {
			reqStr = "required"
		}
		typeStr := def.Type
		if def.Default != nil {
			typeStr = fmt.Sprintf("%s, optional, default: %v", def.Type, def.Default)
		}
		fmt.Printf("  %s (%s, %s):\n    %s\n", name, typeStr, reqStr, def.Description)
		if def.Example != nil {
			exampleData, _ := yaml.Marshal(def.Example)
			fmt.Printf("    Example: %s\n", strings.TrimSpace(string(exampleData)))
		}
		fmt.Println()
	}
}

func writeParamsExample(dir string, schema *params.Schema) {
	examplePath := filepath.Join(dir, "params.yaml.example")
	example := map[string]any{}
	for name, def := range schema.Parameters {
		if def.Example != nil {
			example[name] = def.Example
		} else if def.Default != nil {
			example[name] = def.Default
		}
	}
	if len(example) == 0 {
		return
	}
	data, err := yaml.Marshal(example)
	if err != nil {
		return
	}
	_ = os.WriteFile(examplePath, data, 0o644) //nolint:gosec // non-sensitive example file
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
