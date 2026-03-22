// Unit tests for scaffold discovery functions.
//
// Uses temp directories to simulate private scaffold storage and a local
// quickstarts index, avoiding any real filesystem or network dependencies.
// Covers:
//   - ReadPrivateScaffolds: empty dir, valid scaffold, malformed yaml, mixed
//   - ReadPublicScaffolds: missing file, valid index, invalid yaml
//   - FindScaffold: private win, public fallback, source filter, not found
//   - ListScaffolds: merge order, category filter, tag filter
//   - SearchScaffolds: name match, description match, no match

package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- test helpers ---

// makeIndexFile writes a scaffolds-index.yaml to a temp file and returns the path.
func makeIndexFile(t *testing.T, scaffolds []ScaffoldEntry) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "scaffolds-index.yaml")
	content := buildIndexYAML(scaffolds)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	return path
}

func buildScaffoldYAML(s ScaffoldEntry) string {
	var sb strings.Builder
	sb.WriteString("name: " + s.Name + "\n")
	sb.WriteString("displayName: " + s.DisplayName + "\n")
	sb.WriteString("description: " + s.Description + "\n")
	sb.WriteString("category: " + s.Category + "\n")
	sb.WriteString("tags:")
	for _, tag := range s.Tags {
		sb.WriteString("\n  - " + tag)
	}
	sb.WriteString("\nauthor: " + s.Author + "\n")
	sb.WriteString("version: \"" + s.Version + "\"\n")
	return sb.String()
}

func buildIndexYAML(scaffolds []ScaffoldEntry) string {
	var sb strings.Builder
	sb.WriteString("version: \"1\"\ngenerated: \"2026-01-01\"\nscaffolds:\n")
	for _, s := range scaffolds {
		sb.WriteString("  - name: " + s.Name + "\n")
		sb.WriteString("    displayName: " + s.DisplayName + "\n")
		sb.WriteString("    description: " + s.Description + "\n")
		sb.WriteString("    category: " + s.Category + "\n")
		sb.WriteString("    author: " + s.Author + "\n")
		sb.WriteString("    version: \"" + s.Version + "\"\n")
		if len(s.Tags) == 0 {
			sb.WriteString("    tags: []\n")
		} else {
			sb.WriteString("    tags:\n")
			for _, tag := range s.Tags {
				sb.WriteString("      - " + tag + "\n")
			}
		}
	}
	return sb.String()
}

var testScaffoldA = ScaffoldEntry{
	Name:        "uptime-tracker",
	DisplayName: "Uptime Tracker",
	Description: "Probe HTTP endpoints for uptime monitoring",
	Category:    "monitoring",
	Tags:        []string{"uptime-monitoring", "postgres-state"},
	Author:      "randybias",
	Version:     "1.0",
}

var testScaffoldB = ScaffoldEntry{
	Name:        "github-security-digest",
	DisplayName: "GitHub Security Digest",
	Description: "Weekly security digest from GitHub advisories",
	Category:    "security",
	Tags:        []string{"github", "security"},
	Author:      "randybias",
	Version:     "1.0",
}

// --- ReadPrivateScaffolds ---

// TestReadPrivateScaffoldsNonExistentDirReturnsNil verifies that a missing
// private scaffolds directory returns nil without error.
func TestReadPrivateScaffoldsNonExistentDirReturnsNil(t *testing.T) {
	// Point PrivateScaffoldsDir at a non-existent path via HOME override
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	results, err := ReadPrivateScaffolds()
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for missing dir, got %d entries", len(results))
	}
}

