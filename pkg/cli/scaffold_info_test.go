// Integration tests for tntc scaffold info.
//
// Covers 5 cases from design doc Section 12.6:
//   - Info public scaffold
//   - Info private scaffold (source label = private)
//   - Info with params.schema.yaml (parameter summary displayed)
//   - Info without params.schema.yaml (no parameters section)
//   - Info not found (error)

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInfoCmd runs tntc scaffold info <name> and returns stdout + error.
func runInfoCmd(t *testing.T, home, name string) (string, error) {
	t.Helper()
	setTestHome(t, home)

	cmd := newScaffoldInfoCmd()
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, []string{name})
	})
	return out, runErr
}

// setupInfoTestEnv creates a temp home with a private scaffold and an index
// for a second public scaffold. Returns home dir and index path.
func setupInfoTestEnv(t *testing.T, includeSchema bool) string {
	t.Helper()
	home := t.TempDir()

	// Private scaffold with schema
	privDir := filepath.Join(home, ".tentacular", "scaffolds", "our-monitor")
	if err := os.MkdirAll(privDir, 0o755); err != nil {
		t.Fatal(err)
	}
	privMeta := "name: our-monitor\ndisplayName: Our Monitor\n" +
		"description: Internal uptime monitoring\ncategory: monitoring\n" +
		"version: \"1.0\"\nauthor: internal\ntags: []\n"
	if err := os.WriteFile(filepath.Join(privDir, "scaffold.yaml"),
		[]byte(privMeta), 0o644); err != nil {
		t.Fatal(err)
	}
	if includeSchema {
		if err := os.WriteFile(filepath.Join(privDir, "params.schema.yaml"),
			[]byte(testParamsSchemaYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Cache index with one public scaffold
	cacheDir := filepath.Join(home, ".tentacular", "cache")
	idxPath := filepath.Join(cacheDir, "scaffolds-index.yaml")
	makeIndexFileAtPath(t, idxPath, nil)

	return home
}

// TestScaffoldInfoPrivateScaffold verifies that info for a private scaffold
// shows metadata with "private" in the source label.
func TestScaffoldInfoPrivateScaffold(t *testing.T) {
	home := setupInfoTestEnv(t, false)
	out, err := runInfoCmd(t, home, "our-monitor")
	if err != nil {
		t.Fatalf("scaffold info: %v", err)
	}
	if !strings.Contains(out, "our-monitor") {
		t.Errorf("expected scaffold name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "private") {
		t.Errorf("expected 'private' source label in output, got:\n%s", out)
	}
}

// TestScaffoldInfoWithParams verifies that info for a scaffold with
// params.schema.yaml shows a parameter summary.
func TestScaffoldInfoWithParams(t *testing.T) {
	home := setupInfoTestEnv(t, true) // includes schema
	out, err := runInfoCmd(t, home, "our-monitor")
	if err != nil {
		t.Fatalf("scaffold info: %v", err)
	}
	if !strings.Contains(out, "Parameters") {
		t.Errorf("expected 'Parameters' section in output for scaffold with schema, got:\n%s", out)
	}
	// Should mention at least one param name
	if !strings.Contains(out, "endpoints") && !strings.Contains(out, "probe_schedule") {
		t.Errorf("expected parameter names in output, got:\n%s", out)
	}
}

// TestScaffoldInfoWithoutParams verifies that info for a scaffold without
// params.schema.yaml does not show a parameters section.
func TestScaffoldInfoWithoutParams(t *testing.T) {
	home := setupInfoTestEnv(t, false) // no schema
	out, err := runInfoCmd(t, home, "our-monitor")
	if err != nil {
		t.Fatalf("scaffold info: %v", err)
	}
	// No parameters section expected
	if strings.Contains(out, "Parameters:") {
		t.Errorf("expected no 'Parameters:' section for scaffold without schema, got:\n%s", out)
	}
}

// TestScaffoldInfoNotFound verifies that info for a nonexistent scaffold
// returns an error.
func TestScaffoldInfoNotFound(t *testing.T) {
	home := setupInfoTestEnv(t, false)
	_, err := runInfoCmd(t, home, "nonexistent-scaffold")
	if err == nil {
		t.Fatal("expected error for nonexistent scaffold, got nil")
	}
}

// TestScaffoldInfoShowsMetadata verifies that the info output includes
// the scaffold metadata fields (name, description, category, version).
func TestScaffoldInfoShowsMetadata(t *testing.T) {
	home := setupInfoTestEnv(t, false)
	out, err := runInfoCmd(t, home, "our-monitor")
	if err != nil {
		t.Fatalf("scaffold info: %v", err)
	}
	for _, expected := range []string{"our-monitor", "monitoring", "1.0", "Internal uptime monitoring"} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q in info output, got:\n%s", expected, out)
		}
	}
}
