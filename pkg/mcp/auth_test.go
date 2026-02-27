package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// --- LoadConfigFromCluster tests ---

func TestLoadConfigFromCluster_EnvVarsOnly(t *testing.T) {
	t.Setenv(envEndpoint, "http://mcp.example.com:8080")
	t.Setenv(envToken, "my-token")

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
	if cfg.Token != "my-token" {
		t.Errorf("token: got %q, want my-token", cfg.Token)
	}
}

func TestLoadConfigFromCluster_NoConfig_ReturnsNil(t *testing.T) {
	// Unset all env vars and use a home dir with no config
	os.Unsetenv(envEndpoint)
	os.Unsetenv(envToken)

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	cfg, err := LoadConfigFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config when no MCP configured, got %+v", cfg)
	}
}

func TestLoadConfigFromCluster_UserConfigFile(t *testing.T) {
	os.Unsetenv(envEndpoint)
	os.Unsetenv(envToken)

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(configDir, 0o755)
	configContent := "mcp:\n  endpoint: http://from-user-config:8080\n  token_path: /some/token\n"
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644)

	// Write a token file to satisfy token_path
	tokenFile := filepath.Join(tmpHome, "mcp-token")
	os.WriteFile(tokenFile, []byte("file-token\n"), 0o600)

	// Override token_path in config to point to our test file
	configContent2 := "mcp:\n  endpoint: http://from-user-config:8080\n  token_path: " + tokenFile + "\n"
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent2), 0o644)

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
	if cfg.Token != "file-token" {
		t.Errorf("token: got %q, want file-token", cfg.Token)
	}
}

func TestLoadConfigFromCluster_EnvOverridesFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("mcp:\n  endpoint: http://from-file:8080\n"), 0o644)

	t.Setenv(envEndpoint, "http://from-env:9090")
	t.Setenv(envToken, "env-token")

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
	if cfg.Token != "env-token" {
		t.Errorf("expected env token to win, got %q", cfg.Token)
	}
}

func TestLoadConfigFromCluster_TokenFileNotFound(t *testing.T) {
	os.Unsetenv(envEndpoint)
	os.Unsetenv(envToken)

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("mcp:\n  endpoint: http://mcp:8080\n  token_path: /nonexistent/token\n"), 0o644)

	_, err := LoadConfigFromCluster(context.Background())
	if err == nil {
		t.Fatal("expected error when token file does not exist")
	}
}

func TestLoadConfigFromCluster_DirectTokenEnvNoFile(t *testing.T) {
	// Token set directly via env (no token_path), endpoint via env
	t.Setenv(envEndpoint, "http://direct-token:8080")
	t.Setenv(envToken, "direct-bearer")

	cfg, err := LoadConfigFromCluster(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Token != "direct-bearer" {
		t.Errorf("token: got %q, want direct-bearer", cfg.Token)
	}
}

// --- loadMCPFromFile tests ---

func TestLoadMCPFromFile_MissingFile(t *testing.T) {
	cfg := &Config{}
	loadMCPFromFile("/does/not/exist.yaml", cfg)
	// Should not modify cfg
	if cfg.Endpoint != "" || cfg.TokenPath != "" {
		t.Errorf("expected unchanged config for missing file, got %+v", cfg)
	}
}

func TestLoadMCPFromFile_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yaml")
	os.WriteFile(p, []byte("mcp:\n  endpoint: http://test:8080\n  token_path: /var/token\n"), 0o644)

	cfg := &Config{}
	loadMCPFromFile(p, cfg)

	if cfg.Endpoint != "http://test:8080" {
		t.Errorf("endpoint: got %q, want http://test:8080", cfg.Endpoint)
	}
	if cfg.TokenPath != "/var/token" {
		t.Errorf("token_path: got %q, want /var/token", cfg.TokenPath)
	}
}

func TestLoadMCPFromFile_MalformedYAML(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yaml")
	os.WriteFile(p, []byte(":::not valid yaml[[["), 0o644)

	cfg := &Config{Endpoint: "existing"}
	loadMCPFromFile(p, cfg) // should silently ignore
	if cfg.Endpoint != "existing" {
		t.Errorf("expected existing endpoint preserved after malformed yaml, got %q", cfg.Endpoint)
	}
}

func TestLoadMCPFromFile_NoMCPSection(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yaml")
	os.WriteFile(p, []byte("registry: my-registry\nnamespace: dev\n"), 0o644)

	cfg := &Config{}
	loadMCPFromFile(p, cfg)
	if cfg.Endpoint != "" {
		t.Errorf("expected empty endpoint, got %q", cfg.Endpoint)
	}
}

// --- readTokenFile tests ---

func TestReadTokenFile_Success(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "token")
	os.WriteFile(p, []byte("  my-bearer-token\n  "), 0o600)

	tok, err := readTokenFile(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "my-bearer-token" {
		t.Errorf("expected trimmed token, got %q", tok)
	}
}

func TestReadTokenFile_NotFound(t *testing.T) {
	_, err := readTokenFile("/nonexistent/path/token")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadTokenFile_Empty(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "empty-token")
	os.WriteFile(p, []byte("\n\n"), 0o600)

	tok, err := readTokenFile(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "" {
		t.Errorf("expected empty string for whitespace-only file, got %q", tok)
	}
}

// --- SaveConfig tests ---

func TestSaveConfig_WritesNewFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	err := SaveConfig("http://mcp:8080", "/var/run/token")
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
	if !containsString(content, "/var/run/token") {
		t.Errorf("expected token_path in saved config, got:\n%s", content)
	}
}

func TestSaveConfig_MergesWithExistingConfig(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(configDir, 0o755)
	// Pre-existing config with a different key
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("registry: my-registry\n"), 0o644)

	err := SaveConfig("http://mcp:8080", "/var/token")
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
	defer os.Setenv("HOME", origHome)

	err := SaveConfig("http://mcp:8080", "")
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
