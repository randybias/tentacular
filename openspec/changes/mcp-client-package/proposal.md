## Why

The tentacular CLI currently talks directly to Kubernetes via `pkg/k8s/`. As the MCP server (tentacular-mcp) becomes the control plane for cluster operations, the CLI needs a client package to communicate with the MCP server over HTTP. This establishes the data plane (CLI) to control plane (MCP) boundary. The MCP server runs in-cluster as a K8s Deployment with its own ServiceAccount and RBAC, so the CLI must connect over HTTP with Bearer token auth rather than requiring local K8s access.

## What Changes

- Add new `pkg/mcp/` package in the tentacular CLI repo with a `Client` struct that wraps HTTP-based MCP Streamable HTTP transport.
- The client connects to the tentacular-mcp server via HTTP (Streamable HTTP transport as implemented in `mcp.NewStreamableHTTPHandler`). The server runs in-cluster at a known endpoint (e.g., `http://<host>:8080/mcp`).
- Authentication: Bearer token passed in the `Authorization` header, matching the `pkg/auth.Middleware` in tentacular-mcp.
- Provides typed helper methods that map to MCP tool calls: `WfApply()`, `WfRemove()`, `WfList()`, `WfDescribe()`, `WfPods()`, `WfLogs()`, `WfEvents()`, `WfTrigger()`, `WfRun()`, `NsList()`, `NsEnsure()`, `ClusterHealth()`, `ClusterAudit()`.
- Connection lifecycle: `NewClient(ctx, serverURL, token)` establishes the HTTP session, `Close()` tears it down.
- All methods return typed Go structs (mirroring the MCP tool result schemas) for compile-time safety.

## Capabilities

### New Capabilities
- `pkg/mcp.Client`: MCP client that connects to tentacular-mcp via HTTP Streamable HTTP transport with Bearer token auth.
- Typed method wrappers for every MCP tool registered in tentacular-mcp.
- Connection health check via `/healthz` endpoint (unauthenticated).

### Modified Capabilities
<!-- None -->

## Impact

- `pkg/mcp/client.go`: Core client struct, HTTP transport setup, Bearer token auth, connection management.
- `pkg/mcp/tools.go`: Typed method wrappers for each MCP tool (params/result structs, CallTool invocations).
- `pkg/mcp/client_test.go`: Unit tests with mock HTTP MCP server.
- `go.mod`: Add dependency on `github.com/modelcontextprotocol/go-sdk`.
- No dependencies on other phases. This is the foundation.
