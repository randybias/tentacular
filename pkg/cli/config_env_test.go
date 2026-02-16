package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- WI-2: Environment Configuration Tests ---
// These tests validate the environment configuration feature.
// Build tag: wi2 -- run with: go test -tags wi2 ./pkg/cli/...

func TestLoadConfigWithEnvironments(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create user-level config with environments
	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	configYAML := `registry: default-registry
namespace: default-ns
environments:
  dev:
    namespace: dev-ns
    runtime_class: ""
    config_overrides:
      pg_host: localhost
      pg_port: 5432
  staging:
    namespace: staging-ns
    context: staging-cluster
    image: staging-registry/tentacular-engine:v1
    runtime_class: gvisor
    config_overrides:
      pg_host: staging-db.internal
      pg_port: 5432
  prod:
    namespace: prod-ns
    context: prod-cluster
    secrets_source: vault://prod/tentacular
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	cfg := LoadConfig()

	// Base config should still work
	if cfg.Registry != "default-registry" {
		t.Errorf("expected default-registry, got %s", cfg.Registry)
	}
	if cfg.Namespace != "default-ns" {
		t.Errorf("expected default-ns, got %s", cfg.Namespace)
	}

	// Environments map should be populated
	if cfg.Environments == nil {
		t.Fatal("expected Environments map to be non-nil")
	}
	if len(cfg.Environments) != 3 {
		t.Errorf("expected 3 environments, got %d", len(cfg.Environments))
	}

	devEnv, ok := cfg.Environments["dev"]
	if !ok {
		t.Fatal("expected 'dev' environment to exist")
	}
	if devEnv.Namespace != "dev-ns" {
		t.Errorf("expected dev namespace dev-ns, got %s", devEnv.Namespace)
	}

	stagingEnv, ok := cfg.Environments["staging"]
	if !ok {
		t.Fatal("expected 'staging' environment to exist")
	}
	if stagingEnv.Context != "staging-cluster" {
		t.Errorf("expected staging context staging-cluster, got %s", stagingEnv.Context)
	}
	if stagingEnv.Image != "staging-registry/tentacular-engine:v1" {
		t.Errorf("expected staging image, got %s", stagingEnv.Image)
	}
	if stagingEnv.RuntimeClass != "gvisor" {
		t.Errorf("expected staging runtime_class gvisor, got %s", stagingEnv.RuntimeClass)
	}
}

func TestLoadEnvironmentFound(t *testing.T) {
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
	configYAML := `registry: base-reg
environments:
  staging:
    namespace: staging-ns
    context: staging-ctx
    config_overrides:
      pg_host: staging-db
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	env, err := LoadEnvironment("staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env == nil {
		t.Fatal("expected non-nil environment")
	}
	if env.Namespace != "staging-ns" {
		t.Errorf("expected staging-ns, got %s", env.Namespace)
	}
	if env.Context != "staging-ctx" {
		t.Errorf("expected staging-ctx, got %s", env.Context)
	}
}

func TestLoadEnvironmentNotFound(t *testing.T) {
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
	configYAML := `registry: base-reg
environments:
  dev:
    namespace: dev-ns
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	_, err := LoadEnvironment("production")
	if err == nil {
		t.Fatal("expected error for unknown environment")
	}
	if !strings.Contains(err.Error(), "production") {
		t.Errorf("expected error message to contain 'production', got: %v", err)
	}
}

func TestLoadEnvironmentNoEnvironmentsDefined(t *testing.T) {
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
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte("registry: base-reg\n"), 0o644)

	_, err := LoadEnvironment("dev")
	if err == nil {
		t.Fatal("expected error when no environments are defined")
	}
}

func TestProjectEnvOverridesUserEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// User-level config has dev environment
	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	userYAML := `environments:
  dev:
    namespace: user-dev-ns
    context: user-dev-ctx
    runtime_class: user-rc
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(userYAML), 0o644)

	// Project-level config overrides dev namespace only
	projDir := filepath.Join(tmpDir, ".tentacular")
	os.MkdirAll(projDir, 0o755)
	projYAML := `environments:
  dev:
    namespace: project-dev-ns
`
	os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(projYAML), 0o644)

	env, err := LoadEnvironment("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Project should override namespace
	if env.Namespace != "project-dev-ns" {
		t.Errorf("expected project-dev-ns, got %s", env.Namespace)
	}
}

func TestApplyConfigOverrides(t *testing.T) {
	wfConfig := map[string]interface{}{
		"pg_host":     "default-host",
		"pg_port":     5432,
		"pg_database": "appdb",
	}
	overrides := map[string]interface{}{
		"pg_host": "staging-host",
		"pg_port": 15432,
	}

	ApplyConfigOverrides(wfConfig, overrides)

	if wfConfig["pg_host"] != "staging-host" {
		t.Errorf("expected staging-host, got %v", wfConfig["pg_host"])
	}
	if wfConfig["pg_port"] != 15432 {
		t.Errorf("expected 15432, got %v", wfConfig["pg_port"])
	}
	// Non-overridden key should remain
	if wfConfig["pg_database"] != "appdb" {
		t.Errorf("expected appdb preserved, got %v", wfConfig["pg_database"])
	}
}

