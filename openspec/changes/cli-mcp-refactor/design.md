## Context

The CLI commands in `pkg/cli/` currently instantiate `k8s.Client` directly and call its methods. After Phase 1 provides `pkg/mcp.Client`, commands need to be refactored to route cluster operations through MCP. The build pipeline (validate, compile, containerize) remains CLI-side.

## Goals / Non-Goals

**Goals:**
- Route deploy (apply step), undeploy, status, list, logs, run through MCP client.
- Keep build pipeline (init, validate, test, build, dev) as direct CLI operations.
- MCP server is mandatory for cluster operations (ADR-3). Clear error messages when not configured or unreachable.
- Minimal API surface change for end users -- same CLI flags and output.

**Non-Goals:**
- Changing CLI output format or user-facing behavior.
- Moving build/compile operations to MCP server.
- Supporting multiple MCP server endpoints (single server per config for v1).

## Decisions

### MCP is mandatory (ADR-3)
If the MCP server URL is not configured, commands that require cluster access (deploy, undeploy, status, list, logs, run) fail with a clear error: "MCP server not configured. Run `tntc cluster install` or set `--mcp-server`." If the server is unreachable, commands fail with a connectivity error. No direct K8s fallback -- the MCP server is the single control plane for all cluster operations.

Rationale: A fallback path would create two code paths to maintain, undermine the architectural boundary, and allow the CLI to bypass MCP-enforced policies (RBAC, audit, guard checks).

### Deploy splits into build + apply-via-MCP
`tntc deploy` currently does: validate -> build container -> generate manifests -> apply to K8s. The refactored version does: validate -> build container -> generate manifests -> send manifests to MCP `wf_apply`. The first three steps stay in the CLI; only the apply step routes through MCP.

Alternative: Send source code to MCP and have it build -- violates the "CLI is data plane" principle.

### MCP client initialization in PersistentPreRun
The `mcp.Client` is created once in a Cobra `PersistentPreRun` hook using the configured server URL and Bearer token, then stored in the command context. Commands that need cluster access retrieve it from context and fail early if the client is nil (not configured) or unhealthy (unreachable).

## Risks / Trade-offs

- **Hard dependency on MCP**: Users must have a running MCP server for any cluster operation. Mitigated by `tntc cluster install` automating the setup, and clear error messages guiding users to configure it.
- **Latency**: HTTP round-trips to the in-cluster MCP server add latency. Acceptable for CLI commands that already take seconds for K8s operations.
- **Error mapping**: MCP tool errors need to be mapped to user-friendly CLI error messages. May lose some K8s-specific error detail.
