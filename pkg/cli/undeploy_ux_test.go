package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/mcp"
)

// makeMCPTestServer creates an httptest.Server backed by a real MCP SDK server
// with the given tool handlers. Returns the server and an mcp.Client connected to it.
func makeMCPTestServer(t *testing.T, tools map[string]func(args map[string]any) (string, bool)) (*httptest.Server, *mcp.Client) {
	t.Helper()

	mcpServer := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "test-mcp", Version: "test"},
		nil,
	)

	for name, handler := range tools {
		h := handler // capture
		mcpServer.AddTool(
			&mcpsdk.Tool{
				Name:        name,
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
			func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
				var args map[string]any
				if req.Params.Arguments != nil {
					if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
						args = map[string]any{}
					}
				} else {
					args = map[string]any{}
				}
				text, isErr := h(args)
				return &mcpsdk.CallToolResult{
					IsError: isErr,
					Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
				}, nil
			},
		)
	}

	mcpHandler := mcpsdk.NewStreamableHTTPHandler(
		func(r *http.Request) *mcpsdk.Server { return mcpServer },
		nil,
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = false
	srv.Start()
	client := mcp.NewClient(mcp.Config{
		Endpoint: srv.URL + "/mcp",
		Token:    "test-token",
		Timeout:  5 * time.Second,
	})

	return srv, client
}

// closeMCPTestServer closes server connections without waiting for graceful shutdown.
// This avoids the 30s wait when MCP streaming connections are still active
// (e.g., when the client was created internally by requireMCPClient).
func closeMCPTestServer(srv *httptest.Server) {
	srv.CloseClientConnections()
	srv.Close()
}

// newUndeployTestCmd creates a minimal cobra command with the flags needed by undeploy helpers.
func newUndeployTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	cmd.Flags().BoolP("yes", "y", false, "Skip confirmation")
	cmd.Flags().Bool("force", false, "Skip exo confirmation")
	cmd.Flags().Bool("detail", false, "Show details")
	return cmd
}

// --- checkExoskeletonCleanup tests ---

func TestCheckExoskeletonCleanup_FullWarning(t *testing.T) {
	exoStatusJSON, _ := json.Marshal(map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": true,
		"postgres_available":  true,
		"nats_available":      true,
		"rustfs_available":    true,
	})
	exoRegJSON, _ := json.Marshal(map[string]any{
		"found":     true,
		"namespace": "test-ns",
		"name":      "my-wf",
	})

	srv, client := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return string(exoStatusJSON), false
		},
		"exo_registration": func(_ map[string]any) (string, bool) {
			return string(exoRegJSON), false
		},
	})
	defer closeMCPTestServer(srv)
	defer func() { _ = client.Close() }()

	cmd := newUndeployTestCmd()
	cmd.SetContext(context.Background())

	warning := checkExoskeletonCleanup(cmd, client, "test-ns", "my-wf")
	if warning == "" {
		t.Fatal("expected non-empty warning")
	}
	if !strings.Contains(warning, "WARNING: Exoskeleton cleanup is enabled") {
		t.Error("expected warning header")
	}
	if !strings.Contains(warning, "Postgres schema") {
		t.Error("expected Postgres in warning")
	}
	if !strings.Contains(warning, "RustFS objects") {
		t.Error("expected RustFS in warning")
	}
	if !strings.Contains(warning, "NATS authorization") {
		t.Error("expected NATS in warning")
	}
}

func TestCheckExoskeletonCleanup_CleanupDisabled(t *testing.T) {
	exoStatusJSON, _ := json.Marshal(map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": false,
		"postgres_available":  true,
	})

	srv, client := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return string(exoStatusJSON), false
		},
	})
	defer closeMCPTestServer(srv)
	defer func() { _ = client.Close() }()

	cmd := newUndeployTestCmd()
	cmd.SetContext(context.Background())

	warning := checkExoskeletonCleanup(cmd, client, "ns", "wf")
	if warning != "" {
		t.Errorf("expected no warning when cleanup_on_undeploy=false, got %q", warning)
	}
}

func TestCheckExoskeletonCleanup_StatusError(t *testing.T) {
	srv, client := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return "internal server error", true
		},
	})
	defer closeMCPTestServer(srv)
	defer func() { _ = client.Close() }()

	cmd := newUndeployTestCmd()
	cmd.SetContext(context.Background())

	warning := checkExoskeletonCleanup(cmd, client, "ns", "wf")
	if warning != "" {
		t.Errorf("expected no warning on exo_status error (graceful degradation), got %q", warning)
	}
}

