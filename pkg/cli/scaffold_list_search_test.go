// Integration tests for tntc scaffold list and tntc scaffold search.
//
// Covers 10 cases from design doc Section 12.5:
//   - List all (both sources shown, private first)
//   - List private only
//   - List public only
//   - List with category filter
//   - List with tag filter
//   - List JSON output
//   - List empty private (no error)
//   - Search matching
//   - Search no match
//   - Search partial match

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/scaffold"
)

// captureStdout redirects os.Stdout to a pipe, runs fn, and returns what was printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

// makeIndexFileAtPath writes a scaffolds-index.yaml at the given path.
func makeIndexFileAtPath(t *testing.T, path string, scaffolds []scaffold.ScaffoldEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	sb.WriteString("version: \"1\"\ngenerated: \"2026-01-01\"\nscaffolds:\n")
	for _, s := range scaffolds {
		sb.WriteString("  - name: " + s.Name + "\n" +
			"    displayName: " + s.DisplayName + "\n" +
			"    description: " + s.Description + "\n" +
			"    category: " + s.Category + "\n" +
			"    author: " + s.Author + "\n" +
			"    version: \"" + s.Version + "\"\n")
		if len(s.Tags) == 0 {
			sb.WriteString("    tags: []\n")
		} else {
			sb.WriteString("    tags:\n")
			for _, tag := range s.Tags {
				sb.WriteString("      - " + tag + "\n")
			}
		}
	}
	out := sb.String()
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupListTestEnv creates a home with a private scaffold and a cache index.
// Returns: home dir.
func setupListTestEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()

	// Private scaffold
	privDir := filepath.Join(home, ".tentacular", "scaffolds", "private-monitor")
	if err := os.MkdirAll(privDir, 0o755); err != nil {
		t.Fatal(err)
	}
	privMeta := "name: private-monitor\ndisplayName: Private Monitor\n" +
		"description: A private monitoring scaffold\ncategory: monitoring\n" +
		"version: \"1.0\"\nauthor: test\ntags:\n  - internal\n"
	if err := os.WriteFile(filepath.Join(privDir, "scaffold.yaml"),
		[]byte(privMeta), 0o644); err != nil {
		t.Fatal(err)
	}

	// Public cache index
	cacheDir := filepath.Join(home, ".tentacular", "cache")
	idx := filepath.Join(cacheDir, "scaffolds-index.yaml")
	makeIndexFileAtPath(t, idx, []scaffold.ScaffoldEntry{
		{
			Name:        "uptime-tracker",
			DisplayName: "Uptime Tracker",
			Description: "Probe HTTP endpoints for uptime monitoring",
			Category:    "monitoring",
			Tags:        []string{"uptime-monitoring", "postgres-state"},
			Version:     "1.0",
			Author:      "randybias",
		},
		{
			Name:        "github-security-digest",
			DisplayName: "GitHub Security Digest",
			Description: "Weekly digest from GitHub security advisories",
			Category:    "security",
			Tags:        []string{"github", "security"},
			Version:     "1.0",
			Author:      "randybias",
		},
	})

	return home
}

// runListCmd runs tntc scaffold list and returns stdout + error.
func runListCmd(t *testing.T, home string, flags map[string]string, boolFlags map[string]bool) (string, error) {
	t.Helper()
	setTestHome(t, home)

	cmd := newScaffoldListCmd()
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("setting flag %s: %v", k, err)
		}
	}
	for k := range boolFlags {
		if err := cmd.Flags().Set(k, "true"); err != nil {
			t.Fatalf("setting bool flag %s: %v", k, err)
		}
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	return out, runErr
}

// runSearchCmd runs tntc scaffold search and returns stdout + error.
func runSearchCmd(t *testing.T, home, query string) (string, error) {
	t.Helper()
	setTestHome(t, home)

	cmd := newScaffoldSearchCmd()
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, []string{query})
	})
	return out, runErr
}

// TestScaffoldListAll verifies that listing without filters shows both private
// and public scaffolds with private appearing first.
func TestScaffoldListAll(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runListCmd(t, home, map[string]string{"source": "all"}, nil)
	if err != nil {
		t.Fatalf("scaffold list: %v", err)
	}
	if !strings.Contains(out, "private-monitor") {
		t.Errorf("expected private scaffold in output, got:\n%s", out)
	}
	if !strings.Contains(out, "uptime-tracker") {
		t.Errorf("expected public scaffold in output, got:\n%s", out)
	}
	// Private should appear before public
	privIdx := strings.Index(out, "private-monitor")
	pubIdx := strings.Index(out, "uptime-tracker")
	if privIdx > pubIdx {
		t.Errorf("expected private scaffold before public in output:\n%s", out)
	}
}

