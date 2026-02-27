## Why

After Phases 1-5, the CLI's `pkg/k8s/` package will have significant dead code -- functions that are now handled by the MCP server. The package should be slimmed down to only contain: (a) bootstrap operations needed before the MCP server is running, (b) build-time operations that are purely CLI-side (manifest generation, importmap resolution). This reduces maintenance burden and makes the architectural boundary explicit.

## What Changes

- Audit `pkg/k8s/client.go` (24.9K) to identify functions that are now handled by MCP tools.
- Remove functions that are fully replaced: direct Apply, Delete, status queries, log retrieval, event listing.
- Keep functions needed for bootstrap: `NewClient()`, `loadConfig()`, pre-MCP cluster setup.
- Keep build-time functions: importmap resolution, manifest generation helpers.
- Keep `pkg/k8s/profile.go` and `pkg/k8s/preflight.go` if they have CLI-only concerns.
- Update any remaining callers in `pkg/cli/` to use `pkg/mcp/Client` instead.

## Capabilities

### New Capabilities
<!-- None -- this is a removal/cleanup phase -->

### Modified Capabilities
- `pkg/k8s/` reduced to bootstrap and build-time operations only.

## Impact

- `pkg/k8s/client.go`: Remove MCP-replaced functions (Apply, Delete, GetPods, GetLogs, GetEvents, TriggerRun, etc.).
- `pkg/k8s/netpol.go`: Evaluate if NetworkPolicy generation is CLI-side (build) or MCP-side (deploy). Keep if build-time.
- `pkg/k8s/profile.go`: Keep if cluster profile management is CLI-side bootstrap.
- `pkg/k8s/importmap.go`: Keep -- build-time importmap resolution is CLI-side.
- `pkg/k8s/preflight.go`: Evaluate -- pre-deploy checks may move to MCP.
- Depends on Phases 1-5 being complete. This is a cleanup phase.
