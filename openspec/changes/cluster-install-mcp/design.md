## Context

`tntc cluster install` sets up tentacular infrastructure: gVisor runtime class, default namespaces, RBAC, network policies. Some of these operations must happen before the MCP server is running (bootstrap), while others can be delegated to the MCP server after it is deployed.

## Goals / Non-Goals

**Goals:**
- Split cluster install into bootstrap (pre-MCP) and configuration (post-MCP) phases.
- Bootstrap phase: deploy MCP server itself, create base RBAC, install gVisor if needed.
- Configuration phase: use MCP tools for namespace setup, health checks, runtime verification.
- `tntc cluster check` uses `cluster_health` MCP tool when available.

**Non-Goals:**
- Automating MCP server deployment to arbitrary cluster types.
- Multi-cluster management.
- Helm chart or operator for MCP server (manual manifests for now).

## Decisions

### Two-phase install
Phase 1 (bootstrap) uses direct K8s calls because the MCP server is not yet running. Phase 2 (configure) uses MCP tools because the server is now available. The CLI detects whether the MCP server is running and routes accordingly.

Alternative: All direct K8s -- ignores the architectural boundary.

### MCP server deployment as K8s manifests
The MCP server is deployed as a Deployment + Service using manifests bundled in the CLI (embedded via `embed.FS`). The CLI applies these directly during bootstrap.

### cluster_install MCP tool
A new MCP tool handles post-bootstrap configuration: ensuring standard namespaces exist, verifying gVisor runtime, applying default network policies. This is idempotent so it can be re-run safely.

## Risks / Trade-offs

- **Chicken-and-egg**: The MCP server must be running before MCP tools can be used. The bootstrap phase handles this, but errors during bootstrap leave a partially configured cluster.
- **Version coupling**: The CLI bundles MCP server manifests. Version mismatches between CLI and MCP server could cause issues.
- **Existing clusters**: Clusters with manually deployed MCP servers need the CLI to detect and skip the bootstrap phase.