func TestCheckExoskeletonCleanup_NotRegistered(t *testing.T) {
	exoStatusJSON, _ := json.Marshal(map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": true,
		"postgres_available":  true,
	})
	exoRegJSON, _ := json.Marshal(map[string]any{
		"found":     false,
		"namespace": "ns",
		"name":      "wf",
	})

	srv, client := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return string(exoStatusJSON), false
		},
		"exo_registration": func(_ map[string]any) (string, bool) {
			return string(exoRegJSON), false
		},
	})
	defer closeMCPTestServer(srv)
	defer func() { _ = client.Close() }()

	cmd := newUndeployTestCmd()
	cmd.SetContext(context.Background())

	warning := checkExoskeletonCleanup(cmd, client, "ns", "wf")
	if warning != "" {
		t.Errorf("expected no warning for unregistered workflow, got %q", warning)
	}
}

func TestCheckExoskeletonCleanup_RegistrationError(t *testing.T) {
	exoStatusJSON, _ := json.Marshal(map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": true,
		"postgres_available":  true,
	})

	srv, client := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return string(exoStatusJSON), false
		},
		"exo_registration": func(_ map[string]any) (string, bool) {
			return "not found", true
		},
	})
	defer closeMCPTestServer(srv)
	defer func() { _ = client.Close() }()

	cmd := newUndeployTestCmd()
	cmd.SetContext(context.Background())

	warning := checkExoskeletonCleanup(cmd, client, "ns", "wf")
	if warning != "" {
		t.Errorf("expected no warning on registration error (graceful degradation), got %q", warning)
	}
}

func TestCheckExoskeletonCleanup_OnlyPostgres(t *testing.T) {
	exoStatusJSON, _ := json.Marshal(map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": true,
		"postgres_available":  true,
		"nats_available":      false,
		"rustfs_available":    false,
	})
	exoRegJSON, _ := json.Marshal(map[string]any{
		"found":     true,
		"namespace": "ns",
		"name":      "wf",
	})

	srv, client := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return string(exoStatusJSON), false
		},
		"exo_registration": func(_ map[string]any) (string, bool) {
			return string(exoRegJSON), false
		},
	})
	defer closeMCPTestServer(srv)
	defer func() { _ = client.Close() }()

	cmd := newUndeployTestCmd()
	cmd.SetContext(context.Background())

	warning := checkExoskeletonCleanup(cmd, client, "ns", "wf")
	if !strings.Contains(warning, "Postgres schema") {
		t.Error("expected Postgres in warning")
	}
	if strings.Contains(warning, "RustFS") {
		t.Error("should NOT contain RustFS when not available")
	}
	if strings.Contains(warning, "NATS") {
		t.Error("should NOT contain NATS when not available")
	}
}

// --- runUndeployWith tests ---

// setupMCPEnv sets up a temp HOME with config pointing to the given endpoint,
// and sets TNTC_MCP_ENDPOINT as a simpler override.
func setupMCPEnv(t *testing.T, endpoint string) func() {
	t.Helper()

	origHome := os.Getenv("HOME")
	origEndpoint := os.Getenv("TNTC_MCP_ENDPOINT")
	origToken := os.Getenv("TNTC_MCP_TOKEN")
	origEnv := os.Getenv("TENTACULAR_ENV")

	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	_ = os.Setenv("TNTC_MCP_ENDPOINT", endpoint)
	_ = os.Setenv("TNTC_MCP_TOKEN", "test-token")
	_ = os.Unsetenv("TENTACULAR_ENV")

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	// Create minimal config
	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(""), 0o644)

	return func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("TNTC_MCP_ENDPOINT", origEndpoint)
		_ = os.Setenv("TNTC_MCP_TOKEN", origToken)
		_ = os.Setenv("TENTACULAR_ENV", origEnv)
		_ = os.Chdir(origDir)
	}
}

func TestRunUndeployWith_UserConfirmsY(t *testing.T) {
	wfRemoveJSON, _ := json.Marshal(map[string]any{
		"deleted": []string{"Deployment/my-wf", "Service/my-wf"},
	})

	var removeCalled bool
	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			j, _ := json.Marshal(map[string]any{"enabled": false})
			return string(j), false
		},
		"wf_remove": func(_ map[string]any) (string, bool) {
			removeCalled = true
			return string(wfRemoveJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewUndeployCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	stdin := bytes.NewBufferString("y\n")
	err := runUndeployWith(cmd, []string{"my-wf"}, stdin)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("runUndeployWith: %v", err)
	}
	if !removeCalled {
		t.Error("expected wf_remove to be called when user confirms with 'y'")
	}
}

func TestRunUndeployWith_UserDeclinesN(t *testing.T) {
	var removeCalled bool
	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			j, _ := json.Marshal(map[string]any{"enabled": false})
			return string(j), false
		},
		"wf_remove": func(_ map[string]any) (string, bool) {
			removeCalled = true
			return `{"deleted":[]}`, false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewUndeployCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	stdin := bytes.NewBufferString("n\n")
	err := runUndeployWith(cmd, []string{"my-wf"}, stdin)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runUndeployWith: %v", err)
	}
	if removeCalled {
		t.Error("wf_remove should NOT be called when user declines")
	}
}

