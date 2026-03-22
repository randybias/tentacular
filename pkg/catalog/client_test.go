// Unit tests for catalog client.
//
// Uses httptest.NewServer to test HTTP fetching without hitting real servers.
// Covers:
//   - NewClient: defaults and config overrides (URL, CacheTTL)
//   - httpGet: success, non-200 status, body reading
//   - FetchIndex: remote fetch, YAML parsing, caching, cache expiry
//   - FetchFile: path concatenation and fetch

package catalog

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- NewClient ---

// TestNewClientDefaults verifies that NewClient uses the default base URL
// and cache TTL when no config overrides are provided.
func TestNewClientDefaults(t *testing.T) {
	c := NewClient(CatalogConfig{})
	if c.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL: got %q, want %q", c.BaseURL, DefaultBaseURL)
	}
	if c.CacheTTL != DefaultCacheTTL {
		t.Errorf("CacheTTL: got %v, want %v", c.CacheTTL, DefaultCacheTTL)
	}
}

// TestNewClientConfigOverrides verifies that URL and CacheTTL from config
// take precedence over defaults.
func TestNewClientConfigOverrides(t *testing.T) {
	c := NewClient(CatalogConfig{
		URL:      "https://example.com/catalog",
		CacheTTL: "30m",
	})
	if c.BaseURL != "https://example.com/catalog" {
		t.Errorf("BaseURL: got %q, want %q", c.BaseURL, "https://example.com/catalog")
	}
	if c.CacheTTL != 30*time.Minute {
		t.Errorf("CacheTTL: got %v, want %v", c.CacheTTL, 30*time.Minute)
	}
}

// TestNewClientInvalidCacheTTLUsesDefault verifies that an unparseable
// CacheTTL string falls back to the default TTL.
func TestNewClientInvalidCacheTTLUsesDefault(t *testing.T) {
	c := NewClient(CatalogConfig{CacheTTL: "not-a-duration"})
	if c.CacheTTL != DefaultCacheTTL {
		t.Errorf("CacheTTL: got %v, want default %v", c.CacheTTL, DefaultCacheTTL)
	}
}

// --- httpGet ---

// TestHttpGetSuccess verifies that httpGet returns the response body
// for a 200 OK response.
func TestHttpGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	c := &Client{}
	data, err := c.httpGet(srv.URL)
	if err != nil {
		t.Fatalf("httpGet: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("body: got %q, want %q", string(data), "hello")
	}
}

// TestHttpGetNon200ReturnsError verifies that httpGet returns an error
// containing the HTTP status code for non-200 responses.
func TestHttpGetNon200ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{}
	_, err := c.httpGet(srv.URL)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// TestHttpGetConnectionError verifies that httpGet returns an error
// when the server is unreachable.
func TestHttpGetConnectionError(t *testing.T) {
	c := &Client{}
	_, err := c.httpGet("http://127.0.0.1:1") // nothing listening
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// --- FetchIndex ---

// catalogYAML is a minimal catalog index for testing.
const catalogYAML = `version: "1"
generated: "2026-01-01"
templates:
  - name: hello-world
    displayName: Hello World
    description: A simple starter template
    category: starter
    tags: [beginner]
    author: test
    complexity: basic
    path: templates/hello-world
    files: [workflow.yaml]
`

// TestFetchIndexFromRemote verifies that FetchIndex fetches and parses
// the catalog index from a remote HTTP server.
func TestFetchIndexFromRemote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(catalogYAML))
	}))
	defer srv.Close()

	c := &Client{
		BaseURL:  srv.URL,
		CacheDir: "", // no caching
		CacheTTL: time.Hour,
	}

	idx, err := c.FetchIndex(true)
	if err != nil {
		t.Fatalf("FetchIndex: %v", err)
	}
	if idx.Version != "1" {
		t.Errorf("Version: got %q, want %q", idx.Version, "1")
	}
	if len(idx.Templates) != 1 {
		t.Fatalf("Templates: got %d, want 1", len(idx.Templates))
	}
	if idx.Templates[0].Name != "hello-world" {
		t.Errorf("Template name: got %q, want %q", idx.Templates[0].Name, "hello-world")
	}
}

// TestFetchIndexWritesCache verifies that FetchIndex writes the fetched
// catalog to the cache directory for subsequent reads.
func TestFetchIndexWritesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(catalogYAML))
	}))
	defer srv.Close()

	cacheDir := filepath.Join(t.TempDir(), "cache")
	c := &Client{
		BaseURL:  srv.URL,
		CacheDir: cacheDir,
		CacheTTL: time.Hour,
	}

	_, err := c.FetchIndex(false)
	if err != nil {
		t.Fatalf("FetchIndex: %v", err)
	}

	// Verify cache file was written
	cachePath := filepath.Join(cacheDir, "scaffolds-index.yaml")
	if _, err := os.Stat(cachePath); err != nil {
		t.Errorf("expected cache file at %s: %v", cachePath, err)
	}
}

// TestFetchIndexUsesCache verifies that FetchIndex reads from cache when
// the cached file is fresh, without making an HTTP request.
func TestFetchIndexUsesCache(t *testing.T) {
	// Create a cache file directly
	cacheDir := filepath.Join(t.TempDir(), "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "scaffolds-index.yaml")
	if err := os.WriteFile(cachePath, []byte(catalogYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use a server that would fail if called — proves cache is used
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{
		BaseURL:  srv.URL,
		CacheDir: cacheDir,
		CacheTTL: time.Hour,
	}

	idx, err := c.FetchIndex(false)
	if err != nil {
		t.Fatalf("FetchIndex should use cache: %v", err)
	}
	if idx.Version != "1" {
		t.Errorf("Version: got %q, want %q", idx.Version, "1")
	}
}

// TestFetchIndexNoCacheBypassesCache verifies that FetchIndex with noCache=true
// ignores the cache and fetches from remote even when cache exists.
func TestFetchIndexNoCacheBypassesCache(t *testing.T) {
	// Create a stale cache file with different content
	cacheDir := filepath.Join(t.TempDir(), "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "scaffolds-index.yaml")
	if err := os.WriteFile(cachePath, []byte(`version: "old"`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Server returns fresh data
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(catalogYAML))
	}))
	defer srv.Close()

	c := &Client{
		BaseURL:  srv.URL,
		CacheDir: cacheDir,
		CacheTTL: time.Hour,
	}

	idx, err := c.FetchIndex(true)
	if err != nil {
		t.Fatalf("FetchIndex: %v", err)
	}
	// Should get fresh data, not "old"
	if idx.Version != "1" {
		t.Errorf("Version: got %q, want %q (should bypass cache)", idx.Version, "1")
	}
}

// TestFetchIndexInvalidYAMLReturnsError verifies that FetchIndex returns
// a parse error when the remote returns invalid YAML.
func TestFetchIndexInvalidYAMLReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(":::not yaml"))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	_, err := c.FetchIndex(true)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// --- FetchFile ---

// TestFetchFileSuccess verifies that FetchFile concatenates the base URL
// with the file path and returns the file content.
func TestFetchFileSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/templates/hello/workflow.yaml" {
			_, _ = w.Write([]byte("name: hello"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	data, err := c.FetchFile("templates/hello/workflow.yaml")
	if err != nil {
		t.Fatalf("FetchFile: %v", err)
	}
	if string(data) != "name: hello" {
		t.Errorf("body: got %q, want %q", string(data), "name: hello")
	}
}

// TestFetchFileNotFound verifies that FetchFile returns an error when
// the requested file does not exist on the remote.
func TestFetchFileNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	_, err := c.FetchFile("nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
