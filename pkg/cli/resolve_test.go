package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

// TestMCPErrorHint_Unauthorized verifies the hint mentions re-authentication.
func TestMCPErrorHint_Unauthorized(t *testing.T) {
	authErr := &mcp.Error{Code: 401, Message: "unauthorized"}
	hint := mcpErrorHint(authErr)
	if !strings.Contains(hint, "tntc login") {
		t.Errorf("expected hint to mention tntc login for 401, got %q", hint)
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
	t.Setenv("TENTACULAR_CLUSTER", "")

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
	t.Setenv("TENTACULAR_CLUSTER", "")

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
	t.Setenv("TENTACULAR_CLUSTER", "")

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

// --- TNTC_ACCESS_TOKEN env var tests (transitive trust) ---

// makeTestJWT builds a minimal JWT with the given claims for testing.
// The signature is invalid but DecodeJWTClaims doesn't verify signatures.
func makeTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf("%s.%s.fake-signature", header, payloadB64)
}

// TestResolveOIDCToken_EnvVarOverride verifies that TNTC_ACCESS_TOKEN takes
// precedence over cached token files.
func TestResolveOIDCToken_EnvVarOverride(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a cached token that would normally be returned.
	writeTestOIDCToken(t, tmpHome, "default", &OIDCTokenStore{
		AccessToken: "cached-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	})

	// Set env var with a valid JWT that expires in the future.
	envJWT := makeTestJWT(t, map[string]any{
		"sub":   "user-123",
		"email": "rbias@mirantis.com",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
	})
	t.Setenv("TNTC_ACCESS_TOKEN", envJWT)

	token, err := resolveOIDCToken("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != envJWT {
		t.Errorf("expected env var token to take precedence, got cached token instead")
	}
}

// TestResolveOIDCToken_EnvVarExpired verifies that an expired TNTC_ACCESS_TOKEN
// returns an error when no refresh token is available.
func TestResolveOIDCToken_EnvVarExpired(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	expiredJWT := makeTestJWT(t, map[string]any{
		"sub": "user-123",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})
	t.Setenv("TNTC_ACCESS_TOKEN", expiredJWT)
	_ = os.Unsetenv("TNTC_REFRESH_TOKEN")

	token, err := resolveOIDCToken("")
	if err == nil {
		t.Fatal("expected error for expired TNTC_ACCESS_TOKEN with no refresh token")
	}
	if token != "" {
		t.Fatalf("expected empty token on error, got: %q", token)
	}
	if !strings.Contains(err.Error(), "TNTC_ACCESS_TOKEN expired") {
		t.Errorf("expected error about expired env var token, got: %v", err)
	}
}

// TestResolveOIDCToken_EnvVarInvalidJWT verifies that a non-JWT TNTC_ACCESS_TOKEN
// returns a clear error.
func TestResolveOIDCToken_EnvVarInvalidJWT(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	t.Setenv("TNTC_ACCESS_TOKEN", "not-a-jwt")

	token, err := resolveOIDCToken("")
	if err == nil {
		t.Fatal("expected error for invalid JWT in TNTC_ACCESS_TOKEN")
	}
	if token != "" {
		t.Fatalf("expected empty token on error, got: %q", token)
	}
	if !strings.Contains(err.Error(), "not a valid JWT") {
		t.Errorf("expected error about invalid JWT, got: %v", err)
	}
}

// TestResolveOIDCToken_EnvVarNoExpClaim verifies that a JWT without an exp claim
// is accepted (some tokens may not have exp).
func TestResolveOIDCToken_EnvVarNoExpClaim(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	noExpJWT := makeTestJWT(t, map[string]any{
		"sub":   "user-123",
		"email": "rbias@mirantis.com",
	})
	t.Setenv("TNTC_ACCESS_TOKEN", noExpJWT)

	token, err := resolveOIDCToken("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != noExpJWT {
		t.Error("expected token to be returned when no exp claim present")
	}
}
