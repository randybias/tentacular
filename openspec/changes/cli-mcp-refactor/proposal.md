## Why

CLI commands (`deploy`, `undeploy`, `status`, `list`, `logs`, `run`) currently call `pkg/k8s/` directly to interact with the cluster. With the MCP server as the control plane, these commands should route through `pkg/mcp/Client` instead. This enforces the architectural boundary: CLI builds and ships, MCP server operates and enforces.

## What Changes

- Refactor `pkg/cli/deploy.go` to use `mcp.Client.WfApply()` instead of `k8s.Client.Apply()` for the K8s resource application step. The build step (container image build, manifest generation) remains in the CLI.
- Refactor `pkg/cli/undeploy.go` to use `mcp.Client.WfRemove()`.
- Refactor `pkg/cli/status.go` to use `mcp.Client.WfDescribe()`.
- Refactor `pkg/cli/list.go` to use `mcp.Client.WfList()`.
- Refactor `pkg/cli/logs.go` to use `mcp.Client.WfLogs()`.
- Refactor `pkg/cli/run.go` to use `mcp.Client.WfRun()` (after Phase 3 adds the tool).
- Add `--mcp-server` and `--mcp-token` flags (or config options) to specify the MCP server endpoint and auth token.
- MCP server is mandatory for cluster operations (ADR-3). If not configured or unreachable, commands that require cluster access fail with a clear error directing the user to run `tntc cluster install` or configure the MCP endpoint.

## Capabilities

### New Capabilities
- CLI commands route all cluster operations through MCP client (mandatory).
- `--mcp-server` and `--mcp-token` flags / config for specifying MCP server URL and Bearer token.

### Modified Capabilities
- `deploy`, `undeploy`, `status`, `list`, `logs`, `run` commands gain MCP-routed execution paths.

## Impact

- `pkg/cli/deploy.go`: Replace direct `k8s.Client` calls with `mcp.Client.WfApply()` for the apply step.
- `pkg/cli/undeploy.go`: Replace with `mcp.Client.WfRemove()`.
- `pkg/cli/status.go`: Replace with `mcp.Client.WfDescribe()`.
- `pkg/cli/list.go`: Replace with `mcp.Client.WfList()`.
- `pkg/cli/logs.go`: Replace with `mcp.Client.WfLogs()`.
- `pkg/cli/run.go`: Replace with `mcp.Client.WfRun()`.
- `pkg/cli/config.go` or `pkg/cli/environment.go`: Add MCP server URL and token config.
- Depends on Phase 1 (mcp-client-package). Partially depends on Phase 3 (wf_run tool for `run` command).
