# Tasks

## Implementation

- [ ] Remove `install` subcommand registration from `NewClusterCmd()` in `pkg/cli/cluster.go`
- [ ] Remove `runClusterInstall()` function from `pkg/cli/cluster.go`
- [ ] Remove `waitForMCPReady()` function from `pkg/cli/cluster.go`
- [ ] Remove `saveMCPToken()` function from `pkg/cli/cluster.go`
- [ ] Remove install-related imports from `pkg/cli/cluster.go` (net/http, time, os, path/filepath, k8s, mcp)
- [ ] Delete `pkg/k8s/mcp_deploy.go`
- [ ] Delete `pkg/k8s/mcp_deploy_test.go`
- [ ] Delete `pkg/k8s/mcp_token.go`
- [ ] Update `cmd/tntc/main.go` if it references any removed symbols
- [ ] Run `go mod tidy`

## Testing

- [ ] `go build ./...` passes
- [ ] `go test -count=1 ./...` passes
- [ ] `tntc cluster --help` shows `check` and `profile` but not `install`
- [ ] `tntc cluster install` returns "unknown command" error
