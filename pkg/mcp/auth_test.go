package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// --- LoadConfigFromCluster tests ---

func TestLoadConfigFromCluster_EnvVarsOnly(t *testing.T) {
	t.Setenv("TNTC_MCP_ENDPOINT", "http://mcp.example.com:8080")

	cfg, err := LoadConfigFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Endpoint != "http://mcp.example.com:8080" {
		t.Errorf("endpoint: got %q, want http://mcp.example.com:8080", cfg.Endpoint)
	}
}

func TestLoadConfigFromCluster_NoConfig_ReturnsNil(t *testing.T) {
	// Unset endpoint env var and use a home dir with no config
	_ = os.Unsetenv("TNTC_MCP_ENDPOINT")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	cfg, err := LoadConfigFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config when no MCP configured, got %+v", cfg)
	}
}

func TestLoadConfigFromCluster_UserConfigFile(t *testing.T) {
	_ = os.Unsetenv("TNTC_MCP_ENDPOINT")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(configDir, 0o755)
	configContent := "mcp:\n  endpoint: http://from-user-config:8080\n"
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644)

	cfg, err := LoadConfigFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config from user config file")
	}
	if cfg.Endpoint != "http://from-user-config:8080" {
		t.Errorf("endpoint: got %q, want http://from-user-config:8080", cfg.Endpoint)
	}
}

func TestLoadConfigFromCluster_EnvOverridesFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(configDir, 0o755)
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("mcp:\n  endpoint: http://from-file:8080\n"), 0o644)

	t.Setenv("TNTC_MCP_ENDPOINT", "http://from-env:9090")

	cfg, err := LoadConfigFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Endpoint != "http://from-env:9090" {
		t.Errorf("expected env endpoint to win, got %q", cfg.Endpoint)
	}
}

// --- loadMCPFromFile tests ---

func TestLoadMCPFromFile_MissingFile(t *testing.T) {
	cfg := &Config{}
	loadMCPFromFile("/does/not/exist.yaml", cfg)
	// Should not modify cfg
	if cfg.Endpoint != "" {
		t.Errorf("expected unchanged config for missing file, got %+v", cfg)
	}
}

func TestLoadMCPFromFile_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yaml")
	_ = os.WriteFile(p, []byte("mcp:\n  endpoint: http://test:8080\n"), 0o644)

	cfg := &Config{}
	loadMCPFromFile(p, cfg)

	if cfg.Endpoint != "http://test:8080" {
		t.Errorf("endpoint: got %q, want http://test:8080", cfg.Endpoint)
	}
}

func TestLoadMCPFromFile_MalformedYAML(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yaml")
	_ = os.WriteFile(p, []byte(":::not valid yaml[[["), 0o644)

	cfg := &Config{Endpoint: "existing"}
	loadMCPFromFile(p, cfg) // should silently ignore
	if cfg.Endpoint != "existing" {
		t.Errorf("expected existing endpoint preserved after malformed yaml, got %q", cfg.Endpoint)
	}
}

func TestLoadMCPFromFile_NoMCPSection(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yaml")
	_ = os.WriteFile(p, []byte("registry: my-registry\nnamespace: dev\n"), 0o644)

	cfg := &Config{}
	loadMCPFromFile(p, cfg)
	if cfg.Endpoint != "" {
		t.Errorf("expected empty endpoint, got %q", cfg.Endpoint)
	}
}

// --- SaveConfig tests ---

func TestSaveConfig_WritesNewFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	err := SaveConfig("http://mcp:8080")
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfgPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}

	content := string(data)
	if !containsString(content, "http://mcp:8080") {
		t.Errorf("expected endpoint in saved config, got:\n%s", content)
	}
}

func TestSaveConfig_MergesWithExistingConfig(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(configDir, 0o755)
	// Pre-existing config with a different key
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("registry: my-registry\n"), 0o644)

	err := SaveConfig("http://mcp:8080")
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	content := string(data)
	// Both registry and mcp keys should be present
	if !containsString(content, "registry") {
		t.Errorf("expected registry key preserved, got:\n%s", content)
	}
	if !containsString(content, "http://mcp:8080") {
		t.Errorf("expected new endpoint, got:\n%s", content)
	}
}

func TestSaveConfig_FilePermissions(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	err := SaveConfig("http://mcp:8080")
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	cfgPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// File should be readable only by owner (0o600)
	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("expected mode 0o600, got %v", mode)
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStringHelper(s, sub))
}

func containsStringHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
