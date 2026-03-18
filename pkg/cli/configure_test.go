package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// setupConfigTest creates a temp HOME and workdir, returning a cleanup function.
func setupConfigTest(t *testing.T) (tmpHome string, cleanup func()) {
	t.Helper()

	origHome := os.Getenv("HOME")
	tmpHome = t.TempDir()
	_ = os.Setenv("HOME", tmpHome)

	origEnv := os.Getenv("TENTACULAR_ENV")
	_ = os.Unsetenv("TENTACULAR_ENV")

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cleanup = func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("TENTACULAR_ENV", origEnv)
		_ = os.Chdir(origDir)
	}
	return tmpHome, cleanup
}

func TestConfigure_TopLevelFlags(t *testing.T) {
	tmpHome, cleanup := setupConfigTest(t)
	defer cleanup()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)

	_ = cmd.Flags().Set("registry", "ghcr.io/myorg")
	_ = cmd.Flags().Set("default-namespace", "myapp")
	_ = cmd.Flags().Set("runtime-class", "gvisor")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runConfigure: %v", err)
	}

	// Verify config was written
	cfgPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	if cfg.Registry != "ghcr.io/myorg" {
		t.Errorf("registry: got %q, want %q", cfg.Registry, "ghcr.io/myorg")
	}
	if cfg.Namespace != "myapp" {
		t.Errorf("namespace: got %q, want %q", cfg.Namespace, "myapp")
	}
	if cfg.RuntimeClass != "gvisor" {
		t.Errorf("runtime_class: got %q, want %q", cfg.RuntimeClass, "gvisor")
	}
}

func TestConfigure_EnvScoped_NewEnvironment(t *testing.T) {
	tmpHome, cleanup := setupConfigTest(t)
	defer cleanup()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	_ = cmd.PersistentFlags().Set("env", "staging")
	_ = cmd.Flags().Set("oidc-issuer", "https://auth.example.com/realms/dev")
	_ = cmd.Flags().Set("oidc-client-id", "myclient")
	_ = cmd.Flags().Set("oidc-client-secret", "mysecret")
	_ = cmd.Flags().Set("mcp-endpoint", "https://mcp.example.com")
	_ = cmd.Flags().Set("kubeconfig", "~/.kube/staging.yaml")
	_ = cmd.Flags().Set("context", "staging-ctx")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runConfigure: %v", err)
	}

	// Verify config
	cfgPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	env, ok := cfg.Environments["staging"]
	if !ok {
		t.Fatal("staging environment not found in config")
	}
	if env.OIDCIssuer != "https://auth.example.com/realms/dev" {
		t.Errorf("oidc_issuer: got %q", env.OIDCIssuer)
	}
	if env.OIDCClientID != "myclient" {
		t.Errorf("oidc_client_id: got %q", env.OIDCClientID)
	}
	if env.OIDCClientSecret != "mysecret" {
		t.Errorf("oidc_client_secret: got %q", env.OIDCClientSecret)
	}
	if env.MCPEndpoint != "https://mcp.example.com" {
		t.Errorf("mcp_endpoint: got %q", env.MCPEndpoint)
	}
	if env.Kubeconfig != "~/.kube/staging.yaml" {
		t.Errorf("kubeconfig: got %q", env.Kubeconfig)
	}
	if env.Context != "staging-ctx" {
		t.Errorf("context: got %q", env.Context)
	}
}

func TestConfigure_EnvScoped_UpdatePreservesOtherFields(t *testing.T) {
	tmpHome, cleanup := setupConfigTest(t)
	defer cleanup()

	// Pre-populate config with existing environment
	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`environments:
  prod:
    namespace: prod-ns
    oidc_issuer: https://auth.example.com/realms/prod
    oidc_client_id: existing-client
    kubeconfig: ~/.kube/prod.yaml
`), 0o644)

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Only update the MCP endpoint -- other fields should be preserved
	_ = cmd.PersistentFlags().Set("env", "prod")
	_ = cmd.Flags().Set("mcp-endpoint", "https://mcp.prod.example.com")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runConfigure: %v", err)
	}

	cfgPath := filepath.Join(cfgDir, "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	env := cfg.Environments["prod"]
	if env.MCPEndpoint != "https://mcp.prod.example.com" {
		t.Errorf("mcp_endpoint: got %q", env.MCPEndpoint)
	}
	// Preserved fields
	if env.Namespace != "prod-ns" {
		t.Errorf("namespace should be preserved: got %q", env.Namespace)
	}
	if env.OIDCIssuer != "https://auth.example.com/realms/prod" {
		t.Errorf("oidc_issuer should be preserved: got %q", env.OIDCIssuer)
	}
	if env.OIDCClientID != "existing-client" {
		t.Errorf("oidc_client_id should be preserved: got %q", env.OIDCClientID)
	}
	if env.Kubeconfig != "~/.kube/prod.yaml" {
		t.Errorf("kubeconfig should be preserved: got %q", env.Kubeconfig)
	}
}

