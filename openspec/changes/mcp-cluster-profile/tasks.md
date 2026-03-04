# Tasks

## Implementation

- [ ] Add `ClusterProfile()` method to `pkg/mcp/client.go`
- [ ] Rewrite `runProfileForEnv()` in `pkg/cli/profile.go` to use MCP client
- [ ] Remove `buildClientForEnv()` from `pkg/cli/profile.go`
- [ ] Update `AutoProfileEnvironments()` to use MCP client per environment
- [ ] Update profile rendering to work with MCP response format
- [ ] Handle case where MCP server is not yet configured (skip with warning)
- [ ] Remove direct K8s imports from `pkg/cli/profile.go` if no longer needed

## Testing

- [ ] Add unit tests for `ClusterProfile()` MCP client method
- [ ] Update profile tests to mock MCP calls instead of K8s client
- [ ] Test `--save` flag writes correct markdown and JSON from MCP response
- [ ] Test `--all` flag profiles all environments via their respective MCP servers
- [ ] Test freshness check still works with MCP-sourced profiles
- [ ] `go build ./...` passes
- [ ] `go test -count=1 ./...` passes
