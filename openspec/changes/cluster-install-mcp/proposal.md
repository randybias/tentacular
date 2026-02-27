## Why

`tntc cluster install` currently sets up tentacular infrastructure on a K8s cluster (gVisor runtime, namespaces, RBAC, etc.) by directly calling `pkg/k8s/`. With the MCP server as the control plane, cluster setup should be orchestrated through MCP tools. This also enables AI assistants to manage cluster setup via MCP.

## What Changes

- Refactor `pkg/cli/cluster.go` to use MCP client for cluster operations where applicable.
- Use existing MCP tools: `ns_ensure` for namespace creation, `cluster_health` for health checks, `gvisor_status` for runtime verification.
- Operations that are truly one-time bootstrap (installing the MCP server itself, initial RBAC) remain as direct K8s calls since the MCP server may not be running yet.
- Add a `cluster_install` MCP tool for post-bootstrap cluster setup (gVisor profile, network policies, default namespaces).

## Capabilities

### New Capabilities
- `cluster_install`: MCP tool for post-bootstrap cluster configuration.
- CLI cluster commands route through MCP for operations after the MCP server is running.

### Modified Capabilities
- `tntc cluster install` uses MCP for post-bootstrap steps.
- `tntc cluster check` routes through `cluster_health` MCP tool.

## Impact

- `pkg/cli/cluster.go`: Refactor to use `mcp.Client` for post-bootstrap operations.
- `../tentacular-mcp/pkg/tools/clusterops.go`: Add `cluster_install` handler for post-bootstrap setup.
- Depends on Phase 1 (mcp-client-package) and Phase 2 (cli-mcp-refactor pattern).