// TestReadPrivateScaffoldsEmptyDirReturnsNil verifies that an existing but
// empty private scaffolds directory returns nil without error.
func TestReadPrivateScaffoldsEmptyDirReturnsNil(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	results, err := ReadPrivateScaffolds()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestReadPrivateScaffoldsValidScaffold verifies that a directory containing a
// valid scaffold.yaml is returned with Source="private".
func TestReadPrivateScaffoldsValidScaffold(t *testing.T) {
	tmpHome := t.TempDir()
	scaffoldsDir := filepath.Join(tmpHome, ".tentacular", "scaffolds")
	if err := os.MkdirAll(scaffoldsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sDir := filepath.Join(scaffoldsDir, "uptime-tracker")
	if err := os.MkdirAll(sDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sDir, "scaffold.yaml"),
		[]byte(buildScaffoldYAML(testScaffoldA)), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	results, err := ReadPrivateScaffolds()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "uptime-tracker" {
		t.Errorf("Name: got %q, want %q", results[0].Name, "uptime-tracker")
	}
	if results[0].Source != "private" {
		t.Errorf("Source: got %q, want %q", results[0].Source, "private")
	}
}

// TestReadPrivateScaffoldsMalformedYAMLSkipped verifies that a directory with
// malformed scaffold.yaml is silently skipped (not an error).
func TestReadPrivateScaffoldsMalformedYAMLSkipped(t *testing.T) {
	tmpHome := t.TempDir()
	scaffoldsDir := filepath.Join(tmpHome, ".tentacular", "scaffolds")
	sDir := filepath.Join(scaffoldsDir, "bad-scaffold")
	if err := os.MkdirAll(sDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sDir, "scaffold.yaml"),
		[]byte(":::not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	results, err := ReadPrivateScaffolds()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected malformed scaffold to be skipped, got %d results", len(results))
	}
}

// TestReadPrivateScaffoldsNonDirFilesSkipped verifies that plain files in the
// scaffolds directory (not subdirectories) are ignored.
func TestReadPrivateScaffoldsNonDirFilesSkipped(t *testing.T) {
	tmpHome := t.TempDir()
	scaffoldsDir := filepath.Join(tmpHome, ".tentacular", "scaffolds")
	if err := os.MkdirAll(scaffoldsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a plain file (not a directory) in scaffolds dir
	if err := os.WriteFile(filepath.Join(scaffoldsDir, "stray-file.txt"),
		[]byte("not a scaffold"), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	results, err := ReadPrivateScaffolds()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- ReadPublicScaffolds ---

// TestReadPublicScaffoldsMissingFileReturnsNil verifies that a non-existent
// index file returns nil without error.
func TestReadPublicScaffoldsMissingFileReturnsNil(t *testing.T) {
	results, err := ReadPublicScaffolds("/nonexistent/path/scaffolds-index.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %d", len(results))
	}
}

// TestReadPublicScaffoldsValidIndex verifies that a valid index file is parsed
// and all entries have Source="public".
func TestReadPublicScaffoldsValidIndex(t *testing.T) {
	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA, testScaffoldB})

	results, err := ReadPublicScaffolds(indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Source != "public" {
			t.Errorf("Source: got %q, want %q for %s", r.Source, "public", r.Name)
		}
	}
}

// TestReadPublicScaffoldsInvalidYAML verifies that invalid YAML returns an error.
func TestReadPublicScaffoldsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scaffolds-index.yaml")
	if err := os.WriteFile(path, []byte(":::not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadPublicScaffolds(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// --- FindScaffold ---

// TestFindScaffoldPrivateWins verifies that when both private and public have
// the same scaffold name, private is returned.
func TestFindScaffoldPrivateWins(t *testing.T) {
	tmpHome := t.TempDir()
	scaffoldsDir := filepath.Join(tmpHome, ".tentacular", "scaffolds")
	sDir := filepath.Join(scaffoldsDir, "uptime-tracker")
	if err := os.MkdirAll(sDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sDir, "scaffold.yaml"),
		[]byte(buildScaffoldYAML(testScaffoldA)), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Public index also has uptime-tracker
	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA})

	entry, err := FindScaffold("uptime-tracker", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Source != "private" {
		t.Errorf("Source: got %q, want %q (private should win)", entry.Source, "private")
	}
}

// TestFindScaffoldFallsBackToPublic verifies that a scaffold not in private
// is found in the public index.
func TestFindScaffoldFallsBackToPublic(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA})

	entry, err := FindScaffold("uptime-tracker", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Source != "public" {
		t.Errorf("Source: got %q, want %q", entry.Source, "public")
	}
}

// TestFindScaffoldSourcePublicBypassesPrivate verifies that --source=public
// skips private scaffolds even when private has a matching name.
func TestFindScaffoldSourcePublicBypassesPrivate(t *testing.T) {
	tmpHome := t.TempDir()
	scaffoldsDir := filepath.Join(tmpHome, ".tentacular", "scaffolds")
	sDir := filepath.Join(scaffoldsDir, "uptime-tracker")
	if err := os.MkdirAll(sDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sDir, "scaffold.yaml"),
		[]byte(buildScaffoldYAML(testScaffoldA)), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA})

	entry, err := FindScaffold("uptime-tracker", "public", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Source != "public" {
		t.Errorf("Source: got %q, want %q (should use public)", entry.Source, "public")
	}
}

// TestFindScaffoldNotFound verifies that searching for a nonexistent scaffold
// returns an error.
func TestFindScaffoldNotFound(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA})

	_, err := FindScaffold("nonexistent-scaffold", "", indexPath)
	if err == nil {
		t.Fatal("expected error for nonexistent scaffold, got nil")
	}
}

// --- ListScaffolds ---

