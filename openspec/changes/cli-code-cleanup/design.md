# Design: Remove dead code from pkg/k8s/

## Dependency Graph After Phases 1 and 3

After Phase 1 (remove-cluster-install) removes:
- `mcp_deploy.go`, `mcp_deploy_test.go`, `mcp_token.go`

After Phase 3 (mcp-cluster-profile) removes:
- `profile.go`, `profile_test.go`
- `buildClientForEnv()` from `pkg/cli/profile.go`

### Remaining `pkg/k8s/` files and their callers

| File | Exports | Called By | Keep? |
|------|---------|-----------|-------|
| `client.go` | `NewClient`, `Client`, `Apply`, `EnsureNamespace`, etc. | `deploy.go` (kind detection), possibly others | Audit |
| `kind.go` | `DetectKindCluster`, `KindClusterInfo` | `deploy.go:buildManifests` | Yes |
| `kind_test.go` | tests | n/a | Yes |
| `netpol.go` | `GenerateNetworkPolicy` | `deploy.go:buildManifests` | Yes |
| `netpol_test.go` | tests | n/a | Yes |
| `importmap.go` | `GenerateImportMapWithNamespace`, `ScanNodeImports` | `deploy.go:buildManifests` | Yes |
| `importmap_test.go` | tests | n/a | Yes |
| `preflight.go` | `RunPreflight` | Verify if called anywhere | Audit |
| `preflight_test.go` | tests | n/a | Audit |
| `e2e_security_test.go` | tests | n/a | Audit |

## Audit Process

1. For each exported function in `pkg/k8s/`, search all Go files outside
   `pkg/k8s/` for references.
2. If a function has zero external callers and is not part of a kept interface,
   remove it.
3. If a file has all exports removed, delete the file.
4. Run `go build ./...` and `go test ./...` after each removal batch.

## Client.go Analysis

The `Client` struct provides:
- `NewClient()` / `NewClientFromConfig()` / `NewClientWithContext()` -- K8s
  client constructors.
- `Apply()` / `ApplyWithStatus()` -- manifest application.
- `EnsureNamespace()` -- namespace creation.
- `Profile()` -- cluster profiling (removed in Phase 3).

If `deploy.go` no longer creates a K8s client directly (it uses MCP for apply),
and `profile.go` no longer creates a K8s client directly (it uses MCP), then
the `Client` struct and its constructors may be fully unused.

The `DetectKindCluster()` function in `kind.go` uses `NewClient()` internally.
This needs to be checked -- if kind detection still requires a direct K8s client,
then `client.go` must be kept.

## go.mod Cleanup

After removing dead code, run:
```bash
go mod tidy
go build ./...
go test -count=1 ./...
```

If `k8s.io/client-go` and `k8s.io/apimachinery` are no longer imported by any
remaining source file, they will be removed by `go mod tidy`.
