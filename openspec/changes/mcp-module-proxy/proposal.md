## Why

The CLI currently manages the esm.sh module proxy installation during `tntc cluster install --module-proxy`. This persistent proxy runs as a Deployment in `tentacular-system` namespace and caches npm/jsr modules for all workflow pods. As the MCP server becomes the control plane, ownership of this persistent infrastructure should move to the MCP server, which can reconcile the proxy Deployment state, monitor its health, and ensure it stays configured correctly.

## What Changes

- Add module proxy reconciliation in tentacular-mcp: the MCP server owns the lifecycle of the persistent esm.sh Deployment, Service, and NetworkPolicy in `tentacular-system`.
- Add `proxy_status` MCP tool to check module proxy health, cache status, and configuration.
- Add `proxy_reconcile` MCP tool to ensure the esm.sh Deployment matches desired state (image version, storage config, NetworkPolicy). Idempotent -- safe to call repeatedly.
- The CLI's `tntc cluster install --module-proxy` routes through the MCP server to trigger initial proxy deployment (after the MCP server itself is bootstrapped).
- The MCP server can run a background reconciliation loop to detect and fix proxy drift (e.g., scaled down, wrong image version, missing NetworkPolicy).

## Capabilities

### New Capabilities
- `proxy_status`: MCP tool to check esm.sh proxy health, readiness, cache storage, and configuration.
- `proxy_reconcile`: MCP tool to reconcile the esm.sh proxy Deployment to desired state. Creates if missing, updates if drifted.
- Background reconciliation loop in MCP server for proxy health monitoring.

### Modified Capabilities
- `tntc cluster install --module-proxy` routes through MCP `proxy_reconcile` tool (after bootstrap).

## Impact

- `../tentacular-mcp/pkg/tools/proxy.go`: New file with `handleProxyStatus()` and `handleProxyReconcile()` handlers.
- `../tentacular-mcp/pkg/tools/proxy_test.go`: Tests for proxy tools.
- `../tentacular-mcp/pkg/tools/register.go`: Add `registerProxyTools()` call.
- `../tentacular-mcp/pkg/reconciler/proxy.go`: Background reconciliation loop for proxy health (optional, can be deferred).
- `pkg/cli/cluster.go` (tentacular CLI): Route `--module-proxy` through MCP client after bootstrap.
- Depends on understanding the existing esm.sh proxy manifests in `pkg/k8s/` and config in `pkg/cli/cluster.go`.
- Logically follows Phases 1-2 (needs MCP client in CLI).
