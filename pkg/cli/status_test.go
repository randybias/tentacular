package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/mcp"
)

func TestStatusCmd_BasicOutput(t *testing.T) {
	statusJSON, _ := json.Marshal(map[string]any{
		"name":      "my-app",
		"enclave":   "staging",
		"version":   "v1.2.0",
		"ready":     true,
		"replicas":  3,
		"available": 3,
	})

	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"wf_status": func(_ map[string]any) (string, bool) {
			return string(statusJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewStatusCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.RunE(cmd, []string{"my-app"})

	w.Close()
	os.Stdout = oldStdout
	var output bytes.Buffer
	_, _ = output.ReadFrom(r)

	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "Name:      my-app") {
		t.Errorf("expected deployment name, got:\n%s", out)
	}
	if !strings.Contains(out, "Namespace: staging") {
		t.Errorf("expected namespace, got:\n%s", out)
	}
	if !strings.Contains(out, "Version:   v1.2.0") {
		t.Errorf("expected version, got:\n%s", out)
	}
	if !strings.Contains(out, "ready") {
		t.Errorf("expected ready status, got:\n%s", out)
	}
	if !strings.Contains(out, "3/3") {
		t.Errorf("expected replicas 3/3, got:\n%s", out)
	}
}

func TestStatusCmd_NotReady(t *testing.T) {
	statusJSON, _ := json.Marshal(map[string]any{
		"name":      "my-app",
		"enclave":   "default",
		"ready":     false,
		"replicas":  2,
		"available": 1,
	})

	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"wf_status": func(_ map[string]any) (string, bool) {
			return string(statusJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewStatusCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.RunE(cmd, []string{"my-app"})

	w.Close()
	os.Stdout = oldStdout
	var output bytes.Buffer
	_, _ = output.ReadFrom(r)

	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "not ready") {
		t.Errorf("expected 'not ready' status, got:\n%s", out)
	}
	if !strings.Contains(out, "1/2") {
		t.Errorf("expected replicas 1/2, got:\n%s", out)
	}
}

func TestStatusCmd_DetailMode(t *testing.T) {
	statusJSON, _ := json.Marshal(map[string]any{
		"name":      "my-app",
		"enclave":   "prod",
		"ready":     true,
		"replicas":  2,
		"available": 2,
		"pods": []map[string]any{
			{"name": "my-app-abc-123", "phase": "Running", "ready": true, "nodeName": "node-1"},
			{"name": "my-app-def-456", "phase": "Running", "ready": true, "nodeName": "node-2"},
		},
		"events": []map[string]any{
			{"type": "Normal", "reason": "Scheduled", "message": "Assigned to node-1", "count": 1},
			{"type": "Warning", "reason": "BackOff", "message": "Back-off restarting failed container", "count": 3},
		},
	})

	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"wf_status": func(_ map[string]any) (string, bool) {
			return string(statusJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewStatusCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	_ = cmd.Flags().Set("detail", "true")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.RunE(cmd, []string{"my-app"})

	w.Close()
	os.Stdout = oldStdout
	var output bytes.Buffer
	_, _ = output.ReadFrom(r)

	if err != nil {
		t.Fatalf("runStatus --detail: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "Pods:") {
		t.Errorf("expected Pods section, got:\n%s", out)
	}
	if !strings.Contains(out, "my-app-abc-123") {
		t.Errorf("expected pod name, got:\n%s", out)
	}
	if !strings.Contains(out, "my-app-def-456") {
		t.Errorf("expected second pod name, got:\n%s", out)
	}
	if !strings.Contains(out, "Events:") {
		t.Errorf("expected Events section, got:\n%s", out)
	}
	if !strings.Contains(out, "BackOff") {
		t.Errorf("expected event reason, got:\n%s", out)
	}
	if !strings.Contains(out, "Warning") {
		t.Errorf("expected event type, got:\n%s", out)
	}
}

func TestStatusCmd_JSONOutput(t *testing.T) {
	statusJSON, _ := json.Marshal(map[string]any{
		"name":      "json-app",
		"enclave":   "default",
		"ready":     true,
		"replicas":  1,
		"available": 1,
	})

	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"wf_status": func(_ map[string]any) (string, bool) {
			return string(statusJSON), false
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewStatusCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.Flags().StringP("output", "o", "", "Output format")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	_ = cmd.Flags().Set("output", "json")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.RunE(cmd, []string{"json-app"})

	w.Close()
	os.Stdout = oldStdout
	var output bytes.Buffer
	_, _ = output.ReadFrom(r)

	if err != nil {
		t.Fatalf("runStatus -o json: %v", err)
	}

	var result mcp.WfStatusResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\nraw: %s", err, output.String())
	}
	if result.Name != "json-app" {
		t.Errorf("expected name=json-app, got %q", result.Name)
	}
	if !result.Ready {
		t.Error("expected ready=true")
	}
}

func TestStatusCmd_ToolError(t *testing.T) {
	srv, _ := makeMCPTestServer(t, map[string]func(args map[string]any) (string, bool){
		"wf_status": func(_ map[string]any) (string, bool) {
			return "workflow not found: missing-app", true
		},
	})
	defer closeMCPTestServer(srv)

	cleanup := setupMCPEnv(t, srv.URL+"/mcp")
	defer cleanup()

	cmd := NewStatusCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format")
	cmd.PersistentFlags().StringP("namespace", "n", "", "Namespace")
	cmd.SetContext(context.Background())

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.RunE(cmd, []string{"missing-app"})

	w.Close()
	os.Stdout = oldStdout

	if err == nil {
		t.Fatal("expected error for tool error response")
	}
	if !strings.Contains(err.Error(), "getting status") {
		t.Errorf("expected 'getting status' error, got: %v", err)
	}
}
