# Tasks

## Implementation

- [ ] Create `pkg/cli/promote.go` with `NewPromoteCmd()` and `runPromote()`
- [ ] Register `promote` command in `cmd/tntc/main.go`
- [ ] Add `WfHealth()` method to `pkg/mcp/client.go` if not already present
- [ ] Implement source health gate via `wf_health` (only promote GREEN workflows)
- [ ] Re-build manifests locally from workflow source using target env settings via `buildManifests()`
- [ ] Implement target deployment via `wf_apply`
- [ ] Implement optional `--verify` post-promotion check via `wf_run`
- [ ] Add secrets warning when source has secrets and target may not
- [ ] Add confirmation prompt (skippable with `--force`)

## Testing

- [ ] Add unit tests for promote flow with mocked MCP clients
- [ ] Test health gate: source workflow not healthy -> promotion aborted
- [ ] Test error case: source workflow does not exist
- [ ] Test error case: target MCP server unreachable
- [ ] Test `--verify` flag triggers post-promotion `wf_run`
- [ ] Test secrets warning is emitted when applicable
- [ ] `go build ./...` passes
- [ ] `go test -count=1 ./...` passes
