# Design: Remove cluster install subcommand

## Removal Scope

### Files to delete

- `pkg/k8s/mcp_deploy.go` -- all MCP manifest generation functions
- `pkg/k8s/mcp_deploy_test.go` -- tests
- `pkg/k8s/mcp_token.go` -- `GenerateMCPToken()` function

### Functions to remove from `pkg/cli/cluster.go`

- `runClusterInstall()` -- the install command handler
- `waitForMCPReady()` -- polls /healthz during install
- `saveMCPToken()` -- writes token to `~/.tentacular/mcp-token`
- The `install` cobra.Command registration in `NewClusterCmd()`
- All install-related flag definitions

### What remains in `pkg/cli/cluster.go`

```go
func NewClusterCmd() *cobra.Command {
    cluster := &cobra.Command{
        Use:   "cluster",
        Short: "Cluster management commands",
    }
    cluster.AddCommand(/* check */)
    cluster.AddCommand(NewProfileCmd())
    return cluster
}
```

The `check` subcommand and `runClusterCheck` stay unchanged -- they already
route through MCP.

## Migration Path

Users running `tntc cluster install` will get a clear error:

```
Error: unknown command "install" for "tntc cluster"

Did you mean this?
  check

Run 'tntc cluster --help' for usage.
```

Documentation should point users to:
```bash
helm install tentacular-mcp oci://ghcr.io/randybias/charts/tentacular-mcp \
  --namespace tentacular-system --create-namespace
```

## Dependency Analysis

After removing `mcp_deploy.go` and `mcp_token.go`, check if the following
imports are still needed anywhere in the CLI:

- `k8s.io/client-go/kubernetes` -- used by `Client.clientset` in `client.go`
- `k8s.io/client-go/dynamic` -- used by `Client.dynamic` in `client.go`
- `k8s.io/client-go/rest` -- used by `Client.config` in `client.go`
- `k8s.io/client-go/tools/clientcmd` -- used by `loadConfig()` in `client.go`

These are still needed for `pkg/k8s/client.go` which is used by profile, kind
detection, and other paths. Full removal of client-go depends on Phase 3
(routing profile through MCP) and Phase 7 (code cleanup).
