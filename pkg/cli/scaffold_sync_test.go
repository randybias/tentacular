// Integration tests for tntc scaffold sync.
//
// Covers 4 cases from design doc Section 12.9:
//   - Fresh sync: no cached quickstarts, fetches index and reports count
//   - Stale cache: cache exists but expired, forces re-fetch
//   - Force sync: cache is fresh but sync always forces re-fetch
//   - Network error: returns clear error, preserves existing cache

package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testIndexYAML is a minimal scaffolds-index.yaml for sync tests.
const testIndexYAML = `version: "1"
generated: "2026-01-01"
scaffolds:
  - name: uptime-tracker
    displayName: Uptime Tracker
    description: Probe HTTP endpoints for uptime monitoring
    category: monitoring
    version: "1.0"
    author: randybias
    tags:
      - uptime-monitoring
  - name: github-security-digest
    displayName: GitHub Security Digest
    description: Weekly digest from GitHub security advisories
    category: security
    version: "1.0"
    author: randybias
    tags:
      - github
      - security
`

// setupSyncTestEnv creates a temp HOME with a tentacular config pointing to
// the given server URL. Returns the home directory.
func setupSyncTestEnv(t *testing.T, serverURL string) string {
	t.Helper()
	home := t.TempDir()
	setTestHome(t, home)

	// Write a config.yaml that points scaffold URL at our test server.
	tentacularDir := filepath.Join(home, ".tentacular")
	if err := os.MkdirAll(tentacularDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configYAML := fmt.Sprintf("scaffold:\n  url: %s\n  cacheTTL: 1h\n", serverURL)
	if err := os.WriteFile(filepath.Join(tentacularDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	return home
}

// cachedIndexPath returns the expected cache path in the given home dir.
func cachedIndexPath(home string) string {
	return filepath.Join(home, ".tentacular", "cache", "scaffolds-index.yaml")
}

// runSyncCmd runs tntc scaffold sync and returns stdout + error.
func runSyncCmd(t *testing.T) (string, error) {
	t.Helper()
	cmd := newScaffoldSyncCmd()
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	return out, runErr
}

// TestScaffoldSyncFresh verifies that sync with no existing cache fetches the
// remote index and reports the correct scaffold count.
func TestScaffoldSyncFresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/scaffolds-index.yaml" {
			w.Header().Set("Content-Type", "text/yaml")
			_, _ = w.Write([]byte(testIndexYAML))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	home := setupSyncTestEnv(t, srv.URL)

	// Confirm no cache exists yet.
	if _, err := os.Stat(cachedIndexPath(home)); !os.IsNotExist(err) {
		t.Fatal("expected no cache before sync")
	}

	out, err := runSyncCmd(t)
	if err != nil {
		t.Fatalf("scaffold sync: %v\nOutput:\n%s", err, out)
	}

	// Should report 2 scaffolds (from testIndexYAML).
	if !strings.Contains(out, "2") {
		t.Errorf("expected scaffold count 2 in output, got:\n%s", out)
	}

	// Cache file should now exist.
	if _, err := os.Stat(cachedIndexPath(home)); os.IsNotExist(err) {
		t.Error("expected cache file to be written after sync")
	}
}

// TestScaffoldSyncUpdateStale verifies that sync re-fetches even when a stale
// cache file exists (older than cache TTL).
func TestScaffoldSyncUpdateStale(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/scaffolds-index.yaml" {
			fetchCount++
			w.Header().Set("Content-Type", "text/yaml")
			_, _ = w.Write([]byte(testIndexYAML))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	home := setupSyncTestEnv(t, srv.URL)

	// Write a stale cache file backdated by 2 hours (beyond the 1h TTL).
	cacheDir := filepath.Join(home, ".tentacular", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staleData := []byte("version: \"1\"\nscaffolds: []\n")
	cachePath := cachedIndexPath(home)
	if err := os.WriteFile(cachePath, staleData, 0o644); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(cachePath, staleTime, staleTime); err != nil {
		t.Fatalf("backdating cache file: %v", err)
	}

	out, err := runSyncCmd(t)
	if err != nil {
		t.Fatalf("scaffold sync stale: %v\nOutput:\n%s", err, out)
	}

	// Sync always force-fetches (noCache=true in client.Sync()), so fetchCount must be 1.
	if fetchCount != 1 {
		t.Errorf("expected exactly 1 HTTP fetch, got %d", fetchCount)
	}
	if !strings.Contains(out, "Updated scaffolds index") {
		t.Errorf("expected success message, got:\n%s", out)
	}
}

// TestScaffoldSyncForce verifies that sync always fetches from remote even
// when a fresh valid cache exists.
func TestScaffoldSyncForce(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/scaffolds-index.yaml" {
			fetchCount++
			w.Header().Set("Content-Type", "text/yaml")
			_, _ = w.Write([]byte(testIndexYAML))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	home := setupSyncTestEnv(t, srv.URL)

	// Write a fresh cache file (current time = valid within 1h TTL).
	cacheDir := filepath.Join(home, ".tentacular", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	freshData := []byte("version: \"1\"\nscaffolds: []\n")
	if err := os.WriteFile(cachedIndexPath(home), freshData, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runSyncCmd(t)
	if err != nil {
		t.Fatalf("scaffold sync force: %v\nOutput:\n%s", err, out)
	}

	// sync must bypass cache and fetch remotely (client.Sync calls FetchIndex(noCache=true)).
	if fetchCount != 1 {
		t.Errorf("expected 1 forced fetch (sync always bypasses cache), got %d", fetchCount)
	}
	// Updated count should reflect the remote index (2 scaffolds), not the stale local empty one.
	if !strings.Contains(out, "2") {
		t.Errorf("expected count 2 from remote index, got:\n%s", out)
	}
}

// TestScaffoldSyncNetworkError verifies that sync returns a clear error when
// the remote is unreachable and does not corrupt existing cache.
func TestScaffoldSyncNetworkError(t *testing.T) {
	// Use a server that immediately closes connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return 503 to simulate remote unavailability.
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	home := setupSyncTestEnv(t, srv.URL)

	// Write existing cache data that should be preserved.
	cacheDir := filepath.Join(home, ".tentacular", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingCache := []byte(testIndexYAML)
	if err := os.WriteFile(cachedIndexPath(home), existingCache, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runSyncCmd(t)
	if err == nil {
		t.Fatal("expected error on network failure, got nil")
	}

	// Error message should be informative.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "sync") && !strings.Contains(errMsg, "HTTP") {
		t.Errorf("expected descriptive error message, got: %s", errMsg)
	}

	// Existing cache should be preserved (not deleted or overwritten with invalid data).
	data, readErr := os.ReadFile(cachedIndexPath(home))
	if readErr != nil {
		t.Fatalf("cache file was deleted on error: %v", readErr)
	}
	if string(data) != string(existingCache) {
		t.Errorf("cache was overwritten on error:\ngot:  %s\nwant: %s", data, existingCache)
	}
}