func TestRunUndeployWith_YesFlag(t *testing.T) {
	wfRemoveJSON, _ := json.Marshal(map[string]any{
		"deleted": []string{"Deployment/my-wf"},
	})

	var removeCalled bool
	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			j, _ := json.Marshal(map[string]any{"enabled": false})
			return string(j), false
		},
		"wf_remove": func(_ map[string]any) (string, bool) {
			removeCalled = true
			return string(wfRemoveJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewUndeployCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	_ = cmd.Flags().Set("yes", "true")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	stdin := bytes.NewBufferString("") // no stdin needed
	err := runUndeployWith(cmd, []string{"my-wf"}, stdin)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runUndeployWith with -y: %v", err)
	}
	if !removeCalled {
		t.Error("wf_remove should be called when -y is set")
	}
}

func TestRunUndeployWith_ForceFlag(t *testing.T) {
	// Set up a server that reports exo cleanup is active
	exoStatusJSON, _ := json.Marshal(map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": true,
		"postgres_available":  true,
		"nats_available":      true,
		"rustfs_available":    true,
	})
	exoRegJSON, _ := json.Marshal(map[string]any{
		"found":     true,
		"namespace": "default",
		"name":      "my-wf",
	})
	wfRemoveJSON, _ := json.Marshal(map[string]any{
		"deleted": []string{"Deployment/my-wf"},
	})

	var removeCalled bool
	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return string(exoStatusJSON), false
		},
		"exo_registration": func(_ map[string]any) (string, bool) {
			return string(exoRegJSON), false
		},
		"wf_remove": func(_ map[string]any) (string, bool) {
			removeCalled = true
			return string(wfRemoveJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewUndeployCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	_ = cmd.Flags().Set("yes", "true")
	_ = cmd.Flags().Set("force", "true")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	stdin := bytes.NewBufferString("") // --force + -y skips all prompts
	err := runUndeployWith(cmd, []string{"my-wf"}, stdin)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runUndeployWith with --force -y: %v", err)
	}
	if !removeCalled {
		t.Error("wf_remove should be called when --force and -y are set")
	}
}

func TestRunUndeployWith_ExoWarningUserDeclines(t *testing.T) {
	exoStatusJSON, _ := json.Marshal(map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": true,
		"postgres_available":  true,
	})
	exoRegJSON, _ := json.Marshal(map[string]any{
		"found":     true,
		"namespace": "default",
		"name":      "my-wf",
	})

	var removeCalled bool
	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			return string(exoStatusJSON), false
		},
		"exo_registration": func(_ map[string]any) (string, bool) {
			return string(exoRegJSON), false
		},
		"wf_remove": func(_ map[string]any) (string, bool) {
			removeCalled = true
			return `{"deleted":[]}`, false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewUndeployCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	_ = cmd.Flags().Set("yes", "true") // skip basic confirm
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// User says "y" to basic confirm (skipped by -y) but "n" to exo warning
	stdin := bytes.NewBufferString("n\n")
	err := runUndeployWith(cmd, []string{"my-wf"}, stdin)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runUndeployWith: %v", err)
	}
	if removeCalled {
		t.Error("wf_remove should NOT be called when user declines exo cleanup warning")
	}
}

func TestRunUndeployWith_ExoCleanupResult(t *testing.T) {
	wfRemoveJSON, _ := json.Marshal(map[string]any{
		"deleted":             3,
		"exo_cleaned_up":      true,
		"exo_cleanup_details": "postgres schema dropped, rustfs user removed",
	})

	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			j, _ := json.Marshal(map[string]any{"enabled": false})
			return string(j), false
		},
		"wf_remove": func(_ map[string]any) (string, bool) {
			return string(wfRemoveJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewUndeployCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	_ = cmd.Flags().Set("yes", "true")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runUndeployWith(cmd, []string{"my-wf"}, bytes.NewBufferString(""))

	w.Close()
	os.Stdout = oldStdout
	var output bytes.Buffer
	_, _ = output.ReadFrom(r)

	if err != nil {
		t.Fatalf("runUndeployWith: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "Exoskeleton cleanup") {
		t.Errorf("expected exo cleanup output, got: %s", out)
	}
}

func TestRunUndeployWith_NoResources(t *testing.T) {
	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"exo_status": func(_ map[string]any) (string, bool) {
			j, _ := json.Marshal(map[string]any{"enabled": false})
			return string(j), false
		},
		"wf_remove": func(_ map[string]any) (string, bool) {
			return `{"deleted":0}`, false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewUndeployCmd()
	cmd.PersistentFlags().StringP("env", "e", "", "Target environment")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	_ = cmd.Flags().Set("yes", "true")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runUndeployWith(cmd, []string{"ghost-wf"}, bytes.NewBufferString(""))

	w.Close()
	os.Stdout = oldStdout
	var output bytes.Buffer
	_, _ = output.ReadFrom(r)

	if err != nil {
		t.Fatalf("runUndeployWith: %v", err)
	}

	out := output.String()
	_ = out // avoid unused
	if !strings.Contains(out, "No resources found") {
		t.Errorf("expected 'No resources found', got: %s", out)
	}
}
