## 1. MCP Client Integration

- [ ] 1.1 Add MCP client initialization in Cobra `PersistentPreRun` hook in `cmd/tntc/main.go`.
- [ ] 1.2 Add `--mcp-server` and `--mcp-token` persistent flags, plus `TNTC_MCP_SERVER` / `TNTC_MCP_TOKEN` env var support.
- [ ] 1.3 Store `mcp.Client` in command context. Nil signals "not configured" for early error in cluster commands.
- [ ] 1.4 Add helper function in `pkg/cli/` to retrieve MCP client from context, returning a clear error if not configured or unreachable.

## 2. Command Refactoring

- [ ] 2.1 Refactor `pkg/cli/deploy.go`: split into build phase (keep) and apply phase (route via `mcp.Client.WfApply()`, fail if MCP unavailable).
- [ ] 2.2 Refactor `pkg/cli/undeploy.go`: route via `mcp.Client.WfRemove()`, fail if MCP unavailable.
- [ ] 2.3 Refactor `pkg/cli/status.go`: route via `mcp.Client.WfDescribe()`, fail if MCP unavailable.
- [ ] 2.4 Refactor `pkg/cli/list.go`: route via `mcp.Client.WfList()`, fail if MCP unavailable.
- [ ] 2.5 Refactor `pkg/cli/logs.go`: route via `mcp.Client.WfLogs()`, fail if MCP unavailable.
- [ ] 2.6 Refactor `pkg/cli/run.go`: route via `mcp.Client.WfRun()`, fail if MCP unavailable (depends on Phase 3).

## 3. Config and Environment

- [ ] 3.1 Update `pkg/cli/config.go` or `pkg/cli/environment.go` to support `mcp.server` and `mcp.token` in `~/.tentacular/config.yaml`.
- [ ] 3.2 Add MCP connection status to `tntc cluster check` output.

## 4. Tests

- [ ] 4.1 Update `pkg/cli/deploy_test.go` to test MCP-routed path.
- [ ] 4.2 Test error handling when MCP server is not configured or unreachable (clear error message, no fallback).
- [ ] 4.3 Add tests for MCP client context injection.
- [ ] 4.4 Run `go test ./pkg/cli/...` -- all pass.
- [ ] 4.5 Run `go test ./...` -- all pass.