func TestApplyConfigOverridesEmpty(t *testing.T) {
	wfConfig := map[string]interface{}{
		"pg_host": "original-host",
	}

	// Nil overrides should be safe
	ApplyConfigOverrides(wfConfig, nil)
	if wfConfig["pg_host"] != "original-host" {
		t.Errorf("expected original-host preserved, got %v", wfConfig["pg_host"])
	}

	// Empty overrides should be safe
	ApplyConfigOverrides(wfConfig, map[string]interface{}{})
	if wfConfig["pg_host"] != "original-host" {
		t.Errorf("expected original-host preserved, got %v", wfConfig["pg_host"])
	}
}

func TestMergeConfigWithEnvironments(t *testing.T) {
	base := &TentacularConfig{
		Registry:  "base-reg",
		Namespace: "base-ns",
		Environments: map[string]EnvironmentConfig{
			"dev": {Namespace: "base-dev-ns", RuntimeClass: "base-rc"},
		},
	}
	override := &TentacularConfig{
		Environments: map[string]EnvironmentConfig{
			"dev":     {Namespace: "override-dev-ns"},
			"staging": {Namespace: "staging-ns"},
		},
	}
	mergeConfig(base, override)

	if len(base.Environments) != 2 {
		t.Errorf("expected 2 environments after merge, got %d", len(base.Environments))
	}
	// dev environment should be overridden
	if base.Environments["dev"].Namespace != "override-dev-ns" {
		t.Errorf("expected override-dev-ns, got %s", base.Environments["dev"].Namespace)
	}
	// staging should be added
	if base.Environments["staging"].Namespace != "staging-ns" {
		t.Errorf("expected staging-ns, got %s", base.Environments["staging"].Namespace)
	}
}

func TestEnvWithEmptyFieldsDoesNotOverrideTopLevel(t *testing.T) {
	// An environment with empty fields should not affect the top-level config.
	// The EnvironmentConfig is a separate struct, so top-level should remain intact.
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
	configYAML := `registry: top-registry
namespace: top-ns
runtime_class: top-rc
environments:
  minimal:
    namespace: minimal-ns
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	cfg := LoadConfig()

	// Top-level should be unaffected by environment entries
	if cfg.Registry != "top-registry" {
		t.Errorf("expected top-registry, got %s", cfg.Registry)
	}
	if cfg.Namespace != "top-ns" {
		t.Errorf("expected top-ns, got %s", cfg.Namespace)
	}
	if cfg.RuntimeClass != "top-rc" {
		t.Errorf("expected top-rc, got %s", cfg.RuntimeClass)
	}

	// The environment should only have what was defined
	env, err := cfg.LoadEnvironment("minimal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "minimal-ns" {
		t.Errorf("expected minimal-ns, got %s", env.Namespace)
	}
	// Empty fields remain empty (env does NOT inherit top-level)
	if env.RuntimeClass != "" {
		t.Errorf("expected empty runtime_class in env, got %s", env.RuntimeClass)
	}
	if env.Context != "" {
		t.Errorf("expected empty context in env, got %s", env.Context)
	}
}

func TestResolveEnvironmentEquivalentToLoadEnvironment(t *testing.T) {
	// ResolveEnvironment and LoadEnvironment should produce the same result
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
	configYAML := `environments:
  dev:
    namespace: dev-ns
    context: dev-ctx
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	env1, err1 := LoadEnvironment("dev")
	env2, err2 := ResolveEnvironment("dev")

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if env1.Namespace != env2.Namespace {
		t.Errorf("namespace mismatch: %s vs %s", env1.Namespace, env2.Namespace)
	}
	if env1.Context != env2.Context {
		t.Errorf("context mismatch: %s vs %s", env1.Context, env2.Context)
	}
}

func TestConfigOverridesShallowMerge(t *testing.T) {
	// ApplyConfigOverrides does a shallow merge: nested maps are replaced, not deep-merged
	wfConfig := map[string]interface{}{
		"database": map[string]interface{}{
			"host": "original-host",
			"port": 5432,
		},
	}
	overrides := map[string]interface{}{
		"database": map[string]interface{}{
			"host": "new-host",
			// port is not specified -- shallow merge replaces entire "database" key
		},
	}

	ApplyConfigOverrides(wfConfig, overrides)

	db, ok := wfConfig["database"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database to be a map")
	}
	if db["host"] != "new-host" {
		t.Errorf("expected new-host, got %v", db["host"])
	}
	// Since this is shallow merge, "port" should be gone
	if _, exists := db["port"]; exists {
		t.Error("expected port to be absent after shallow merge replacement")
	}
}

