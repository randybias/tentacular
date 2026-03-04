# Per-environment MCP configuration with DefaultEnv

## Why

Today the CLI has a single global MCP config (`mcp.endpoint` and
`mcp.token_path` in `~/.tentacular/config.yaml`). All environments share the
same MCP server endpoint. This is incorrect for multi-cluster setups where each
environment (dev, staging, prod) has its own cluster with its own MCP server.

The existing `environments` map in `TentacularConfig` already supports per-env
`kubeconfig`, `context`, `namespace`, `runtime_class`, `image`, and
`config_overrides`. But it has no fields for MCP connection settings.

Additionally, there is no concept of a default environment. Users must pass
`--env` on every command or set `TENTACULAR_ENV`. A `default_env` field would
reduce friction for users who primarily work against one cluster.

## What Changes

- **Add `MCPEndpoint` and `MCPTokenPath` fields to `EnvironmentConfig`** in
  `pkg/cli/environment.go`. These are `yaml:"mcp_endpoint"` and
  `yaml:"mcp_token_path"`.
- **Add `DefaultEnv` field to `TentacularConfig`** in `pkg/cli/config.go`.
  This is `yaml:"default_env,omitempty"`. When set, commands that don't receive
  `--env` and don't have `TENTACULAR_ENV` set will use this environment.
- **Update `requireMCPClient`** (in the CLI helpers) to resolve MCP connection
  from the active environment config rather than the global `mcp` block. The
  resolution cascade becomes: CLI flags > env-specific MCP config > global MCP
  config > error.
- **Update `ResolveEnvironment`** to check `default_env` when no env name is
  provided and `TENTACULAR_ENV` is unset.
- **Update `mergeConfig`** to merge the new `DefaultEnv` field.
- **Update `tntc configure`** to prompt for default environment and per-env
  MCP settings.

## Config Example

```yaml
default_env: dev
environments:
  dev:
    kubeconfig: ~/dev-secrets/kubeconfigs/dev.kubeconfig
    namespace: tentacular-dev-wf
    mcp_endpoint: http://tentacular-mcp.tentacular-system.svc.cluster.local:8080
    mcp_token_path: ~/.tentacular/tokens/dev-mcp-token
  prod:
    kubeconfig: ~/dev-secrets/kubeconfigs/prod.kubeconfig
    namespace: tentacular-prod-wf
    mcp_endpoint: http://tentacular-mcp.tentacular-system.svc.cluster.local:8080
    mcp_token_path: ~/.tentacular/tokens/prod-mcp-token
```

## Impact

- Backwards compatible: existing configs with a global `mcp` block continue to
  work. Per-env settings override global when present.
- `tntc configure` will offer to set up per-env MCP configs.
- All commands that call `requireMCPClient` automatically benefit from per-env
  resolution.

## Non-goals

- This change does NOT add MCP server discovery or auto-detection.
- This change does NOT modify the MCP server itself.
