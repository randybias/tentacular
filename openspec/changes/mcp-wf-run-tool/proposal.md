## Why

The CLI `tntc run` command currently triggers workflow execution by creating a K8s Job directly via `pkg/k8s/`. For the MCP server to be the single control plane for cluster mutations, it needs a `wf_run` tool that triggers workflow execution. This also enables AI assistants to trigger workflows directly through MCP.

## What Changes

- Add `wf_run` MCP tool in `../tentacular-mcp/pkg/tools/` that creates a trigger Job in the workflow's namespace.
- The tool accepts workflow name, namespace, and optional input parameters.
- It creates a Job that runs the workflow's runner image with the provided input, similar to what `tntc run` does today.
- Returns Job name, status, and a flag indicating whether to poll for completion.
- Follows the existing MCP tool handler pattern (params struct, result struct, guard checks via `pkg/guard`).

## Capabilities

### New Capabilities
- `wf_run`: MCP tool to trigger workflow execution by creating a K8s Job in the target namespace.

### Modified Capabilities
<!-- None -->

## Impact

- `../tentacular-mcp/pkg/tools/run.go`: New file with `handleWfRun()`, params/result structs, Job creation logic.
- `../tentacular-mcp/pkg/tools/run_test.go`: Tests for wf_run (success, invalid namespace, missing workflow).
- `../tentacular-mcp/pkg/tools/register.go`: Add `registerRunTools()` call.
- `../tentacular-mcp/pkg/k8s/client.go`: May need a `CreateJob()` helper if not already present.
- No strict dependency on other phases, but Phase 2 will consume this tool.
