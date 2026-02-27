package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	envEndpoint = "TNTC_MCP_ENDPOINT"
	envToken    = "TNTC_MCP_TOKEN"

	defaultTimeout = 30 * time.Second
)

// mcpYAMLConfig mirrors the yaml structure under `mcp:` in config.yaml.
type mcpYAMLConfig struct {
	Endpoint  string `yaml:"endpoint,omitempty"`
	TokenPath string `yaml:"token_path,omitempty"`
}

type tentacularYAMLConfig struct {
	MCP mcpYAMLConfig `yaml:"mcp,omitempty"`
}

// LoadConfigFromCluster resolves MCP connection settings using the cascade:
// 1. Environment variables: TNTC_MCP_ENDPOINT, TNTC_MCP_TOKEN
// 2. Project config: .tentacular/config.yaml (mcp.endpoint, mcp.token_path)
// 3. User config: ~/.tentacular/config.yaml (mcp.endpoint, mcp.token_path)
//
// Returns nil, nil if no MCP configuration is found anywhere.
func LoadConfigFromCluster(ctx context.Context) (*Config, error) {
	cfg := &Config{Timeout: defaultTimeout}

	// 1. Load from config files (user then project for proper override order)
	home, _ := os.UserHomeDir()
	if home != "" {
		loadMCPFromFile(filepath.Join(home, ".tentacular", "config.yaml"), cfg)
	}
	loadMCPFromFile(filepath.Join(".tentacular", "config.yaml"), cfg)

	// 2. Environment variables override config file values
	if v := os.Getenv(envEndpoint); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv(envToken); v != "" {
		cfg.Token = v
	}

	// If token not set directly, try token_path
	if cfg.Token == "" && cfg.TokenPath != "" {
		token, err := readTokenFile(cfg.TokenPath)
		if err != nil {
			return nil, fmt.Errorf("reading MCP token from %s: %w", cfg.TokenPath, err)
		}
		cfg.Token = token
	}

	if cfg.Endpoint == "" {
		return nil, nil
	}

	return cfg, nil
}

// loadMCPFromFile reads mcp config from a YAML file, updating cfg in place.
// Missing files are silently ignored.
func loadMCPFromFile(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var raw tentacularYAMLConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	if raw.MCP.Endpoint != "" {
		cfg.Endpoint = raw.MCP.Endpoint
	}
	if raw.MCP.TokenPath != "" {
		cfg.TokenPath = raw.MCP.TokenPath
	}
}

// readTokenFile reads a bearer token from a file, trimming whitespace.
func readTokenFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveConfig writes MCP endpoint and token path to the user-level config file.
// Creates the file if it does not exist; merges with existing content.
func SaveConfig(endpoint, tokenPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}

	dir := filepath.Join(home, ".tentacular")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")

	// Load existing config to merge
	var raw map[string]interface{}
	if data, err := os.ReadFile(cfgPath); err == nil {
		yaml.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = make(map[string]interface{})
	}

	raw["mcp"] = map[string]interface{}{
		"endpoint":   endpoint,
		"token_path": tokenPath,
	}

	data, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
