package catalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL  = "https://raw.githubusercontent.com/randybias/tentacular-scaffolds/main"
	DefaultCacheTTL = 1 * time.Hour
)

// CatalogConfig holds catalog client settings from the CLI config.
type CatalogConfig struct {
	URL      string `yaml:"url,omitempty"`
	CacheTTL string `yaml:"cacheTTL,omitempty"`
}

// Client fetches and caches the catalog index and template files.
type Client struct {
	BaseURL  string
	CacheDir string
	CacheTTL time.Duration
}

// NewClient creates a catalog client from config.
func NewClient(cfg CatalogConfig) *Client {
	baseURL := DefaultBaseURL
	if cfg.URL != "" {
		baseURL = cfg.URL
	}

	ttl := DefaultCacheTTL
	if cfg.CacheTTL != "" {
		if parsed, err := time.ParseDuration(cfg.CacheTTL); err == nil {
			ttl = parsed
		}
	}

	cacheDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		cacheDir = filepath.Join(home, ".tentacular", "cache")
	}

	return &Client{
		BaseURL:  baseURL,
		CacheDir: cacheDir,
		CacheTTL: ttl,
	}
}

// FetchIndex returns the catalog index, using cache if valid and noCache is false.
func (c *Client) FetchIndex(noCache bool) (*CatalogIndex, error) {
	cachePath := ""
	if c.CacheDir != "" {
		cachePath = filepath.Join(c.CacheDir, "scaffolds-index.yaml")
	}

	// Try cache first
	if !noCache && cachePath != "" {
		if info, err := os.Stat(cachePath); err == nil {
			if time.Since(info.ModTime()) < c.CacheTTL {
				data, err := os.ReadFile(cachePath) //nolint:gosec // reading catalog cache file by computed path
				if err == nil {
					var idx CatalogIndex
					if err := yaml.Unmarshal(data, &idx); err == nil {
						return &idx, nil
					}
				}
			}
		}
	}

	// Fetch from remote
	url := c.BaseURL + "/scaffolds-index.yaml"
	data, err := c.httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("fetching catalog index: %w", err)
	}

	var idx CatalogIndex
	if err := yaml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing catalog index: %w", err)
	}

	// Write to cache
	if cachePath != "" {
		if err := os.MkdirAll(c.CacheDir, 0o755); err == nil { //nolint:gosec // 0o755 for user cache directory
			_ = os.WriteFile(cachePath, data, 0o644) //nolint:gosec // 0o644 for user cache file
		}
	}

	return &idx, nil
}

// FetchFile fetches a single file from the catalog by relative path.
func (c *Client) FetchFile(path string) ([]byte, error) {
	url := c.BaseURL + "/" + path
	data, err := c.httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", path, err)
	}
	return data, nil
}

func (*Client) httpGet(url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}
