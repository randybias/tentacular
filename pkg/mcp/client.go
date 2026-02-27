// Package mcp provides a client for communicating with the tentacular-mcp server
// using the official MCP Go SDK (github.com/modelcontextprotocol/go-sdk).
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config holds MCP connection settings.
type Config struct {
	Endpoint  string        // e.g. "http://tentacular-mcp.tentacular-system.svc.cluster.local:8080"
	Token     string        // Bearer token (resolved from TokenPath if empty)
	TokenPath string        // Path to token file (used when Token is empty)
	Timeout   time.Duration // Per-request timeout (default: 30s)
}

// Client communicates with the tentacular-mcp server via the MCP protocol.
type Client struct {
	baseURL    string
	token      string
	timeout    time.Duration
	httpClient *http.Client // used for Ping (healthz) only

	mu      sync.Mutex
	session *mcpsdk.ClientSession
}

// NewClient creates an MCP client from config. The MCP session is established
// lazily on the first CallTool invocation.
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL: cfg.Endpoint,
		token:   cfg.Token,
		timeout: timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// connect establishes an MCP session if not already connected.
func (c *Client) connect(ctx context.Context) (*mcpsdk.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		return c.session, nil
	}

	transport := &mcpsdk.StreamableClientTransport{
		Endpoint: c.baseURL + "/mcp",
		HTTPClient: &http.Client{
			Timeout:   c.timeout,
			Transport: &bearerTransport{token: c.token, base: http.DefaultTransport},
		},
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "tntc",
		Version: "0.1.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, mapConnectError(c.baseURL, err)
	}

	c.session = session
	return session, nil
}

// bearerTransport injects the Authorization header into every HTTP request.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// CallTool invokes a named MCP tool with typed params and returns the raw JSON result.
// The MCP session is established on first call.
func (c *Client) CallTool(ctx context.Context, tool string, params interface{}) (json.RawMessage, error) {
	session, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}

	args, err := toArgsMap(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling tool arguments: %w", err)
	}

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      tool,
		Arguments: args,
	})
	if err != nil {
		return nil, mapCallError(c.baseURL, err)
	}

	if result.IsError {
		msg := extractTextContent(result.Content)
		return nil, &ToolError{Tool: tool, Message: msg}
	}

	text := extractTextContent(result.Content)
	if text == "" {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(text), nil
}

// Ping checks if the MCP server is reachable by calling GET /healthz.
// This does not use the MCP protocol — it's a simple HTTP health check.
func (c *Client) Ping(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("building healthz request: %w", err)
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &ServerUnavailableError{Endpoint: c.baseURL, Cause: err}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return &ServerUnavailableError{
			Endpoint: c.baseURL,
			Cause:    fmt.Errorf("healthz returned HTTP %d", httpResp.StatusCode),
		}
	}
	return nil
}

// Close terminates the MCP session. Safe to call multiple times or on a nil session.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		err := c.session.Close()
		c.session = nil
		return err
	}
	return nil
}

// toArgsMap converts a typed struct or map to map[string]any via JSON round-trip.
// The MCP SDK requires map[string]any for tool arguments.
func toArgsMap(params interface{}) (map[string]any, error) {
	if params == nil {
		return map[string]any{}, nil
	}
	if m, ok := params.(map[string]any); ok {
		return m, nil
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// extractTextContent returns the first text content string from MCP content.
func extractTextContent(content []mcpsdk.Content) string {
	for _, c := range content {
		if text, ok := c.(*mcpsdk.TextContent); ok {
			return text.Text
		}
	}
	return ""
}

// mapConnectError maps errors from client.Connect to our error types.
// The auth middleware returns 401 at the HTTP level, which causes the
// MCP initialize handshake to fail with a transport error.
func mapConnectError(endpoint string, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") {
		return &Error{Code: 401, Message: "unauthorized: invalid or missing token"}
	}
	if strings.Contains(msg, "403") || strings.Contains(msg, "Forbidden") {
		return &Error{Code: 403, Message: "forbidden: namespace guard rejected request"}
	}
	return &ServerUnavailableError{Endpoint: endpoint, Cause: err}
}

// mapCallError maps errors from session.CallTool to our error types.
func mapCallError(endpoint string, err error) error {
	msg := err.Error()
	if strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") {
		return &Error{Code: 401, Message: "unauthorized: invalid or missing token"}
	}
	if strings.Contains(msg, "403") || strings.Contains(msg, "Forbidden") {
		return &Error{Code: 403, Message: "forbidden: namespace guard rejected request"}
	}
	return &Error{Code: -1, Message: msg}
}
