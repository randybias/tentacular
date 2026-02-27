package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/mcp"
	"github.com/spf13/cobra"
)

// newTestCmd creates a minimal cobra command for resolve tests.
func newTestCmd() *cobra.Command {
	return &cobra.Command{Use: "test"}
}

// TestResolveMCPClient_NilWhenNotConfigured verifies that resolveMCPClient
// returns (nil, nil) when no MCP endpoint is configured.
func TestResolveMCPClient_NilWhenNotConfigured(t *testing.T) {
	os.Unsetenv("TNTC_MCP_ENDPOINT")
	os.Unsetenv("TNTC_MCP_TOKEN")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	os.Chdir(t.TempDir())
	defer os.Chdir(origDir)

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
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	os.Chdir(t.TempDir())
	defer os.Chdir(origDir)

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
	os.Unsetenv("TNTC_MCP_ENDPOINT")
	os.Unsetenv("TNTC_MCP_TOKEN")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	configDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("mcp:\n  endpoint: http://mcp-from-file:8080\n"),
		0o644)

	projectDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(projectDir)
	defer os.Chdir(origDir)

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
	os.Unsetenv("TNTC_MCP_ENDPOINT")
	os.Unsetenv("TNTC_MCP_TOKEN")

	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	os.Chdir(t.TempDir())
	defer os.Chdir(origDir)

	cmd := newTestCmd()
	_, err := requireMCPClient(cmd)
	if err == nil {
		t.Fatal("expected error when MCP not configured")
	}
	if !strings.Contains(err.Error(), "cluster install") {
		t.Errorf("expected error to mention cluster install, got: %v", err)
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
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	os.Chdir(t.TempDir())
	defer os.Chdir(origDir)

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

// TestMCPErrorHint_Unauthorized verifies the hint mentions rotate-token.
func TestMCPErrorHint_Unauthorized(t *testing.T) {
	authErr := &mcp.Error{Code: 401, Message: "unauthorized"}
	hint := mcpErrorHint(authErr)
	if !strings.Contains(hint, "rotate-token") {
		t.Errorf("expected hint to mention rotate-token for 401, got %q", hint)
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
