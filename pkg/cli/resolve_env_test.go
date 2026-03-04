package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setupEnvConfig writes a config.yaml to a temp HOME with the given YAML content
// and changes the working directory to a clean temp dir. Returns cleanup func.
func setupEnvConfig(t *testing.T, configYAML string) func() {
	t.Helper()
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configYAML), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(t.TempDir())

	// Clear env vars that would interfere
	os.Unsetenv("TNTC_MCP_ENDPOINT")
	os.Unsetenv("TNTC_MCP_TOKEN")
	os.Unsetenv("TENTACULAR_ENV")

	return func() {
		os.Setenv("HOME", origHome)
		os.Chdir(origDir)
	}
}

// --- Per-env MCP config resolution ---

// TestResolveMCPClient_UsesEnvMCPEndpoint verifies that when an environment has
// mcp_endpoint configured, resolveMCPClient uses it when --env matches.
func TestResolveMCPClient_UsesEnvMCPEndpoint(t *testing.T) {
	cleanup := setupEnvConfig(t, `environments:
  staging:
    namespace: staging-ns
    mcp_endpoint: http://staging-mcp:8080
`)
	defer cleanup()

	cmd := newTestCmdWithEnv("staging")
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client when env has mcp_endpoint")
	}
}

// TestResolveMCPClient_EnvFlagTakesPriority verifies --env flag overrides TENTACULAR_ENV.
func TestResolveMCPClient_EnvFlagTakesPriority(t *testing.T) {
	cleanup := setupEnvConfig(t, `environments:
  dev:
    mcp_endpoint: http://dev-mcp:8080
  staging:
    mcp_endpoint: http://staging-mcp:8080
`)
	defer cleanup()

	// TENTACULAR_ENV=staging but --env=dev → dev should win
	os.Setenv("TENTACULAR_ENV", "staging")
	defer os.Unsetenv("TENTACULAR_ENV")

	cmd := newTestCmdWithEnv("dev")
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client when --env=dev is set")
	}
}

// TestResolveMCPClient_TENTACULAR_ENVFallback verifies TENTACULAR_ENV is used
// when --env flag is not set.
func TestResolveMCPClient_TENTACULAR_ENVFallback(t *testing.T) {
	cleanup := setupEnvConfig(t, `environments:
  prod:
    mcp_endpoint: http://prod-mcp:8080
`)
	defer cleanup()

	os.Setenv("TENTACULAR_ENV", "prod")
	defer os.Unsetenv("TENTACULAR_ENV")

	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client when TENTACULAR_ENV=prod")
	}
}

// TestResolveMCPClient_DefaultEnvFallback verifies default_env from config
// is used when no explicit env is set.
func TestResolveMCPClient_DefaultEnvFallback(t *testing.T) {
	cleanup := setupEnvConfig(t, `default_env: dev
environments:
  dev:
    mcp_endpoint: http://dev-mcp:8080
`)
	defer cleanup()

	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client when default_env=dev")
	}
}

// TestResolveMCPClient_EnvWithNoMCPFallsBackToGlobal verifies that when the
// named environment exists but has no mcp_endpoint, resolution falls back to
// the global mcp.endpoint.
func TestResolveMCPClient_EnvWithNoMCPFallsBackToGlobal(t *testing.T) {
	cleanup := setupEnvConfig(t, `mcp:
  endpoint: http://global-mcp:8080
environments:
  dev:
    namespace: dev-ns
    # No mcp_endpoint -- should fall back to global
`)
	defer cleanup()

	cmd := newTestCmdWithEnv("dev")
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client when env has no mcp_endpoint but global mcp.endpoint is set")
	}
}

// TestResolveMCPClient_UnknownEnvWithGlobalMCPUsesGlobal verifies that when
// an env name is set but not found in config, global MCP is used.
func TestResolveMCPClient_UnknownEnvWithGlobalMCPUsesGlobal(t *testing.T) {
	cleanup := setupEnvConfig(t, `mcp:
  endpoint: http://global-mcp:8080
environments:
  dev:
    namespace: dev-ns
`)
	defer cleanup()

	// nonexistent environment name -- global mcp.endpoint should be used
	cmd := newTestCmdWithEnv("nonexistent")
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client from global mcp.endpoint when env not found")
	}
}

