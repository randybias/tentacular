# Remove cluster install subcommand and direct K8s access from CLI

## Why

The `tntc cluster install` command is the only CLI command that talks directly to
the Kubernetes API. It bootstraps the MCP server by generating manifests (via
`pkg/k8s/mcp_deploy.go`) and applying them with a local K8s client. This creates
several problems:

**Direct K8s dependency in the CLI.** The CLI imports `k8s.io/client-go` and
`k8s.io/apimachinery` solely for the `cluster install` path. These dependencies
add significant binary size and compilation time. Every other CLI command routes
through the MCP server via HTTP.

**Duplicated deployment logic.** The MCP server manifests (ServiceAccount,
ClusterRole, ClusterRoleBinding, Secret, Deployment, Service) are generated as
Go string templates in `pkg/k8s/mcp_deploy.go`. This duplicates what a Helm
chart already provides. The tentacular-mcp repo has a Helm chart at `charts/`
that is the canonical installation method.

**Bootstrap is a one-time operation.** Users install the MCP server once per
cluster. This does not justify maintaining a full K8s client in the CLI binary.
The Helm chart (or `kubectl apply`) is a more standard and maintainable approach.

**Token management in CLI is fragile.** The CLI generates a bearer token locally
(`pkg/k8s/mcp_token.go`), writes it to `~/.tentacular/mcp-token`, and embeds it
in a K8s Secret. This bypasses the MCP server's own credential management and
creates a split source of truth.

## What Changes

- **Remove `cluster install` subcommand** from `pkg/cli/cluster.go`. The
  `NewClusterCmd()` function will only register `check` and `profile`.
- **Remove `runClusterInstall` function** and its helpers (`waitForMCPReady`,
  `saveMCPToken`) from `pkg/cli/cluster.go`.
- **Remove `pkg/k8s/mcp_deploy.go`** -- the MCP server manifest generation
  functions (`GenerateMCPServerManifests`, `mcpServiceAccount`, `mcpClusterRole`,
  etc.) are no longer needed in the CLI.
- **Remove `pkg/k8s/mcp_token.go`** -- the token generation function is no
  longer needed in the CLI.
- **Remove `pkg/k8s/mcp_deploy_test.go`** -- tests for the removed code.
- **Update `cmd/tntc/main.go`** if it references any removed symbols.
- **Update `go.mod`** -- run `go mod tidy` to drop unused K8s client-go
  dependencies (if they are no longer transitively required).

## Impact

- Users install via `helm install tentacular-mcp charts/tentacular-mcp` (or the
  published Helm chart) instead of `tntc cluster install`.
- The `tntc cluster check` and `tntc cluster profile` commands are unaffected.
- The CLI binary will be smaller after removing unused K8s client-go deps (if
  they are fully removable -- `cluster profile` still uses them today but will
  be routed through MCP in Phase 3).
- Existing clusters with MCP servers installed via `tntc cluster install` are
  unaffected; the running MCP server and its manifests remain.

## Non-goals

- This change does NOT remove `pkg/k8s/` entirely. Other code paths (profile,
  kind detection, netpol generation, import maps) still use it.
- This change does NOT modify the MCP server or its Helm chart.
