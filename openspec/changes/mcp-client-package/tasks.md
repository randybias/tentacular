## 1. Core Client

- [ ] 1.1 Create `pkg/mcp/client.go` with `Client` struct wrapping MCP SDK Streamable HTTP client transport.
- [ ] 1.2 Implement `NewClient(ctx, serverURL, token)`: configure HTTP transport with Bearer token auth header, initialize MCP client session via `initialize` handshake.
- [ ] 1.3 Implement `Close()`: graceful session shutdown.
- [ ] 1.4 Implement `Healthy()`: GET `/healthz` endpoint (unauthenticated) to verify server reachability.
- [ ] 1.5 Add server URL resolution: `--mcp-server` flag > `TNTC_MCP_SERVER` env > `~/.tentacular/config.yaml` `mcp.server`.
- [ ] 1.6 Add token resolution: `--mcp-token` flag > `TNTC_MCP_TOKEN` env > `~/.tentacular/config.yaml` `mcp.token`.

## 2. Typed Tool Wrappers

- [ ] 2.1 Create `pkg/mcp/tools.go` with params and result structs mirroring tentacular-mcp tool schemas.
- [ ] 2.2 Implement namespace tools: `NsList()`, `NsEnsure()`.
- [ ] 2.3 Implement workflow tools: `WfPods()`, `WfLogs()`, `WfEvents()`, `WfTrigger()`.
- [ ] 2.4 Implement deploy tools: `WfApply()`, `WfRemove()`.
- [ ] 2.5 Implement discovery tools: `WfList()`, `WfDescribe()`.
- [ ] 2.6 Implement cluster tools: `ClusterHealth()`, `ClusterAudit()`, `GVisorStatus()`.
- [ ] 2.7 Implement credential tools: `CredentialValidate()`.

## 3. Tests

- [ ] 3.1 Create `pkg/mcp/client_test.go` with mock HTTP MCP server (httptest.Server with `mcp.NewStreamableHTTPHandler`).
- [ ] 3.2 Test connection lifecycle: connect, health check, close.
- [ ] 3.3 Test Bearer token auth: valid token succeeds, missing/invalid token returns 401.
- [ ] 3.4 Test tool call wrappers with mock responses.
- [ ] 3.5 Test error handling: server unreachable, auth failure, tool call error.
- [ ] 3.6 Run `go test ./pkg/mcp/...` -- all pass.

## 4. Integration

- [ ] 4.1 Update `go.mod` with MCP SDK dependency.
- [ ] 4.2 Run `go test ./...` -- all existing tests still pass.