func TestConfigOverridesAddsNewKeys(t *testing.T) {
	wfConfig := map[string]interface{}{
		"existing_key": "value",
	}
	overrides := map[string]interface{}{
		"new_key": "new_value",
	}

	ApplyConfigOverrides(wfConfig, overrides)

	if wfConfig["existing_key"] != "value" {
		t.Errorf("expected existing_key preserved, got %v", wfConfig["existing_key"])
	}
	if wfConfig["new_key"] != "new_value" {
		t.Errorf("expected new_key added, got %v", wfConfig["new_key"])
	}
}

func TestEnvironmentConfigOverridesFieldTypes(t *testing.T) {
	// Verify ConfigOverrides can hold various types
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
	configYAML := `environments:
  dev:
    namespace: dev-ns
    config_overrides:
      string_val: hello
      int_val: 42
      bool_val: true
      float_val: 3.14
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	env, err := LoadEnvironment("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.ConfigOverrides == nil {
		t.Fatal("expected ConfigOverrides to be non-nil")
	}
	if env.ConfigOverrides["string_val"] != "hello" {
		t.Errorf("expected string_val hello, got %v", env.ConfigOverrides["string_val"])
	}
	if env.ConfigOverrides["int_val"] != 42 {
		t.Errorf("expected int_val 42, got %v", env.ConfigOverrides["int_val"])
	}
	if env.ConfigOverrides["bool_val"] != true {
		t.Errorf("expected bool_val true, got %v", env.ConfigOverrides["bool_val"])
	}
}

func TestResolveEnvironmentEmptyReturnsTopLevel(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Clear TENTACULAR_ENV to ensure we test the empty-name path
	origEnv := os.Getenv("TENTACULAR_ENV")
	os.Unsetenv("TENTACULAR_ENV")
	defer os.Setenv("TENTACULAR_ENV", origEnv)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	configYAML := `namespace: top-ns
runtime_class: top-rc
environments:
  dev:
    namespace: dev-ns
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	// ResolveEnvironment("") should return top-level config promoted
	env, err := ResolveEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "top-ns" {
		t.Errorf("expected top-ns, got %s", env.Namespace)
	}
	if env.RuntimeClass != "top-rc" {
		t.Errorf("expected top-rc, got %s", env.RuntimeClass)
	}
	// Should NOT contain dev env values
	if env.Context != "" {
		t.Errorf("expected empty context for top-level, got %s", env.Context)
	}
}

func TestResolveEnvironmentTENTACULAR_ENVFallback(t *testing.T) {
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
	configYAML := `namespace: top-ns
environments:
  staging:
    namespace: staging-ns
    context: staging-ctx
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	// Set TENTACULAR_ENV and call ResolveEnvironment with empty name
	origEnv := os.Getenv("TENTACULAR_ENV")
	os.Setenv("TENTACULAR_ENV", "staging")
	defer os.Setenv("TENTACULAR_ENV", origEnv)

	env, err := ResolveEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "staging-ns" {
		t.Errorf("expected staging-ns from TENTACULAR_ENV, got %s", env.Namespace)
	}
	if env.Context != "staging-ctx" {
		t.Errorf("expected staging-ctx from TENTACULAR_ENV, got %s", env.Context)
	}
}

func TestResolveEnvironmentExplicitOverridesTENTACULAR_ENV(t *testing.T) {
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
	configYAML := `environments:
  dev:
    namespace: dev-ns
  staging:
    namespace: staging-ns
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	// Set TENTACULAR_ENV to staging, but explicitly ask for dev
	origEnv := os.Getenv("TENTACULAR_ENV")
	os.Setenv("TENTACULAR_ENV", "staging")
	defer os.Setenv("TENTACULAR_ENV", origEnv)

	env, err := ResolveEnvironment("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Explicit name should win over TENTACULAR_ENV
	if env.Namespace != "dev-ns" {
		t.Errorf("expected dev-ns (explicit wins over env var), got %s", env.Namespace)
	}
}

func TestLoadEnvironmentEmptyNameReturnsTopLevel(t *testing.T) {
	cfg := &TentacularConfig{
		Namespace:    "default-ns",
		RuntimeClass: "default-rc",
		Environments: map[string]EnvironmentConfig{
			"dev": {Namespace: "dev-ns"},
		},
	}

	env, err := cfg.LoadEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Namespace != "default-ns" {
		t.Errorf("expected default-ns, got %s", env.Namespace)
	}
	if env.RuntimeClass != "default-rc" {
		t.Errorf("expected default-rc, got %s", env.RuntimeClass)
	}
}

func TestEnvironmentSecretsSource(t *testing.T) {
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
	configYAML := `environments:
  prod:
    namespace: prod-ns
    secrets_source: vault://prod/tentacular
`
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(configYAML), 0o644)

	env, err := LoadEnvironment("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.SecretsSource != "vault://prod/tentacular" {
		t.Errorf("expected vault://prod/tentacular, got %s", env.SecretsSource)
	}
}