// TestScaffoldListPrivateOnly verifies that --source=private shows only
// private scaffolds.
func TestScaffoldListPrivateOnly(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runListCmd(t, home, map[string]string{"source": "private"}, nil)
	if err != nil {
		t.Fatalf("scaffold list --source=private: %v", err)
	}
	if strings.Contains(out, "uptime-tracker") {
		t.Errorf("expected no public scaffolds with --source=private, got:\n%s", out)
	}
	if !strings.Contains(out, "private-monitor") {
		t.Errorf("expected private scaffold in output, got:\n%s", out)
	}
}

// TestScaffoldListPublicOnly verifies that --source=public shows only
// public scaffolds.
func TestScaffoldListPublicOnly(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runListCmd(t, home, map[string]string{"source": "public"}, nil)
	if err != nil {
		t.Fatalf("scaffold list --source=public: %v", err)
	}
	if strings.Contains(out, "private-monitor") {
		t.Errorf("expected no private scaffolds with --source=public, got:\n%s", out)
	}
	if !strings.Contains(out, "uptime-tracker") {
		t.Errorf("expected public scaffold in output, got:\n%s", out)
	}
}

// TestScaffoldListCategoryFilter verifies that --category filters to only
// matching scaffolds.
func TestScaffoldListCategoryFilter(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runListCmd(t, home, map[string]string{"source": "public", "category": "security"}, nil)
	if err != nil {
		t.Fatalf("scaffold list --category: %v", err)
	}
	if strings.Contains(out, "uptime-tracker") {
		t.Errorf("expected no monitoring scaffolds when filtering for security, got:\n%s", out)
	}
	if !strings.Contains(out, "github-security-digest") {
		t.Errorf("expected security scaffold in output, got:\n%s", out)
	}
}

// TestScaffoldListTagFilter verifies that --tag filters to scaffolds with
// the matching tag.
func TestScaffoldListTagFilter(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runListCmd(t, home, map[string]string{"source": "public", "tag": "postgres-state"}, nil)
	if err != nil {
		t.Fatalf("scaffold list --tag: %v", err)
	}
	if !strings.Contains(out, "uptime-tracker") {
		t.Errorf("expected uptime-tracker with postgres-state tag, got:\n%s", out)
	}
	if strings.Contains(out, "github-security-digest") {
		t.Errorf("expected github-security-digest NOT to appear, got:\n%s", out)
	}
}

// TestScaffoldListJSONOutput verifies that --json produces valid JSON array.
func TestScaffoldListJSONOutput(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runListCmd(t, home, map[string]string{"source": "public"}, map[string]bool{"json": true})
	if err != nil {
		t.Fatalf("scaffold list --json: %v", err)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("expected valid JSON array, got: %v\nOutput:\n%s", err, out)
	}
	if len(result) == 0 {
		t.Error("expected non-empty JSON array")
	}
}

// TestScaffoldListEmptyPrivateNoError verifies that listing private-only with
// no private scaffolds returns success (not error).
func TestScaffoldListEmptyPrivateNoError(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".tentacular", "scaffolds"), 0o755)
	out, err := runListCmd(t, home, map[string]string{"source": "private"}, nil)
	if err != nil {
		t.Fatalf("expected no error for empty private list, got: %v", err)
	}
	_ = out // output may say "No scaffolds found" or be empty -- both OK
}

// TestScaffoldSearchMatching verifies that a matching query returns results.
func TestScaffoldSearchMatching(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runSearchCmd(t, home, "uptime")
	if err != nil {
		t.Fatalf("scaffold search: %v", err)
	}
	if !strings.Contains(out, "uptime-tracker") {
		t.Errorf("expected uptime-tracker in search results, got:\n%s", out)
	}
}

// TestScaffoldSearchNoMatch verifies that a query with no matches returns
// success (not an error), with empty or "no results" output.
func TestScaffoldSearchNoMatch(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runSearchCmd(t, home, "xyznonexistent")
	if err != nil {
		t.Fatalf("expected no error for no-match search, got: %v\nOutput:\n%s", err, out)
	}
}

// TestScaffoldSearchPartialMatch verifies that a partial query matches
// scaffolds with "monitor" in name, description, or tags.
func TestScaffoldSearchPartialMatch(t *testing.T) {
	home := setupListTestEnv(t)
	out, err := runSearchCmd(t, home, "monitor")
	if err != nil {
		t.Fatalf("scaffold search monitor: %v", err)
	}
	if !strings.Contains(out, "private-monitor") {
		t.Errorf("expected private-monitor in search results for 'monitor', got:\n%s", out)
	}
}
