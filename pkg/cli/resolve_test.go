package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/mcp"
)

// newTestCmd creates a minimal cobra command for resolve tests.
func newTestCmd() *cobra.Command {
	return &cobra.Command{Use: "test"}
}

// TestResolveMCPClient_NilWhenNotConfigured verifies that resolveMCPClient
// returns (nil, nil) when no MCP endpoint is configured.
func TestResolveMCPClient_NilWhenNotConfigured(t *testing.T) {
	_ = os.Unsetenv("TNTC_MCP_ENDPOINT")
	_ = os.Unsetenv("TNTC_MCP_TOKEN")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Error("expected nil client when no MCP configured")
	}
}

// TestResolveMCPClient_ReturnsClientWhenEnvSet verifies that resolveMCPClient
// returns a non-nil client when TNTC_MCP_ENDPOINT is set.
func TestResolveMCPClient_ReturnsClientWhenEnvSet(t *testing.T) {
	t.Setenv("TNTC_MCP_ENDPOINT", "http://mcp.test:8080")
	t.Setenv("TNTC_MCP_TOKEN", "test-token")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client when TNTC_MCP_ENDPOINT is set")
	}
}

// TestResolveMCPClient_ReturnsClientWhenConfigFileSet verifies config file path.
func TestResolveMCPClient_ReturnsClientWhenConfigFileSet(t *testing.T) {
	_ = os.Unsetenv("TNTC_MCP_ENDPOINT")
	_ = os.Unsetenv("TNTC_MCP_TOKEN")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(configDir, 0o755)
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("mcp:\n  endpoint: http://mcp-from-file:8080\n"),
		0o644)

	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client when config file sets mcp.endpoint")
	}
}

// TestRequireMCPClient_ErrorWhenNotConfigured verifies requireMCPClient returns
// an error when MCP is not configured.
func TestRequireMCPClient_ErrorWhenNotConfigured(t *testing.T) {
	_ = os.Unsetenv("TNTC_MCP_ENDPOINT")
	_ = os.Unsetenv("TNTC_MCP_TOKEN")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestCmd()
	_, err := requireMCPClient(cmd)
	if err == nil {
		t.Fatal("expected error when MCP not configured")
	}
	if !strings.Contains(err.Error(), "MCP server not configured") {
		t.Errorf("expected error to mention MCP server not configured, got: %v", err)
	}
}

// TestRequireMCPClient_SuccessWhenConfigured verifies requireMCPClient returns
// a client when configured.
func TestRequireMCPClient_SuccessWhenConfigured(t *testing.T) {
	t.Setenv("TNTC_MCP_ENDPOINT", "http://mcp.test:8080")
	t.Setenv("TNTC_MCP_TOKEN", "tok")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newTestCmd()
	client, err := requireMCPClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
}

// TestMCPErrorHint_NilReturnsEmpty verifies nil error gives empty hint.
func TestMCPErrorHint_NilReturnsEmpty(t *testing.T) {
	hint := mcpErrorHint(nil)
	if hint != "" {
		t.Errorf("expected empty string for nil error, got %q", hint)
	}
}

// TestMCPErrorHint_UnknownError verifies unknown error gives empty hint.
func TestMCPErrorHint_UnknownError(t *testing.T) {
	hint := mcpErrorHint(errors.New("some random error"))
	if hint != "" {
		t.Errorf("expected empty string for unknown error, got %q", hint)
	}
}

// TestMCPErrorHint_ServerUnavailable verifies the hint mentions kubectl.
func TestMCPErrorHint_ServerUnavailable(t *testing.T) {
	// Create a ServerUnavailableError through the mcp package
	serverErr := &mcp.ServerUnavailableError{
		Endpoint: "http://mcp:8080",
		Cause:    errors.New("connection refused"),
	}
	hint := mcpErrorHint(serverErr)
	if !strings.Contains(hint, "kubectl") {
		t.Errorf("expected hint to mention kubectl for server unavailable, got %q", hint)
	}
}

// TestMCPErrorHint_Unauthorized verifies the hint mentions token config.
func TestMCPErrorHint_Unauthorized(t *testing.T) {
	authErr := &mcp.Error{Code: 401, Message: "unauthorized"}
	hint := mcpErrorHint(authErr)
	if !strings.Contains(hint, "token") {
		t.Errorf("expected hint to mention token for 401, got %q", hint)
	}
}