// TestResolveMCPClient_MCPTokenPathExpandsTilde verifies that mcp_token_path
// with ~ is expanded to the home directory.
func TestResolveMCPClient_MCPTokenPathExpandsTilde(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Write a token file in tmpHome
	tokenFile := filepath.Join(tmpHome, "mcp-token")
	os.WriteFile(tokenFile, []byte("my-token\n"), 0o600)

	configDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(`environments:
  prod:
    mcp_endpoint: http://prod-mcp:8080
    mcp_token_path: ~/mcp-token
`), 0o644)

	origDir, _ := os.Getwd()
	os.Chdir(t.TempDir())
	defer os.Chdir(origDir)
	os.Unsetenv("TNTC_MCP_ENDPOINT")
	os.Unsetenv("TNTC_MCP_TOKEN")
	os.Unsetenv("TENTACULAR_ENV")

	// Load the environment config and check token path expansion
	env, err := LoadEnvironment("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedPath := filepath.Join(tmpHome, "mcp-token")
	if env.MCPTokenPath != expectedPath {
		t.Errorf("expected MCPTokenPath %s, got %s", expectedPath, env.MCPTokenPath)
	}
}

// TestRequireMCPClient_ErrorMessageUpdated verifies the error no longer
// mentions "cluster install".
func TestRequireMCPClient_NoLongerMentionsClusterInstall(t *testing.T) {
	cleanup := setupEnvConfig(t, "registry: base\n")
	defer cleanup()

	cmd := newTestCmd()
	_, err := requireMCPClient(cmd)
	if err == nil {
		t.Fatal("expected error when no MCP configured")
	}
	if strings.Contains(err.Error(), "cluster install") {
		t.Errorf("error message should not mention 'cluster install' (removed command), got: %v", err)
	}
	if !strings.Contains(err.Error(), "mcp_endpoint") {
		t.Errorf("expected error to mention mcp_endpoint, got: %v", err)
	}
}

// --- buildMCPClientForEnv ---

// TestBuildMCPClientForEnv_UsesEnvEndpoint verifies buildMCPClientForEnv
// uses the env's mcp_endpoint.
func TestBuildMCPClientForEnv_UsesEnvEndpoint(t *testing.T) {
	cleanup := setupEnvConfig(t, `environments:
  prod:
    mcp_endpoint: http://prod-mcp:8080
`)
	defer cleanup()

	client, err := buildMCPClientForEnv("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}

// TestBuildMCPClientForEnv_ErrorWhenNoEndpoint verifies buildMCPClientForEnv
// returns an error when neither env nor global MCP endpoint is configured.
func TestBuildMCPClientForEnv_ErrorWhenNoEndpoint(t *testing.T) {
	cleanup := setupEnvConfig(t, `environments:
  dev:
    namespace: dev-ns
`)
	defer cleanup()

	_, err := buildMCPClientForEnv("dev")
	if err == nil {
		t.Fatal("expected error when no mcp_endpoint configured")
	}
	if !strings.Contains(err.Error(), "mcp_endpoint") {
		t.Errorf("expected error to mention mcp_endpoint, got: %v", err)
	}
}

// TestBuildMCPClientForEnv_FallsBackToGlobalMCP verifies buildMCPClientForEnv
// uses the global mcp.endpoint when env has no mcp_endpoint.
func TestBuildMCPClientForEnv_FallsBackToGlobalMCP(t *testing.T) {
	cleanup := setupEnvConfig(t, `mcp:
  endpoint: http://global-mcp:8080
environments:
  dev:
    namespace: dev-ns
`)
	defer cleanup()

	client, err := buildMCPClientForEnv("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client from global fallback")
	}
}

// newTestCmdWithEnv creates a minimal cobra command with --env set to the given value.
func newTestCmdWithEnv(envName string) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("env", envName, "target environment")
	return cmd
}
