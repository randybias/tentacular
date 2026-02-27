## Context

The tentacular CLI needs to communicate with the tentacular-mcp server, which runs in-cluster as a K8s Deployment on port 8080. The server uses `mcp.NewStreamableHTTPHandler` for MCP protocol and `pkg/auth.Middleware` for Bearer token authentication (token loaded from `/etc/tentacular-mcp/token`). The `/healthz` endpoint bypasses auth. The MCP Go SDK provides client-side Streamable HTTP transport support.

## Goals / Non-Goals

**Goals:**
- Provide a typed Go client wrapping MCP tool calls for compile-time safety.
- Use HTTP Streamable HTTP transport: CLI connects to the in-cluster MCP server endpoint.
- Bearer token authentication matching the existing `pkg/auth.Middleware` pattern.
- Mirror all tools registered in tentacular-mcp's `RegisterAll()`.
- Clean connection lifecycle: connect, call tools, close.

**Non-Goals:**
- Stdio transport (the MCP server runs in-cluster, not as a local subprocess).
- mTLS or certificate-based auth (Bearer token is sufficient for v1).
- Connection pooling or multiplexing (one client per CLI invocation).

## Decisions

### HTTP Streamable HTTP transport
The CLI connects to the MCP server over HTTP using the MCP SDK's Streamable HTTP client transport. The server is reachable via `kubectl port-forward`, an Ingress, or a LoadBalancer Service. The CLI does not need a kubeconfig -- all K8s operations are delegated to the MCP server which has its own ServiceAccount and RBAC.

Alternative: Stdio transport (spawn MCP binary locally) -- requires the MCP binary on the developer's machine AND a kubeconfig, which defeats the purpose of centralizing K8s operations in the MCP server.

### Bearer token authentication
The CLI sends a Bearer token in the `Authorization` header on every request. The token is configured via: (1) `--mcp-token` flag, (2) `TNTC_MCP_TOKEN` env var, (3) `~/.tentacular/config.yaml` under `mcp.token`. This matches the server-side `pkg/auth.Middleware` which validates Bearer tokens.

Alternative: Kubeconfig-based auth -- ties the CLI to K8s credentials, doesn't enforce the control plane boundary.

### Server URL discovery
The client accepts an explicit server URL. The CLI resolves this from: (1) `--mcp-server` flag, (2) `TNTC_MCP_SERVER` env var, (3) `~/.tentacular/config.yaml` under `mcp.server`. No default URL -- must be explicitly configured after `tntc cluster install`.

### Typed method wrappers
Each MCP tool gets a Go method that marshals params and unmarshals results into typed structs. This catches schema mismatches at compile time rather than runtime.

Alternative: Generic `CallTool(name, params)` -- no type safety, error-prone.

### Health check via /healthz
The client provides a `Healthy()` method that hits the unauthenticated `/healthz` endpoint to verify server reachability before attempting MCP tool calls.

## Risks / Trade-offs

- **Network reachability**: The CLI must be able to reach the MCP server endpoint. For development clusters, `kubectl port-forward` or a NodePort Service is needed. Mitigated by documenting setup in `tntc cluster install` output.
- **Token management**: Users must obtain and configure the Bearer token. Mitigated by `tntc cluster install` outputting the token and writing it to config.
- **Schema drift**: If MCP tool schemas change in tentacular-mcp but the CLI client is not updated, calls will fail. Mitigated by version checking at connection time via MCP `initialize` handshake.
- **Latency**: HTTP round-trips add latency compared to direct K8s calls. Acceptable for CLI commands that already take seconds for K8s operations.
