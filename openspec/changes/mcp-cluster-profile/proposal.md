# Route cluster profile through MCP server

## Why

The `tntc cluster profile` command currently creates a direct K8s client (via
`buildClientForEnv` in `pkg/cli/profile.go`) and calls `client.Profile()` which
uses `k8s.io/client-go` to enumerate nodes, RuntimeClasses, StorageClasses, CRDs,
quotas, etc. This is one of the remaining CLI commands that bypasses the MCP server
and talks directly to the Kubernetes API.

The MCP server already exposes a `cluster_profile` tool that returns equivalent
data. Routing `tntc cluster profile` through MCP eliminates the CLI's direct K8s
dependency for profiling and makes the architecture consistent: all cluster
operations go through MCP.

Once this change and Phase 1 (remove-cluster-install) are both complete, the CLI
will have no direct K8s API calls remaining, and `k8s.io/client-go` can
potentially be removed from `go.mod` (pending analysis of remaining transitive
usage by builder/netpol/importmap code).

## What Changes

- **Rewrite `runProfileForEnv`** in `pkg/cli/profile.go` to call
  `mcpClient.ClusterProfile()` instead of creating a direct K8s client.
- **Remove `buildClientForEnv`** from `pkg/cli/profile.go` -- no longer needed.
- **Add `ClusterProfile` method to `pkg/mcp/client.go`** that calls the MCP
  server's `cluster_profile` tool and returns the result.
- **Update profile rendering** to work with the MCP response format. The MCP
  `cluster_profile` tool returns a structured JSON object. The CLI needs to
  either render this directly or map it to the existing `k8s.ClusterProfile`
  struct.
- **Remove `AutoProfileEnvironments` direct K8s path** -- this function (called
  from `tntc configure`) also uses `buildClientForEnv`. It should be updated to
  use MCP as well, or removed if profiling on configure is no longer desired.
- **Update tests** to mock MCP calls instead of K8s client calls.

## Impact

- `tntc cluster profile` requires a running MCP server (same as all other
  commands except the now-removed `cluster install`).
- Profile output format may differ slightly from the current direct-K8s output
  since it now comes through MCP. The MCP `cluster_profile` tool may not include
  every field the direct profiler returned. Verify parity.
- `--env` flag continues to work -- it selects which MCP server to talk to
  (after Phase 2 per-env MCP config).

## Non-goals

- This change does NOT modify the MCP server's `cluster_profile` tool itself.
  If the MCP tool needs updates for field parity, that is tracked in the
  `cluster-profile-api` change in the tentacular-mcp repo.