// TestListScaffoldsPrivateFirst verifies that private scaffolds appear before
// public scaffolds in the merged result.
func TestListScaffoldsPrivateFirst(t *testing.T) {
	tmpHome := t.TempDir()
	scaffoldsDir := filepath.Join(tmpHome, ".tentacular", "scaffolds")
	sDir := filepath.Join(scaffoldsDir, "private-monitor")
	if err := os.MkdirAll(sDir, 0o755); err != nil {
		t.Fatal(err)
	}
	privateScaffold := ScaffoldEntry{
		Name:     "private-monitor",
		Category: "monitoring",
		Version:  "1.0",
	}
	if err := os.WriteFile(filepath.Join(sDir, "scaffold.yaml"),
		[]byte(buildScaffoldYAML(privateScaffold)), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA})

	results, err := ListScaffolds("", "", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Source != "private" {
		t.Errorf("first result Source: got %q, want %q (private first)", results[0].Source, "private")
	}
}

// TestListScaffoldsCategoryFilter verifies that --category filters results
// to only matching scaffolds.
func TestListScaffoldsCategoryFilter(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA, testScaffoldB})

	results, err := ListScaffolds("", "monitoring", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for category=monitoring, got %d", len(results))
	}
	if results[0].Name != "uptime-tracker" {
		t.Errorf("Name: got %q, want %q", results[0].Name, "uptime-tracker")
	}
}

// TestListScaffoldsTagFilter verifies that --tag filters to scaffolds with
// the matching tag (case-insensitive).
func TestListScaffoldsTagFilter(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA, testScaffoldB})

	results, err := ListScaffolds("", "", "postgres-state", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for tag=postgres-state, got %d", len(results))
	}
	if results[0].Name != "uptime-tracker" {
		t.Errorf("Name: got %q, want %q", results[0].Name, "uptime-tracker")
	}
}

// TestListScaffoldsEmptyPrivateNoError verifies that --source=private with
// no private scaffolds returns an empty slice, not an error.
func TestListScaffoldsEmptyPrivateNoError(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA})

	results, err := ListScaffolds("private", "", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error for empty private: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- SearchScaffolds ---

// TestSearchScaffoldsNameMatch verifies that a query matching the scaffold name
// returns that scaffold.
func TestSearchScaffoldsNameMatch(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA, testScaffoldB})

	results, err := SearchScaffolds("uptime", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for query=uptime, got %d", len(results))
	}
	if results[0].Name != "uptime-tracker" {
		t.Errorf("Name: got %q, want %q", results[0].Name, "uptime-tracker")
	}
}

// TestSearchScaffoldsDescriptionMatch verifies that a query matching the
// description is returned.
func TestSearchScaffoldsDescriptionMatch(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA, testScaffoldB})

	results, err := SearchScaffolds("advisories", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for query=advisories, got %d", len(results))
	}
	if results[0].Name != "github-security-digest" {
		t.Errorf("Name: got %q, want %q", results[0].Name, "github-security-digest")
	}
}

// TestSearchScaffoldsNoMatch verifies that a query with no matches returns
// an empty slice (not an error).
func TestSearchScaffoldsNoMatch(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA, testScaffoldB})

	results, err := SearchScaffolds("xyznonexistent", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for no-match query, got %d", len(results))
	}
}

// TestSearchScaffoldsTagMatch verifies that a query matching a tag is returned.
func TestSearchScaffoldsTagMatch(t *testing.T) {
	tmpHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpHome, ".tentacular", "scaffolds"), 0o755)

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	indexPath := makeIndexFile(t, []ScaffoldEntry{testScaffoldA, testScaffoldB})

	results, err := SearchScaffolds("postgres-state", "", indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for query=postgres-state, got %d", len(results))
	}
	if results[0].Name != "uptime-tracker" {
		t.Errorf("Name: got %q, want %q", results[0].Name, "uptime-tracker")
	}
}

// TestEnsurePrivateScaffoldsDirPermissions verifies that EnsurePrivateScaffoldsDir
// creates the directory with 0700 permissions so other users on shared systems
// cannot read private scaffold content.
func TestEnsurePrivateScaffoldsDirPermissions(t *testing.T) {
	tmpHome := t.TempDir()

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	dir, err := EnsurePrivateScaffoldsDir()
	if err != nil {
		t.Fatalf("EnsurePrivateScaffoldsDir: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0o700 {
		t.Errorf("expected 0700 permissions on private scaffolds dir, got %04o", mode)
	}
}

// TestEnsurePrivateScaffoldsDirIdempotent verifies that calling
// EnsurePrivateScaffoldsDir twice does not fail and preserves the directory.
func TestEnsurePrivateScaffoldsDirIdempotent(t *testing.T) {
	tmpHome := t.TempDir()

	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	dir1, err := EnsurePrivateScaffoldsDir()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	dir2, err := EnsurePrivateScaffoldsDir()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if dir1 != dir2 {
		t.Errorf("path changed between calls: %q vs %q", dir1, dir2)
	}
}