func TestConfigure_DefaultEnv(t *testing.T) {
	tmpHome, cleanup := setupConfigTest(t)
	defer cleanup()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)

	_ = cmd.Flags().Set("default-env", "staging")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runConfigure: %v", err)
	}

	cfgPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	if cfg.DefaultEnv != "staging" {
		t.Errorf("default_env: got %q, want %q", cfg.DefaultEnv, "staging")
	}
}

func TestConfigure_SSOWithAllFlags_SkipsPrompts(t *testing.T) {
	tmpHome, cleanup := setupConfigTest(t)
	defer cleanup()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Provide all OIDC fields via flags -- --sso should not prompt
	_ = cmd.PersistentFlags().Set("env", "test")
	_ = cmd.Flags().Set("sso", "true")
	_ = cmd.Flags().Set("oidc-issuer", "https://auth.example.com/realms/test")
	_ = cmd.Flags().Set("oidc-client-id", "testclient")
	_ = cmd.Flags().Set("oidc-client-secret", "testsecret")
	_ = cmd.Flags().Set("mcp-endpoint", "https://mcp.test.example.com")

	// Stdin is empty -- if prompts fired, they would fail
	cmd.SetIn(bytes.NewReader(nil))

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runConfigure with --sso and all flags: %v", err)
	}

	cfgPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing config: %v", err)
	}

	env, ok := cfg.Environments["test"]
	if !ok {
		t.Fatal("test environment not found")
	}
	if env.OIDCIssuer != "https://auth.example.com/realms/test" {
		t.Errorf("oidc_issuer: got %q", env.OIDCIssuer)
	}
	if env.OIDCClientID != "testclient" {
		t.Errorf("oidc_client_id: got %q", env.OIDCClientID)
	}
}

func TestConfigure_SecretPresent_FilePermissions0600(t *testing.T) {
	tmpHome, cleanup := setupConfigTest(t)
	defer cleanup()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	_ = cmd.PersistentFlags().Set("env", "secure")
	_ = cmd.Flags().Set("oidc-issuer", "https://auth.example.com/realms/secure")
	_ = cmd.Flags().Set("oidc-client-id", "secureclient")
	_ = cmd.Flags().Set("oidc-client-secret", "topsecret")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runConfigure: %v", err)
	}

	cfgPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions: got %o, want 0600", perm)
	}
}

func TestConfigure_ProjectConfig(t *testing.T) {
	_, cleanup := setupConfigTest(t)
	defer cleanup()

	// Get the workdir (Chdir'd by setupConfigTest)
	workdir, _ := os.Getwd()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	_ = cmd.Flags().Set("project", "true")
	_ = cmd.PersistentFlags().Set("env", "dev")
	_ = cmd.Flags().Set("oidc-issuer", "https://auth.example.com/realms/dev")
	_ = cmd.Flags().Set("oidc-client-id", "devclient")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("runConfigure --project: %v", err)
	}

	cfgPath := filepath.Join(workdir, ".tentacular", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading project config: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing project config: %v", err)
	}

	env, ok := cfg.Environments["dev"]
	if !ok {
		t.Fatal("dev environment not found in project config")
	}
	if env.OIDCIssuer != "https://auth.example.com/realms/dev" {
		t.Errorf("oidc_issuer: got %q", env.OIDCIssuer)
	}
}

func TestConfigure_EnvFlagWithoutEnv_Errors(t *testing.T) {
	_, cleanup := setupConfigTest(t)
	defer cleanup()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)

	// Try setting an env-scoped flag without --env
	_ = cmd.Flags().Set("oidc-issuer", "https://auth.example.com")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when using --oidc-issuer without --env")
	}
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestConfigure_SSOWithoutEnv_Errors(t *testing.T) {
	_, cleanup := setupConfigTest(t)
	defer cleanup()

	cmd := NewConfigureCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")

	var out bytes.Buffer
	cmd.SetOut(&out)

	_ = cmd.Flags().Set("sso", "true")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when using --sso without --env")
	}
}
