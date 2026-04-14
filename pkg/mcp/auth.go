package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	envEndpoint = "TNTC_MCP_ENDPOINT"

	defaultTimeout = 30 * time.Second
)

// mcpYAMLConfig mirrors the yaml structure under `mcp:` in config.yaml.
type mcpYAMLConfig struct {
	Endpoint string `yaml:"endpoint,omitempty"`
}

type tentacularYAMLConfig struct {
	MCP mcpYAMLConfig `yaml:"mcp,omitempty"`
}

// LoadConfigFromCluster resolves MCP connection settings using the cascade:
// 1. Environment variable: TNTC_MCP_ENDPOINT
// 2. Project config: .tentacular/config.yaml (mcp.endpoint)
// 3. User config: ~/.tentacular/config.yaml (mcp.endpoint)
//
// Note: Authentication is handled via OIDC tokens resolved by the CLI layer,
// not by this function. This only resolves the endpoint.
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

	// 2. Environment variable overrides config file value
	if v := os.Getenv(envEndpoint); v != "" {
		cfg.Endpoint = v
	}

	if cfg.Endpoint == "" {
		return nil, nil
	}

	return cfg, nil
}

// loadMCPFromFile reads mcp config from a YAML file, updating cfg in place.
// Missing files are silently ignored.
func loadMCPFromFile(path string, cfg *Config) {
	data, err := os.ReadFile(path) //nolint:gosec // reading config file by computed path
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
}

// SaveConfig writes MCP endpoint to the user-level config file.
// Creates the file if it does not exist; merges with existing content.
func SaveConfig(endpoint string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}

	dir := filepath.Join(home, ".tentacular")
	if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil { //nolint:gosec // 0o755 for config directory
		return fmt.Errorf("creating config directory: %w", mkdirErr)
	}

	cfgPath := filepath.Join(dir, "config.yaml")

	// Load existing config to merge
	var raw map[string]any
	if data, readErr := os.ReadFile(cfgPath); readErr == nil { //nolint:gosec // reading config file
		_ = yaml.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = make(map[string]any)
	}

	raw["mcp"] = map[string]any{
		"endpoint": endpoint,
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
