package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigDefaultEnv verifies default_cluster is parsed and merged.
func TestLoadConfigDefaultEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_cluster: staging
clusters:
  staging:
    namespace: staging-ns
`), 0o644)

	cfg := LoadConfig()
	if cfg.DefaultCluster != "staging" {
		t.Errorf("expected default_cluster=staging, got %q", cfg.DefaultCluster)
	}
}

// TestMergeConfigDefaultEnv verifies project config overrides user's default_cluster.
func TestMergeConfigDefaultEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// User config: default_cluster=dev
	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_cluster: dev
clusters:
  dev:
    namespace: dev-ns
`), 0o644)

	// Project config: overrides default_cluster=staging
	projDir := filepath.Join(tmpDir, ".tentacular")
	_ = os.MkdirAll(projDir, 0o755)
	_ = os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`default_cluster: staging
clusters:
  staging:
    namespace: staging-ns
`), 0o644)

	cfg := LoadConfig()
	if cfg.DefaultCluster != "staging" {
		t.Errorf("expected project default_cluster=staging to override user, got %q", cfg.DefaultCluster)
	}
}

// TestLoadConfigPerEnvMCPEndpoint verifies mcp_endpoint is parsed from
// per-environment config.
func TestLoadConfigPerEnvMCPEndpoint(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`clusters:
  prod:
    namespace: prod-ns
    mcp_endpoint: http://prod-mcp.tentacular-system.svc.cluster.local:8080
`), 0o644)

	cfg := LoadConfig()
	prod, ok := cfg.Clusters["prod"]
	if !ok {
		t.Fatal("expected prod environment")
	}
	if prod.MCPEndpoint != "http://prod-mcp.tentacular-system.svc.cluster.local:8080" {
		t.Errorf("expected prod mcp_endpoint, got %q", prod.MCPEndpoint)
	}
}

// TestEnvironmentMCPEndpointOmittedWhenEmpty verifies mcp_endpoint is
// omitted when not set (omitempty).
func TestEnvironmentMCPEndpointOmittedWhenEmpty(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`clusters:
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
}

// TestLoadConfigMultipleEnvsWithMCP verifies multiple environments can each
// have their own mcp_endpoint.
func TestLoadConfigMultipleEnvsWithMCP(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_cluster: dev
clusters:
  dev:
    namespace: dev-ns
    mcp_endpoint: http://dev-mcp:8080
  staging:
    namespace: staging-ns
    mcp_endpoint: http://staging-mcp:8080
  prod:
    namespace: prod-ns
    mcp_endpoint: http://prod-mcp:8080
`), 0o644)

	cfg := LoadConfig()
	if cfg.DefaultCluster != "dev" {
		t.Errorf("expected default_cluster=dev, got %q", cfg.DefaultCluster)
	}
	if len(cfg.Clusters) != 3 {
		t.Errorf("expected 3 environments, got %d", len(cfg.Clusters))
	}

	dev := cfg.Clusters["dev"]
	if dev.MCPEndpoint != "http://dev-mcp:8080" {
		t.Errorf("expected dev mcp_endpoint http://dev-mcp:8080, got %q", dev.MCPEndpoint)
	}

	prod := cfg.Clusters["prod"]
	if prod.MCPEndpoint != "http://prod-mcp:8080" {
		t.Errorf("expected prod mcp_endpoint http://prod-mcp:8080, got %q", prod.MCPEndpoint)
	}
}

// TestResolveEnvironmentUsesDefaultEnv verifies that ResolveEnvironment
// picks up default_cluster when no explicit name is given.
func TestResolveEnvironmentUsesDefaultEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Ensure TENTACULAR_CLUSTER is not set
	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Unsetenv("TENTACULAR_CLUSTER")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_cluster: staging
clusters:
  staging:
    namespace: staging-ns
    mcp_endpoint: http://staging-mcp:8080
`), 0o644)

	env, err := ResolveEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "staging-ns" {
		t.Errorf("expected staging-ns from default_cluster, got %q", env.Namespace)
	}
	if env.MCPEndpoint != "http://staging-mcp:8080" {
		t.Errorf("expected staging mcp_endpoint from default_cluster, got %q", env.MCPEndpoint)
	}
}

// TestResolveEnvironmentDefaultEnvOverriddenByTENTACULAR_CLUSTER verifies that
// TENTACULAR_CLUSTER takes priority over default_cluster.
func TestResolveEnvironmentDefaultEnvOverriddenByTENTACULAR_CLUSTER(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Setenv("TENTACULAR_CLUSTER", "prod")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`default_cluster: dev
clusters:
  dev:
    namespace: dev-ns
  prod:
    namespace: prod-ns
`), 0o644)

	// TENTACULAR_CLUSTER=prod should override default_cluster=dev
	env, err := ResolveEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "prod-ns" {
		t.Errorf("expected prod-ns (TENTACULAR_CLUSTER wins over default_cluster), got %q", env.Namespace)
	}
}