// TestMCPErrorHint_Forbidden verifies the hint mentions namespace permissions.
func TestMCPErrorHint_Forbidden(t *testing.T) {
	forbiddenErr := &mcp.Error{Code: 403, Message: "forbidden"}
	hint := mcpErrorHint(forbiddenErr)
	if !strings.Contains(hint, "namespace") {
		t.Errorf("expected hint to mention namespace for 403, got %q", hint)
	}
}

// --- Issue #79: bearer-token fallback tests ---

// writeTestOIDCToken writes an OIDCTokenStore to the expected path for an environment.
func writeTestOIDCToken(t *testing.T, home, envName string, store *OIDCTokenStore) {
	t.Helper()
	dir := filepath.Join(home, ".tentacular", "tokens")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, envName+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestResolveMCPClient_OIDCFailure_NoFallback verifies that when OIDC is configured
// but expired (no refresh token), resolveMCPClient returns a hard error instead of
// silently falling back to the static bearer token. This is the core fix for issue #79.
func TestResolveMCPClient_OIDCFailure_NoFallback(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("TNTC_MCP_ENDPOINT", "http://mcp.test:8080")
	t.Setenv("TNTC_MCP_TOKEN", "superuser-bearer-token")
	t.Setenv("TENTACULAR_ENV", "")

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	// Write an expired OIDC token with no refresh token.
	writeTestOIDCToken(t, tmpHome, "default", &OIDCTokenStore{
		AccessToken: "expired-token",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	})

	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err == nil {
		t.Fatal("expected hard error when OIDC is configured but failed; got nil (silent fallback to bearer token)")
	}
	if client != nil {
		t.Fatal("expected nil client on OIDC failure")
	}
	if !strings.Contains(err.Error(), "tntc login") {
		t.Errorf("error should tell user to run 'tntc login', got: %v", err)
	}
}

// TestResolveMCPClient_NoOIDC_BearerTokenWorks verifies that when no OIDC is
// configured and a static bearer token is available, the client is created
// successfully. This is the admin/bootstrap case.
func TestResolveMCPClient_NoOIDC_BearerTokenWorks(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("TNTC_MCP_ENDPOINT", "http://mcp.test:8080")
	t.Setenv("TNTC_MCP_TOKEN", "admin-bearer-token")
	t.Setenv("TENTACULAR_ENV", "")

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	// No OIDC token file — resolveOIDCToken returns ("", nil).
	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("expected no error for bearer-token-only mode, got: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client in bearer-token-only mode")
	}
}

// TestResolveMCPClient_OIDCValid verifies the happy path: OIDC token is valid,
// client is created with the OIDC token (no fallback needed).
func TestResolveMCPClient_OIDCValid(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("TNTC_MCP_ENDPOINT", "http://mcp.test:8080")
	t.Setenv("TNTC_MCP_TOKEN", "")
	t.Setenv("TENTACULAR_ENV", "")

	origDir, _ := os.Getwd()
	_ = os.Chdir(t.TempDir())
	defer func() { _ = os.Chdir(origDir) }()

	writeTestOIDCToken(t, tmpHome, "default", &OIDCTokenStore{
		AccessToken: "good-oidc-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	})

	cmd := newTestCmd()
	client, err := resolveMCPClient(cmd)
	if err != nil {
		t.Fatalf("expected no error with valid OIDC token, got: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client with valid OIDC token")
	}
}

// TestResolveOIDCToken_NoTokenFile verifies that when no OIDC token file exists,
// resolveOIDCToken returns empty string and no error (allowing bearer fallback).
func TestResolveOIDCToken_NoTokenFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	token, err := resolveOIDCToken("testenv")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty token, got: %q", token)
	}
}

// TestResolveOIDCToken_ExpiredNoRefresh verifies that an expired OIDC token
// with no refresh token returns an error (not empty string).
func TestResolveOIDCToken_ExpiredNoRefresh(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	writeTestOIDCToken(t, tmpHome, "testenv", &OIDCTokenStore{
		AccessToken: "expired-token",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	})

	token, err := resolveOIDCToken("testenv")
	if err == nil {
		t.Fatal("expected error for expired OIDC token with no refresh token")
	}
	if token != "" {
		t.Fatalf("expected empty token on error, got: %q", token)
	}
	if !strings.Contains(err.Error(), "tntc login") {
		t.Errorf("error should mention 'tntc login', got: %v", err)
	}
}
