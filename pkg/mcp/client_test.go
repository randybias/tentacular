package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// makeTestServer creates an httptest.Server backed by a real MCP SDK server with
// the given tool handlers. Each handler receives the raw arguments JSON and returns
// (JSON text content, isError). The test client connects with a 5s timeout.
func makeTestServer(t *testing.T, tools map[string]func(args map[string]any) (string, bool)) (*httptest.Server, *Client) {
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
				// Parse raw arguments into map
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := httptest.NewServer(mux)
	client := NewClient(Config{
		Endpoint: srv.URL,
		Token:    "test-token",
		Timeout:  5 * time.Second,
	})

	return srv, client
}

func TestCallTool_Success(t *testing.T) {
	srv, client := makeTestServer(t, map[string]func(map[string]any) (string, bool){
		"test_tool": func(args map[string]any) (string, bool) {
			return `{"status":"ok"}`, false
		},
	})
	defer srv.Close()
	defer client.Close()

	raw, err := client.CallTool(context.Background(), "test_tool", map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestCallTool_ToolError(t *testing.T) {
	srv, client := makeTestServer(t, map[string]func(map[string]any) (string, bool){
		"wf_status": func(args map[string]any) (string, bool) {
			return "workflow not found", true
		},
	})
	defer srv.Close()
	defer client.Close()

	_, err := client.CallTool(context.Background(), "wf_status", nil)
	if !IsToolError(err) {
		t.Errorf("expected tool error, got: %v", err)
	}
}

func TestCallTool_ServerUnavailable(t *testing.T) {
	client := NewClient(Config{
		Endpoint: "http://127.0.0.1:1", // nothing listening
		Timeout:  1 * time.Second,
	})

	_, err := client.CallTool(context.Background(), "test_tool", nil)
	if !IsServerUnavailable(err) {
		t.Errorf("expected server unavailable error, got: %v", err)
	}
}

func TestCallTool_BearerToken(t *testing.T) {
	var receivedAuth string
	// Create a server that captures the auth header
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if receivedAuth == "" {
			receivedAuth = r.Header.Get("Authorization")
		}
		// Return 400 to fail fast — we only need to capture the header
		w.WriteHeader(http.StatusBadRequest)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(Config{
		Endpoint: srv.URL,
		Token:    "test-token",
		Timeout:  2 * time.Second,
	})
	// This will fail because the server doesn't speak MCP, but the header is captured
	client.CallTool(context.Background(), "test_tool", nil)

	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Bearer test-token, got %q", receivedAuth)
	}
}

func TestPing_Success(t *testing.T) {
	srv, client := makeTestServer(t, nil)
	defer srv.Close()
	defer client.Close()

	if err := client.Ping(context.Background()); err != nil {
		t.Errorf("unexpected ping error: %v", err)
	}
}

func TestPing_Unavailable(t *testing.T) {
	client := NewClient(Config{
		Endpoint: "http://127.0.0.1:1",
		Timeout:  1 * time.Second,
	})
	if err := client.Ping(context.Background()); !IsServerUnavailable(err) {
		t.Errorf("expected server unavailable, got: %v", err)
	}
}

func TestPing_Non200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(Config{Endpoint: srv.URL, Timeout: 2 * time.Second})
	err := client.Ping(context.Background())
	if !IsServerUnavailable(err) {
		t.Errorf("expected server unavailable for non-200 healthz, got: %v", err)
	}
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	client := NewClient(Config{Endpoint: "http://mcp:8080"})
	if client.httpClient == nil {
		t.Error("expected non-nil http client")
	}
	if client.httpClient.Timeout != defaultTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultTimeout, client.httpClient.Timeout)
	}
}

func TestCallTool_MultipleCalls(t *testing.T) {
	callCount := 0
	srv, client := makeTestServer(t, map[string]func(map[string]any) (string, bool){
		"counter": func(args map[string]any) (string, bool) {
			callCount++
			return `{"count":1}`, false
		},
	})
	defer srv.Close()
	defer client.Close()

	for i := 0; i < 3; i++ {
		_, err := client.CallTool(context.Background(), "counter", nil)
		if err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}

	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestCallTool_ArgsPassthrough(t *testing.T) {
	var receivedNS string
	srv, client := makeTestServer(t, map[string]func(map[string]any) (string, bool){
		"wf_list": func(args map[string]any) (string, bool) {
			if ns, ok := args["namespace"]; ok {
				receivedNS, _ = ns.(string)
			}
			return `[]`, false
		},
	})
	defer srv.Close()
	defer client.Close()

	_, err := client.CallTool(context.Background(), "wf_list", map[string]any{"namespace": "my-ns"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedNS != "my-ns" {
		t.Errorf("expected namespace=my-ns, got %q", receivedNS)
	}
}

func TestClose_Idempotent(t *testing.T) {
	srv, client := makeTestServer(t, map[string]func(map[string]any) (string, bool){
		"test": func(args map[string]any) (string, bool) {
			return `{}`, false
		},
	})
	defer srv.Close()

	// Connect by making a call
	_, err := client.CallTool(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}

	// Close multiple times should not panic
	if err := client.Close(); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}
