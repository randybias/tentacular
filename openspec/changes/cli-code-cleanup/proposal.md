# Remove dead code from pkg/k8s/

## Why

After Phases 1 and 3 (remove-cluster-install and mcp-cluster-profile), several
files in `pkg/k8s/` will be unused:

- `mcp_deploy.go` -- MCP server manifest generation (removed in Phase 1).
- `mcp_deploy_test.go` -- tests for the above.
- `mcp_token.go` -- MCP bearer token generation (removed in Phase 1).
- `profile.go` -- direct K8s cluster profiling (removed in Phase 3).
- `profile_test.go` -- tests for the above.
- `preflight.go` -- direct K8s preflight checks (already routed through MCP via
  `cluster check`, but the local implementation may still exist).
- `preflight_test.go` -- tests for the above.
- `client.go` functions -- `NewClient`, `NewClientFromConfig`, `NewClientWithContext`,
  `EnsureNamespace`, `Apply`, etc. may become unused if no other code paths
  reference them.

Additionally, `k8s.io/client-go` and `k8s.io/apimachinery` may become removable
from `go.mod` if no remaining code paths require direct K8s API access.

## What Changes

- **Audit `pkg/k8s/`** for functions that are no longer called after Phases 1
  and 3.
- **Remove dead files** (`mcp_deploy.go`, `mcp_token.go`, `profile.go`,
  `preflight.go` and their tests) if confirmed unused.
- **Remove dead functions** from `client.go` if confirmed unused.
- **Run `go mod tidy`** to drop unused dependencies.
- **Verify `go build ./...` and `go test ./...` pass** after cleanup.
- **Keep** files still in use: `netpol.go`, `importmap.go`, `kind.go`, and any
  builder/spec code that remains necessary for manifest generation.

## Acceptance Criteria

- `go build ./...` passes with no errors.
- `go test -count=1 ./...` passes with no failures.
- No unreferenced exported functions remain in `pkg/k8s/`.
- `go mod tidy` produces no diff (all deps are necessary).

## Non-goals

- Refactoring or reorganizing remaining `pkg/k8s/` code -- this is strictly a
  dead-code removal pass.
- Moving code to other packages.
