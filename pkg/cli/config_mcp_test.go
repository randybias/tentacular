package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigDefaultEnv verifies default_env is parsed and merged.
func TestLoadConfigDefaultEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_env: staging
environments:
  staging:
    namespace: staging-ns
`), 0o644)

	cfg := LoadConfig()
	if cfg.DefaultEnv != "staging" {
		t.Errorf("expected default_env=staging, got %q", cfg.DefaultEnv)
	}
}

// TestMergeConfigDefaultEnv verifies project config overrides user's default_env.
func TestMergeConfigDefaultEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// User config: default_env=dev
	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_env: dev
environments:
  dev:
    namespace: dev-ns
`), 0o644)

	// Project config: overrides default_env=staging
	projDir := filepath.Join(tmpDir, ".tentacular")
	os.MkdirAll(projDir, 0o755)
	os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`default_env: staging
environments:
  staging:
    namespace: staging-ns
`), 0o644)

	cfg := LoadConfig()
	if cfg.DefaultEnv != "staging" {
		t.Errorf("expected project default_env=staging to override user, got %q", cfg.DefaultEnv)
	}
}

// TestLoadConfigPerEnvMCPEndpoint verifies mcp_endpoint is parsed from
// per-environment config.
func TestLoadConfigPerEnvMCPEndpoint(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`environments:
  prod:
    namespace: prod-ns
    mcp_endpoint: http://prod-mcp.tentacular-system.svc.cluster.local:8080
    mcp_token_path: ~/.tentacular/prod-token
`), 0o644)

	cfg := LoadConfig()
	prod, ok := cfg.Environments["prod"]
	if !ok {
		t.Fatal("expected prod environment")
	}
	if prod.MCPEndpoint != "http://prod-mcp.tentacular-system.svc.cluster.local:8080" {
		t.Errorf("expected prod mcp_endpoint, got %q", prod.MCPEndpoint)
	}
	if prod.MCPTokenPath != "~/.tentacular/prod-token" {
		t.Errorf("expected prod mcp_token_path, got %q", prod.MCPTokenPath)
	}
}

// TestEnvironmentMCPEndpointOmittedWhenEmpty verifies mcp_endpoint is
// omitted when not set (omitempty).
func TestEnvironmentMCPEndpointOmittedWhenEmpty(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`environments:
  dev:
    namespace: dev-ns
`), 0o644)

	env, err := LoadEnvironment("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.MCPEndpoint != "" {
		t.Errorf("expected empty mcp_endpoint when not configured, got %q", env.MCPEndpoint)
	}
	if env.MCPTokenPath != "" {
		t.Errorf("expected empty mcp_token_path when not configured, got %q", env.MCPTokenPath)
	}
}

// TestLoadConfigMultipleEnvsWithMCP verifies multiple environments can each
// have their own mcp_endpoint.
func TestLoadConfigMultipleEnvsWithMCP(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_env: dev
environments:
  dev:
    namespace: dev-ns
    mcp_endpoint: http://dev-mcp:8080
  staging:
    namespace: staging-ns
    mcp_endpoint: http://staging-mcp:8080
  prod:
    namespace: prod-ns
    mcp_endpoint: http://prod-mcp:8080
    mcp_token_path: /etc/tentacular/prod-token
`), 0o644)

	cfg := LoadConfig()
	if cfg.DefaultEnv != "dev" {
		t.Errorf("expected default_env=dev, got %q", cfg.DefaultEnv)
	}
	if len(cfg.Environments) != 3 {
		t.Errorf("expected 3 environments, got %d", len(cfg.Environments))
	}

	dev := cfg.Environments["dev"]
	if dev.MCPEndpoint != "http://dev-mcp:8080" {
		t.Errorf("expected dev mcp_endpoint http://dev-mcp:8080, got %q", dev.MCPEndpoint)
	}

	prod := cfg.Environments["prod"]
	if prod.MCPTokenPath != "/etc/tentacular/prod-token" {
		t.Errorf("expected prod mcp_token_path /etc/tentacular/prod-token, got %q", prod.MCPTokenPath)
	}
}

// TestResolveEnvironmentUsesDefaultEnv verifies that ResolveEnvironment
// picks up default_env when no explicit name is given.
func TestResolveEnvironmentUsesDefaultEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Ensure TENTACULAR_ENV is not set
	origEnv := os.Getenv("TENTACULAR_ENV")
	os.Unsetenv("TENTACULAR_ENV")
	defer os.Setenv("TENTACULAR_ENV", origEnv)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_env: staging
environments:
  staging:
    namespace: staging-ns
    mcp_endpoint: http://staging-mcp:8080
`), 0o644)

	env, err := ResolveEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "staging-ns" {
		t.Errorf("expected staging-ns from default_env, got %q", env.Namespace)
	}
	if env.MCPEndpoint != "http://staging-mcp:8080" {
		t.Errorf("expected staging mcp_endpoint from default_env, got %q", env.MCPEndpoint)
	}
}

// TestResolveEnvironmentDefaultEnvOverriddenByTENTACULAR_ENV verifies that
// TENTACULAR_ENV takes priority over default_env.
func TestResolveEnvironmentDefaultEnvOverriddenByTENTACULAR_ENV(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	origEnv := os.Getenv("TENTACULAR_ENV")
	os.Setenv("TENTACULAR_ENV", "prod")
	defer os.Setenv("TENTACULAR_ENV", origEnv)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_env: dev
environments:
  dev:
    namespace: dev-ns
  prod:
    namespace: prod-ns
`), 0o644)

	// TENTACULAR_ENV=prod should override default_env=dev
	env, err := ResolveEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "prod-ns" {
		t.Errorf("expected prod-ns (TENTACULAR_ENV wins over default_env), got %q", env.Namespace)
	}
}
