// Integration tests for tntc scaffold init.
//
// Tests run in-process by calling runScaffoldInit directly.
// Each test sets HOME to a temp dir to avoid touching real user state.
//
// Covers 12 cases from design doc Section 12.4:
//   - Init with --no-params
//   - Init with --params-file (valid params)
//   - Init with invalid params (missing required field)
//   - Init with type mismatch
//   - Init default mode (no flags)
//   - Init scaffold not found
//   - Init with --dir override
//   - Init name conflict (directory already exists)
//   - Init from private scaffold
//   - Init private vs public precedence
//   - Init with --source=public
//   - Init scaffold without params.schema

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/scaffold"
)

// scaffoldTestFixture builds a minimal scaffold in a temp dir and returns the
// HOME directory set up with private scaffolds.
//
// If schemaYAML is "", no params.schema.yaml is written.
func scaffoldTestFixture(t *testing.T, scaffoldName, schemaYAML string) (homeDir, scaffoldDir string) {
	t.Helper()
	home := t.TempDir()
	sDir := filepath.Join(home, ".tentacular", "scaffolds", scaffoldName)
	if err := os.MkdirAll(sDir, 0o755); err != nil {
		t.Fatalf("mkdir scaffold: %v", err)
	}

	// scaffold.yaml
	scaffoldMeta := "name: " + scaffoldName + "\n" +
		"displayName: Test Scaffold\n" +
		"description: A test scaffold\n" +
		"category: testing\n" +
		"version: \"1.0\"\n" +
		"author: test\n" +
		"tags: []\n"
	if err := os.WriteFile(filepath.Join(sDir, "scaffold.yaml"),
		[]byte(scaffoldMeta), 0o644); err != nil {
		t.Fatal(err)
	}

	// workflow.yaml with example values
	workflowYAML := "name: " + scaffoldName + "\n" +
		"version: \"1.0\"\n" +
		"triggers:\n" +
		"  - type: manual\n" +
		"  - type: cron\n" +
		"    name: check-endpoints\n" +
		"    schedule: \"*/5 * * * *\"\n" +
		"config:\n" +
		"  timeout: 120s\n" +
		"  endpoints:\n" +
		"    - url: \"https://example.com\"\n" +
		"nodes:\n" +
		"  probe:\n" +
		"    path: ./nodes/probe.ts\n" +
		"deployment:\n" +
		"  namespace: \"\"\n"
	if err := os.WriteFile(filepath.Join(sDir, "workflow.yaml"),
		[]byte(workflowYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	// .secrets.yaml.example
	if err := os.WriteFile(filepath.Join(sDir, ".secrets.yaml.example"),
		[]byte("# secrets\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// params.schema.yaml (optional)
	if schemaYAML != "" {
		if err := os.WriteFile(filepath.Join(sDir, "params.schema.yaml"),
			[]byte(schemaYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return home, sDir
}

const testParamsSchemaYAML = `version: "1"
parameters:
  probe_schedule:
    path: triggers[name=check-endpoints].schedule
    type: string
    description: "Cron schedule"
    required: false
    default: "*/5 * * * *"
  endpoints:
    path: config.endpoints
    type: list
    description: "Endpoints to probe"
    required: true
    example:
      - url: "https://example.com"
`

// setTestHome sets HOME to the given dir for the duration of the test,
// and also clears TENTACULAR_ENV to avoid config file lookups bleeding through.
func setTestHome(t *testing.T, home string) {
	t.Helper()
	orig := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", orig) })
	_ = os.Setenv("HOME", home)
}

// makeCmdWithArgs returns a scaffold init command pre-configured with the
// given arguments and flags. Use cmd.SetOut to capture output.
func makeScaffoldInitCmd(scaffoldName, tentacleName string, flags map[string]string, boolFlags map[string]bool) error {
	cmd := newScaffoldInitCmd()
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			return err
		}
	}
	for k, v := range boolFlags {
		_ = v
		if err := cmd.Flags().Set(k, "true"); err != nil {
			_ = err
		}
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	args := []string{scaffoldName, tentacleName}
	return cmd.RunE(cmd, args)
}

// TestScaffoldInitNoParams verifies that --no-params copies all scaffold
// files, creates tentacle.yaml with provenance, and writes params.yaml.example.
func TestScaffoldInitNoParams(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", testParamsSchemaYAML)
	setTestHome(t, home)

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{"dir": outDir},
		map[string]bool{"no-params": true},
	)
	if err != nil {
		t.Fatalf("scaffold init --no-params: %v", err)
	}

	// workflow.yaml must exist
	if _, statErr := os.Stat(filepath.Join(outDir, "workflow.yaml")); statErr != nil {
		t.Errorf("expected workflow.yaml in output dir: %v", statErr)
	}
	// tentacle.yaml must exist with provenance
	tentacleData, err := os.ReadFile(filepath.Join(outDir, "tentacle.yaml"))
	if err != nil {
		t.Fatalf("expected tentacle.yaml: %v", err)
	}
	var tentacle map[string]any
	if unmarshalErr := yaml.Unmarshal(tentacleData, &tentacle); unmarshalErr != nil {
		t.Fatalf("parsing tentacle.yaml: %v", unmarshalErr)
	}
	scaffoldSection, ok := tentacle["scaffold"]
	if !ok {
		t.Error("tentacle.yaml missing 'scaffold' section")
	}
	scaffoldMap, _ := scaffoldSection.(map[string]any)
	if scaffoldMap["name"] != "test-scaffold" {
		t.Errorf("scaffold.name: got %v, want %q", scaffoldMap["name"], "test-scaffold")
	}
	// With --no-params, params.yaml.example is NOT written (only in default mode).
	// Verify workflow.yaml still has the scaffold's example values (not replaced).
	workflowData, err := os.ReadFile(filepath.Join(outDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("reading workflow.yaml: %v", err)
	}
	if !strings.Contains(string(workflowData), "example.com") {
		t.Errorf("expected example values preserved in workflow.yaml with --no-params, got:\n%s", string(workflowData))
	}
}

// TestScaffoldInitWithParamsFile verifies that --params-file applies parameter
// values to workflow.yaml and creates tentacle.yaml.
func TestScaffoldInitWithParamsFile(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", testParamsSchemaYAML)
	setTestHome(t, home)

	// Write params file
	paramsDir := t.TempDir()
	paramsFile := filepath.Join(paramsDir, "params.yaml")
	paramsContent := "endpoints:\n  - url: \"https://mysite.com\"\nprobe_schedule: \"*/10 * * * *\"\n"
	if err := os.WriteFile(paramsFile, []byte(paramsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{"dir": outDir, "params-file": paramsFile},
		nil,
	)
	if err != nil {
		t.Fatalf("scaffold init --params-file: %v", err)
	}

	// workflow.yaml should have the applied values
	data, err := os.ReadFile(filepath.Join(outDir, "workflow.yaml"))
	if err != nil {
		t.Fatalf("reading workflow.yaml: %v", err)
	}
	if !strings.Contains(string(data), "mysite.com") {
		t.Errorf("expected workflow.yaml to contain applied endpoint, got:\n%s", string(data))
	}
	// tentacle.yaml must exist
	if _, err := os.Stat(filepath.Join(outDir, "tentacle.yaml")); err != nil {
		t.Errorf("expected tentacle.yaml: %v", err)
	}
}

// TestScaffoldInitInvalidParamsMissingRequired verifies that --params-file
// with a missing required parameter returns an error.
func TestScaffoldInitInvalidParamsMissingRequired(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", testParamsSchemaYAML)
	setTestHome(t, home)

	// params file is missing 'endpoints' (required)
	paramsDir := t.TempDir()
	paramsFile := filepath.Join(paramsDir, "params.yaml")
	if err := os.WriteFile(paramsFile,
		[]byte("probe_schedule: \"*/10 * * * *\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{"dir": outDir, "params-file": paramsFile},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for missing required param, got nil")
	}
	if !strings.Contains(err.Error(), "endpoints") {
		t.Errorf("expected error to mention 'endpoints', got: %v", err)
	}
}

// TestScaffoldInitTypeMismatch verifies that --params-file with wrong type
// for a parameter returns an error mentioning the parameter.
func TestScaffoldInitTypeMismatch(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", testParamsSchemaYAML)
	setTestHome(t, home)

	// endpoints must be a list, not a string
	paramsDir := t.TempDir()
	paramsFile := filepath.Join(paramsDir, "params.yaml")
	if err := os.WriteFile(paramsFile,
		[]byte("endpoints: \"not-a-list\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{"dir": outDir, "params-file": paramsFile},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for type mismatch, got nil")
	}
}

// TestScaffoldInitDefaultMode verifies that the default mode (no flags) runs
// without error and writes files.
func TestScaffoldInitDefaultMode(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", testParamsSchemaYAML)
	setTestHome(t, home)

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{"dir": outDir},
		nil,
	)
	if err != nil {
		t.Fatalf("scaffold init default mode: %v", err)
	}
	// Files should be present
	if _, err := os.Stat(filepath.Join(outDir, "workflow.yaml")); err != nil {
		t.Errorf("expected workflow.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "tentacle.yaml")); err != nil {
		t.Errorf("expected tentacle.yaml: %v", err)
	}
}

// TestScaffoldInitNotFound verifies that initializing a nonexistent scaffold
// returns an error.
func TestScaffoldInitNotFound(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".tentacular", "scaffolds"), 0o755)
	setTestHome(t, home)

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("nonexistent-scaffold", "my-tentacle",
		map[string]string{"dir": outDir},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for nonexistent scaffold, got nil")
	}
}

// TestScaffoldInitDirOverride verifies that --dir places files in the
// specified directory instead of ~/tentacles/.
func TestScaffoldInitDirOverride(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", "")
	setTestHome(t, home)

	customDir := filepath.Join(t.TempDir(), "custom", "path")
	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{"dir": customDir},
		map[string]bool{"no-params": true},
	)
	if err != nil {
		t.Fatalf("scaffold init --dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(customDir, "workflow.yaml")); err != nil {
		t.Errorf("expected workflow.yaml at --dir path: %v", err)
	}
}

// TestScaffoldInitNameConflict verifies that initializing into an already-existing
// directory returns an error.
func TestScaffoldInitNameConflict(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", "")
	setTestHome(t, home)

	// Pre-create the target directory
	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{"dir": outDir},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for existing directory, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

// TestScaffoldInitPrivateScaffold verifies that a private scaffold is found
// and tentacle.yaml records source="private".
func TestScaffoldInitPrivateScaffold(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "our-monitor", "")
	setTestHome(t, home)

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("our-monitor", "my-tentacle",
		map[string]string{"dir": outDir},
		map[string]bool{"no-params": true},
	)
	if err != nil {
		t.Fatalf("scaffold init private: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "tentacle.yaml"))
	if err != nil {
		t.Fatalf("reading tentacle.yaml: %v", err)
	}
	if !strings.Contains(string(data), "private") {
		t.Errorf("expected tentacle.yaml to record source=private, got:\n%s", string(data))
	}
}

// TestScaffoldInitWithoutParamsSchema verifies that a scaffold without a
// params.schema.yaml initializes successfully.
func TestScaffoldInitWithoutParamsSchema(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "simple-scaffold", "") // no schema
	setTestHome(t, home)

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("simple-scaffold", "my-tentacle",
		map[string]string{"dir": outDir},
		nil,
	)
	if err != nil {
		t.Fatalf("scaffold init without schema: %v", err)
	}
	// workflow.yaml should exist
	if _, err := os.Stat(filepath.Join(outDir, "workflow.yaml")); err != nil {
		t.Errorf("expected workflow.yaml: %v", err)
	}
}

// TestScaffoldInitPrivatePublicPrecedence verifies that when the same scaffold
// name exists in both private and public sources, the private one wins.
func TestScaffoldInitPrivatePublicPrecedence(t *testing.T) {
	// Create the private scaffold (has local files).
	home, _ := scaffoldTestFixture(t, "shared-scaffold", "")
	setTestHome(t, home)

	// Also write a public index entry with the same name (no local Path).
	cacheDir := filepath.Join(home, ".tentacular", "cache")
	makeIndexFileAtPath(t, filepath.Join(cacheDir, "scaffolds-index.yaml"), []scaffold.ScaffoldEntry{
		{
			Name:        "shared-scaffold",
			DisplayName: "Shared Scaffold (public)",
			Description: "Public version of shared scaffold",
			Category:    "testing",
			Version:     "2.0", // different version to distinguish
			Author:      "randybias",
			Tags:        []string{},
		},
	})

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("shared-scaffold", "my-tentacle",
		map[string]string{"dir": outDir},
		map[string]bool{"no-params": true},
	)
	if err != nil {
		t.Fatalf("scaffold init precedence: %v", err)
	}

	// tentacle.yaml must record source=private (private won).
	data, err := os.ReadFile(filepath.Join(outDir, "tentacle.yaml"))
	if err != nil {
		t.Fatalf("reading tentacle.yaml: %v", err)
	}
	if !strings.Contains(string(data), "private") {
		t.Errorf("expected source=private in tentacle.yaml (private should take precedence), got:\n%s", string(data))
	}
	if strings.Contains(string(data), "2.0") {
		t.Errorf("expected private scaffold version (1.0), not public (2.0), got:\n%s", string(data))
	}
}

// TestScaffoldInitSourcePublic verifies that --source=public skips private
// scaffolds and searches the public index. Since public scaffolds don't have
// local files until after sync, it returns a descriptive "sync first" error.
func TestScaffoldInitSourcePublic(t *testing.T) {
	// Set up a private scaffold that should be ignored.
	home, _ := scaffoldTestFixture(t, "uptime-tracker", "")
	setTestHome(t, home)

	// Also write a matching public index entry (no local path).
	cacheDir := filepath.Join(home, ".tentacular", "cache")
	makeIndexFileAtPath(t, filepath.Join(cacheDir, "scaffolds-index.yaml"), []scaffold.ScaffoldEntry{
		{
			Name:        "uptime-tracker",
			DisplayName: "Uptime Tracker",
			Description: "Probe HTTP endpoints",
			Category:    "monitoring",
			Version:     "1.0",
			Author:      "randybias",
			Tags:        []string{},
		},
	})

	outDir := filepath.Join(t.TempDir(), "my-tentacle")
	err := makeScaffoldInitCmd("uptime-tracker", "my-tentacle",
		map[string]string{"dir": outDir, "source": "public"},
		map[string]bool{"no-params": true},
	)
	// Public scaffold has no local Path (sync hasn't downloaded files).
	// Expected: error mentioning "sync" (not "not found").
	if err == nil {
		t.Fatal("expected error for public scaffold without local files, got nil")
	}
	if !strings.Contains(err.Error(), "sync") {
		t.Errorf("expected 'sync' in error message (public scaffold not yet downloaded), got: %v", err)
	}
	// Must NOT say "not found" -- scaffold was found in the index but needs sync.
	if strings.Contains(err.Error(), "not found") {
		t.Errorf("scaffold should be found in public index but need sync, got: %v", err)
	}
}
