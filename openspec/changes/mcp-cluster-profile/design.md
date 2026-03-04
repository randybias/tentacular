# Design: Route cluster profile through MCP

## Architecture

Before:
```
tntc cluster profile --env prod
  -> buildClientForEnv(env) -> k8s.NewClientFromConfig(kubeconfig)
  -> client.Profile(ctx, namespace, label)
  -> renders markdown/json locally
```

After:
```
tntc cluster profile --env prod
  -> requireMCPClient(cmd)  (resolved per-env from Phase 2)
  -> mcpClient.ClusterProfile(ctx, namespace)
  -> renders from MCP response
```

## MCP Client Method

Add to `pkg/mcp/client.go`:

```go
type ClusterProfileResult struct {
    // Fields TBD based on MCP cluster_profile response schema
    Raw json.RawMessage
}

func (c *Client) ClusterProfile(ctx context.Context, namespace string) (*ClusterProfileResult, error) {
    params := map[string]interface{}{}
    if namespace != "" {
        params["namespace"] = namespace
    }
    return c.callTool(ctx, "cluster_profile", params)
}
```

## Profile Rendering

The current CLI renders profiles as markdown (for agent consumption) and JSON
(for programmatic use). Two options:

**Option A: Pass-through.** The MCP `cluster_profile` tool returns structured
JSON. The CLI renders markdown from the JSON fields. This requires mapping the
MCP response to the existing `k8s.ClusterProfile` struct or creating a new
rendering path.

**Option B: MCP returns pre-rendered.** The MCP tool returns both `markdown` and
`json` representations. The CLI just outputs the appropriate format. This is
simpler but puts rendering logic in the MCP server.

**Recommendation: Option A.** Keep rendering in the CLI. The MCP response is
structured data; the CLI renders it. This matches the existing pattern where the
CLI owns presentation.

## AutoProfileEnvironments

The `AutoProfileEnvironments()` function in `pkg/cli/profile.go` is called from
`tntc configure`. It profiles all configured environments using direct K8s
clients. After this change, it should:

1. Iterate over environments.
2. For each, resolve the environment's MCP client.
3. Call `ClusterProfile` via MCP.
4. Save the profile to `.tentacular/envprofiles/`.

If an environment's MCP server is not yet configured (e.g., during initial
`tntc configure`), skip that environment with a warning.

## Removed Code

- `buildClientForEnv()` in `pkg/cli/profile.go` -- no longer needed.
- Direct K8s profiling path in `runProfileForEnv()` -- replaced by MCP call.
