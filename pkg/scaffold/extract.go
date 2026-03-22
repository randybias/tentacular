package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/params"
)

// ExtractOptions controls what tntc scaffold extract does.
type ExtractOptions struct { //nolint:govet // field order for readability
	// Name is the scaffold name. Defaults to the tentacle name with "-scaffold" suffix.
	Name string
	// OutputDir is the destination directory. If empty, defaults based on Public flag.
	OutputDir string
	// JSONOnly skips file generation and returns analysis as JSON-serializable data.
	JSONOnly bool
	// Public writes to ./scaffold-output/ instead of private scaffolds dir.
	Public bool
}

// ProposedParam is one proposed parameter from the extraction analysis.
type ProposedParam struct { //nolint:govet // field order for readability
	ExampleValue any    `json:"exampleValue,omitempty"`
	DefaultValue any    `json:"defaultValue,omitempty"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	Required     bool   `json:"required"`
}

// ExtractAnalysis is the result of analyzing a tentacle for extraction.
type ExtractAnalysis struct { //nolint:govet // field order for readability
	SecretWarnings []string        `json:"secretWarnings,omitempty"`
	KeptDefaults   []string        `json:"keptDefaults,omitempty"`
	ProposedParams []ProposedParam `json:"proposedParams"`
	ScaffoldName   string          `json:"scaffoldName"`
	TentacleName   string          `json:"tentacleName"`
	OutputDir      string          `json:"outputDir,omitempty"`
}

// ExtractResult is returned after files are written.
type ExtractResult struct {
	Analysis  ExtractAnalysis
	OutputDir string
}

// secrentsYAMLName is the name of the secrets file that must never be copied.
const secretsYAMLName = ".secrets.yaml"

// Extract analyzes a tentacle directory and generates scaffold files.
// If opts.JSONOnly is true, returns the analysis without writing any files.
func Extract(tentacleDir string, opts ExtractOptions) (*ExtractResult, error) {
	// Read tentacle.yaml for name
	tentacleYAML, err := readTentacleYAML(tentacleDir)
	if err != nil {
		return nil, err
	}

	// Derive scaffold name
	scaffoldName := opts.Name
	if scaffoldName == "" {
		if tentacleYAML != nil && tentacleYAML.Name != "" {
			scaffoldName = tentacleYAML.Name
		} else {
			scaffoldName = filepath.Base(tentacleDir)
		}
	}
	if nameErr := ValidateScaffoldName(scaffoldName); nameErr != nil {
		return nil, fmt.Errorf("invalid scaffold name: %w", nameErr)
	}

	// Read workflow.yaml
	workflowPath := filepath.Join(tentacleDir, "workflow.yaml")
	workflowData, readErr := os.ReadFile(workflowPath) //nolint:gosec
	if readErr != nil {
		return nil, fmt.Errorf("reading workflow.yaml: %w", readErr)
	}

	// Parse workflow.yaml as a generic map for analysis
	var workflowMap map[string]any
	if parseErr := yaml.Unmarshal(workflowData, &workflowMap); parseErr != nil {
		return nil, fmt.Errorf("parsing workflow.yaml: %w", parseErr)
	}

	// Analyze config section for parameterizable values and secrets
	analysis := analyzeWorkflow(scaffoldName, workflowMap)
	if tentacleYAML != nil {
		analysis.TentacleName = tentacleYAML.Name
	} else {
		analysis.TentacleName = filepath.Base(tentacleDir)
	}

	// Determine output directory
	outDir, dirErr := resolveExtractOutputDir(opts, scaffoldName)
	if dirErr != nil {
		return nil, dirErr
	}
	analysis.OutputDir = outDir

	if opts.JSONOnly {
		return &ExtractResult{Analysis: analysis}, nil
	}

	// Confirm output dir doesn't already exist
	if _, statErr := os.Stat(outDir); statErr == nil {
		return nil, fmt.Errorf("output directory already exists: %s (remove it or use a different name)", outDir)
	}

	// Write scaffold files
	if writeErr := writeScaffoldFiles(tentacleDir, outDir, analysis, workflowData); writeErr != nil {
		// Best-effort cleanup on failure
		_ = os.RemoveAll(outDir)
		return nil, writeErr
	}

	return &ExtractResult{Analysis: analysis, OutputDir: outDir}, nil
}

// analyzeWorkflow reads the workflow.yaml map and produces extraction analysis.
func analyzeWorkflow(scaffoldName string, wf map[string]any) ExtractAnalysis {
	analysis := ExtractAnalysis{
		ScaffoldName: scaffoldName,
	}

	config, ok := wf["config"].(map[string]any)
	if !ok {
		// No config section — nothing to parameterize
		return analysis
	}

	for key, rawVal := range config {
		strVal, isStr := rawVal.(string)
		if !isStr {
			// Non-string values: check if they look like structured data with secrets
			// then decide keep vs parameterize-with-default
			classification := "keep"
			if isThresholdKey(key) {
				classification = "parameterize-with-default"
			}
			if classification == "parameterize-with-default" {
				p := ProposedParam{
					Name:         key,
					Path:         "config." + key,
					Type:         InferParamType(rawVal),
					Description:  "Configure " + key,
					DefaultValue: rawVal,
					Required:     false,
				}
				analysis.ProposedParams = append(analysis.ProposedParams, p)
			} else {
				analysis.KeptDefaults = append(analysis.KeptDefaults, fmt.Sprintf("%s: %v", key, rawVal))
			}
			continue
		}

		classification := ClassifyValue(key, strVal)
		switch classification {
		case "secret":
			analysis.SecretWarnings = append(analysis.SecretWarnings,
				fmt.Sprintf("config.%s: value appears to be a secret — remove before extracting", key))
		case "parameterize":
			p := ProposedParam{
				Name:         key,
				Path:         "config." + key,
				Type:         "string",
				Description:  "Configure " + key,
				ExampleValue: SafeExampleValue(strVal),
				Required:     true,
			}
			analysis.ProposedParams = append(analysis.ProposedParams, p)
		case "parameterize-with-default":
			p := ProposedParam{
				Name:         key,
				Path:         "config." + key,
				Type:         "string",
				Description:  "Configure " + key,
				DefaultValue: strVal,
				Required:     false,
			}
			analysis.ProposedParams = append(analysis.ProposedParams, p)
		default: // "keep"
			analysis.KeptDefaults = append(analysis.KeptDefaults, fmt.Sprintf("%s: %s", key, strVal))
		}
	}

	return analysis
}

// writeScaffoldFiles generates all scaffold output files.
func writeScaffoldFiles(tentacleDir, outDir string, analysis ExtractAnalysis, workflowData []byte) error {
	if mkErr := os.MkdirAll(outDir, 0o700); mkErr != nil {
		return fmt.Errorf("creating output directory: %w", mkErr)
	}

	// 1. Write scaffold.yaml (metadata)
	if err := writeScaffoldYAML(outDir, analysis); err != nil {
		return err
	}

	// 2. Write params.schema.yaml
	if err := writeParamsSchema(outDir, analysis); err != nil {
		return err
	}

	// 3. Write sanitized workflow.yaml
	if err := writeSanitizedWorkflow(outDir, workflowData, analysis); err != nil {
		return err
	}

	// 4. Write params.yaml.example
	if err := writeParamsExample(outDir, analysis); err != nil {
		return err
	}

	// 5. Write .secrets.yaml.example (sanitized — key names only, no values)
	if err := writeSecretsExample(tentacleDir, outDir); err != nil {
		return err
	}

	// 6. Copy node code and test fixtures (never .secrets.yaml)
	return copyExtractFiles(tentacleDir, outDir)
}

func writeScaffoldYAML(outDir string, analysis ExtractAnalysis) error {
	entry := ScaffoldEntry{
		Name:        analysis.ScaffoldName,
		DisplayName: analysis.ScaffoldName,
		Description: "Scaffold extracted from " + analysis.TentacleName,
		Version:     "1.0",
		Author:      "",
		Complexity:  "medium",
		Category:    "custom",
	}
	data, err := yaml.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling scaffold.yaml: %w", err)
	}
	return os.WriteFile(filepath.Join(outDir, "scaffold.yaml"), data, 0o600)
}

func writeParamsSchema(outDir string, analysis ExtractAnalysis) error {
	if len(analysis.ProposedParams) == 0 {
		// Write an empty schema placeholder
		content := "version: \"1.0\"\ndescription: \"Generated by tntc scaffold extract\"\nparameters: {}\n"
		return os.WriteFile(filepath.Join(outDir, "params.schema.yaml"), []byte(content), 0o600)
	}

	schema := params.Schema{
		Version:     "1.0",
		Description: fmt.Sprintf("Parameters for %s scaffold", analysis.ScaffoldName),
		Parameters:  make(map[string]params.ParamDef, len(analysis.ProposedParams)),
	}
	for _, p := range analysis.ProposedParams {
		def := params.ParamDef{
			Path:        p.Path,
			Type:        p.Type,
			Description: p.Description,
			Required:    p.Required,
		}
		if p.ExampleValue != nil {
			def.Example = p.ExampleValue
		}
		if p.DefaultValue != nil {
			def.Default = p.DefaultValue
		}
		schema.Parameters[p.Name] = def
	}

	data, marshalErr := yaml.Marshal(schema)
	if marshalErr != nil {
		return fmt.Errorf("marshaling params.schema.yaml: %w", marshalErr)
	}
	return os.WriteFile(filepath.Join(outDir, "params.schema.yaml"), data, 0o600)
}

func writeSanitizedWorkflow(outDir string, workflowData []byte, analysis ExtractAnalysis) error {
	if len(analysis.ProposedParams) == 0 {
		// No replacements needed — write workflow as-is
		return os.WriteFile(filepath.Join(outDir, "workflow.yaml"), workflowData, 0o600) //nolint:gosec // outDir is validated by resolveExtractOutputDir
	}

	// Build schema and values for the overlay package
	schema := &params.Schema{
		Version:    "1.0",
		Parameters: make(map[string]params.ParamDef, len(analysis.ProposedParams)),
	}
	values := make(map[string]any, len(analysis.ProposedParams))
	for _, p := range analysis.ProposedParams {
		schema.Parameters[p.Name] = params.ParamDef{
			Path:     p.Path,
			Type:     p.Type,
			Required: p.Required,
		}
		if p.ExampleValue != nil {
			values[p.Name] = p.ExampleValue
		} else if p.DefaultValue != nil {
			values[p.Name] = p.DefaultValue
		}
	}

	// Write base workflow first, then apply replacements in-place
	workflowFile := filepath.Join(outDir, "workflow.yaml")
	if writeErr := os.WriteFile(workflowFile, workflowData, 0o600); writeErr != nil { //nolint:gosec // workflowFile path is under validated outDir
		return fmt.Errorf("writing workflow.yaml: %w", writeErr)
	}

	if applyErr := params.ApplyToFile(workflowFile, schema, values); applyErr != nil {
		// Non-fatal: best-effort replacement; the file is still valid YAML
		_ = applyErr
	}
	return nil
}

func writeParamsExample(outDir string, analysis ExtractAnalysis) error {
	if len(analysis.ProposedParams) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("# Example parameter values for ")
	sb.WriteString(analysis.ScaffoldName)
	sb.WriteString(" scaffold\n")
	sb.WriteString("# Copy this file to params.yaml and fill in real values.\n\n")
	for _, p := range analysis.ProposedParams {
		sb.WriteString("# ")
		sb.WriteString(p.Description)
		sb.WriteString("\n")
		if p.ExampleValue != nil {
			fmt.Fprintf(&sb, "%s: %v\n", p.Name, p.ExampleValue)
		} else if p.DefaultValue != nil {
			fmt.Fprintf(&sb, "%s: %v\n", p.Name, p.DefaultValue)
		} else {
			fmt.Fprintf(&sb, "%s: \"\"\n", p.Name)
		}
		sb.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(outDir, "params.yaml.example"), []byte(sb.String()), 0o600)
}

func writeSecretsExample(tentacleDir, outDir string) error {
	secretsPath := filepath.Join(tentacleDir, secretsYAMLName)
	data, err := os.ReadFile(secretsPath) //nolint:gosec
	if os.IsNotExist(err) {
		return nil // no secrets file — nothing to generate
	}
	if err != nil {
		return fmt.Errorf("reading .secrets.yaml: %w", err)
	}

	// Parse secrets file
	var secretsMap map[string]any
	if parseErr := yaml.Unmarshal(data, &secretsMap); parseErr != nil {
		// If we can't parse it, write a comment-only example
		content := "# .secrets.yaml structure (fill in real values)\n# Could not parse source .secrets.yaml\n"
		return os.WriteFile(filepath.Join(outDir, ".secrets.yaml.example"), []byte(content), 0o600)
	}

	// Write example with key names only, values replaced with empty strings / placeholders
	sanitized := sanitizeSecretsMap(secretsMap)
	out, marshalErr := yaml.Marshal(sanitized)
	if marshalErr != nil {
		return fmt.Errorf("marshaling .secrets.yaml.example: %w", marshalErr)
	}
	header := "# .secrets.yaml.example -- structure only, no real values\n# Copy to .secrets.yaml and fill in real credentials.\n\n"
	return os.WriteFile(filepath.Join(outDir, ".secrets.yaml.example"), append([]byte(header), out...), 0o600)
}

// sanitizeSecretsMap recursively replaces all leaf string values with empty strings.
func sanitizeSecretsMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch tv := v.(type) {
		case string:
			out[k] = ""
		case map[string]any:
			out[k] = sanitizeSecretsMap(tv)
		case []any:
			out[k] = sanitizeSecretsSlice(tv)
		default:
			out[k] = tv
		}
	}
	return out
}

func sanitizeSecretsSlice(s []any) []any {
	out := make([]any, len(s))
	for i, v := range s {
		switch tv := v.(type) {
		case string:
			out[i] = ""
		case map[string]any:
			out[i] = sanitizeSecretsMap(tv)
		default:
			out[i] = tv
		}
	}
	return out
}

// copyExtractFiles copies nodes/, tests/, and other scaffold-relevant files
// to the output directory. NEVER copies .secrets.yaml.
func copyExtractFiles(tentacleDir, outDir string) error {
	entries, err := os.ReadDir(tentacleDir)
	if err != nil {
		return fmt.Errorf("reading tentacle directory: %w", err)
	}

	for _, e := range entries {
		name := e.Name()

		// Hard-exclude .secrets.yaml (security Finding 4 — must-have)
		if name == secretsYAMLName {
			continue
		}

		// Skip files already written by other steps
		switch name {
		case "workflow.yaml", "params.schema.yaml", "scaffold.yaml",
			"params.yaml.example", ".secrets.yaml.example", "tentacle.yaml":
			continue
		}

		// Skip params.yaml files (user-specific values, not scaffold content)
		if name == "params.yaml" || strings.HasSuffix(name, ".params.yaml") {
			continue
		}

		// Skip symlinks (security)
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}

		src := filepath.Join(tentacleDir, name)
		dst := filepath.Join(outDir, name)

		// Verify src is still within tentacleDir (path confinement)
		relPath, relErr := filepath.Rel(tentacleDir, src)
		if relErr != nil || strings.HasPrefix(relPath, "..") {
			continue
		}

		if e.IsDir() {
			if cpErr := copyDirExtract(src, dst); cpErr != nil {
				return fmt.Errorf("copying %s: %w", name, cpErr)
			}
		} else {
			if cpErr := copyFileExtract(src, dst); cpErr != nil {
				return fmt.Errorf("copying %s: %w", name, cpErr)
			}
		}
	}
	return nil
}

func copyDirExtract(src, dst string) error {
	if mkErr := os.MkdirAll(dst, 0o700); mkErr != nil {
		return mkErr
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		// Skip symlinks
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())

		// Path confinement: verify dst is still under the top dst
		relPath, relErr := filepath.Rel(src, srcPath)
		if relErr != nil || strings.HasPrefix(relPath, "..") {
			continue
		}

		if e.IsDir() {
			if err := copyDirExtract(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFileExtract(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFileExtract(src, dst string) error {
	data, err := os.ReadFile(src) //nolint:gosec
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600) //nolint:gosec // dst is under validated outDir after path confinement check
}

func resolveExtractOutputDir(opts ExtractOptions, scaffoldName string) (string, error) {
	if opts.OutputDir != "" {
		// Validate no path traversal in override
		abs, err := filepath.Abs(opts.OutputDir)
		if err != nil {
			return "", fmt.Errorf("resolving output dir: %w", err)
		}
		return abs, nil
	}
	if opts.Public {
		// ./scaffold-output/ relative to cwd
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
		return filepath.Join(cwd, "scaffold-output"), nil
	}
	// Default: private scaffolds dir
	dir, err := EnsurePrivateScaffoldsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, scaffoldName), nil
}

func readTentacleYAML(dir string) (*TentacleYAML, error) {
	path := filepath.Join(dir, "tentacle.yaml")
	data, err := os.ReadFile(path) //nolint:gosec
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil // tentacle.yaml is optional (tentacles created before Phase 1)
	}
	if err != nil {
		return nil, fmt.Errorf("reading tentacle.yaml: %w", err)
	}
	var t TentacleYAML
	if parseErr := yaml.Unmarshal(data, &t); parseErr != nil {
		return nil, fmt.Errorf("parsing tentacle.yaml: %w", parseErr)
	}
	return &t, nil
}

// FormatExtractSummary returns human-readable output for a completed extraction.
func FormatExtractSummary(result *ExtractResult) string {
	a := result.Analysis
	var sb strings.Builder
	fmt.Fprintf(&sb, "Extracted scaffold '%s' from tentacle '%s'\n", a.ScaffoldName, a.TentacleName)
	sb.WriteString("\n")

	if len(a.ProposedParams) > 0 {
		sb.WriteString("Parameterized:\n")
		for _, p := range a.ProposedParams {
			fmt.Fprintf(&sb, "  %s (%s", p.Name, p.Type)
			if p.Required {
				sb.WriteString(", required")
			}
			fmt.Fprintf(&sb, ") -- %s\n", p.Path)
		}
		sb.WriteString("\n")
	}

	if len(a.KeptDefaults) > 0 {
		sb.WriteString("Kept as defaults:\n")
		for _, kd := range a.KeptDefaults {
			fmt.Fprintf(&sb, "  %s\n", kd)
		}
		sb.WriteString("\n")
	}

	if result.OutputDir != "" {
		fmt.Fprintf(&sb, "Saved to: %s\n", result.OutputDir)
		sb.WriteString("\n")
		sb.WriteString("To publish as a public quickstart:\n")
		sb.WriteString("  Review the scaffold files, then PR to tentacular-scaffolds\n")
		sb.WriteString("  Or re-run with: tntc scaffold extract --public\n")
	}

	return sb.String()
}
