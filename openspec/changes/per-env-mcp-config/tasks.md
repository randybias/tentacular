# Tasks

## Implementation

- [ ] Add `MCPEndpoint` and `MCPTokenPath` fields to `EnvironmentConfig` in `pkg/cli/environment.go`
- [ ] Add `DefaultEnv` field to `TentacularConfig` in `pkg/cli/config.go`
- [ ] Update `mergeConfig()` in `pkg/cli/config.go` to merge `DefaultEnv`
- [ ] Update `ResolveEnvironment()` in `pkg/cli/environment.go` to check `DefaultEnv` when no env specified
- [ ] Update `requireMCPClient()` to resolve MCP config from active environment before falling back to global
- [ ] Update `tntc configure` in `pkg/cli/configure.go` to prompt for default_env and per-env MCP settings

## Testing

- [ ] Add unit tests for `ResolveEnvironment` with `DefaultEnv` set
- [ ] Add unit tests for MCP client resolution cascade (env-specific > global > error)
- [ ] Add unit test for `mergeConfig` with `DefaultEnv`
- [ ] Test backwards compatibility: config without `default_env` works as before
- [ ] `go build ./...` passes
- [ ] `go test -count=1 ./...` passes
