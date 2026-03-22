package scaffold

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
	defaultCacheTTLDuration = 1 * time.Hour
	scaffoldsIndexFile      = "scaffolds-index.yaml"
)

// ClientConfig holds scaffold client settings.
type ClientConfig struct {
	URL      string `yaml:"url,omitempty"`
	CacheTTL string `yaml:"cacheTTL,omitempty"`
}

// Client fetches and caches the scaffolds index and scaffold files.
type Client struct {
	BaseURL  string
	CacheDir string
	CacheTTL time.Duration
}

// NewClient creates a scaffold client from config.
func NewClient(cfg ClientConfig) *Client {
	baseURL := DefaultBaseURL
	if cfg.URL != "" {
		baseURL = cfg.URL
	}

	ttl := defaultCacheTTLDuration
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

// CachedIndexPath returns the path to the cached scaffolds index file.
func (c *Client) CachedIndexPath() string {
	if c.CacheDir == "" {
		return ""
	}
	return filepath.Join(c.CacheDir, scaffoldsIndexFile)
}

// FetchIndex returns the scaffolds index, using cache if valid and noCache is false.
func (c *Client) FetchIndex(noCache bool) (*ScaffoldIndex, error) {
	cachePath := c.CachedIndexPath()

	// Try cache first
	if !noCache && cachePath != "" {
		if info, err := os.Stat(cachePath); err == nil {
			if time.Since(info.ModTime()) < c.CacheTTL {
				data, err := os.ReadFile(cachePath) //nolint:gosec // reading scaffolds cache file by computed path
				if err == nil {
					var idx ScaffoldIndex
					if err := yaml.Unmarshal(data, &idx); err == nil {
						return &idx, nil
					}
				}
			}
		}
	}

	// Fetch from remote
	url := c.BaseURL + "/" + scaffoldsIndexFile
	data, err := c.httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("fetching scaffolds index: %w", err)
	}

	var idx ScaffoldIndex
	if err := yaml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing scaffolds index: %w", err)
	}

	// Write to cache
	if cachePath != "" {
		if err := os.MkdirAll(c.CacheDir, 0o700); err == nil {
			_ = os.WriteFile(cachePath, data, 0o600) //nolint:gosec // cachePath is under validated CacheDir
		}
	}

	return &idx, nil
}

// FetchFile fetches a single file from the remote scaffolds repo by relative path.
func (c *Client) FetchFile(path string) ([]byte, error) {
	url := c.BaseURL + "/" + path
	data, err := c.httpGet(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", path, err)
	}
	return data, nil
}

// Sync fetches the scaffolds index and writes files to the local quickstarts cache directory.
func (c *Client) Sync() (*ScaffoldIndex, error) {
	idx, err := c.FetchIndex(true) // force re-fetch
	if err != nil {
		return nil, err
	}
	return idx, nil
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
