## 1. Audit

- [ ] 1.1 Catalog all exported functions in `pkg/k8s/client.go` with caller analysis (grep for usage in `pkg/cli/`).
- [ ] 1.2 Categorize each function: remove (MCP-replaced), keep-bootstrap, keep-build.
- [ ] 1.3 Document the audit results for review before proceeding with removal.

## 2. Remove MCP-Replaced Functions

- [ ] 2.1 Remove deploy/apply functions that are now handled by `wf_apply`/`wf_deploy` MCP tools.
- [ ] 2.2 Remove status/query functions replaced by `wf_pods`, `wf_logs`, `wf_events`, `wf_describe`.
- [ ] 2.3 Remove run/trigger functions replaced by `wf_run`, `wf_trigger` MCP tools.
- [ ] 2.4 Remove namespace management functions replaced by `ns_list`, `ns_ensure` MCP tools.
- [ ] 2.5 Remove corresponding test functions from `*_test.go` files.

## 3. Retain and Clean Up

- [ ] 3.1 Keep `NewClient()`, `loadConfig()` for bootstrap operations.
- [ ] 3.2 Keep `pkg/k8s/importmap.go` and `pkg/k8s/importmap_test.go` (build-time).
- [ ] 3.3 Keep `pkg/k8s/netpol.go` if used for manifest generation (build-time).
- [ ] 3.4 Keep `pkg/k8s/kind.go` for local development cluster management.
- [ ] 3.5 Evaluate `pkg/k8s/profile.go` -- keep if used for build-time profile resolution.
- [ ] 3.6 Evaluate `pkg/k8s/preflight.go` -- move pre-deploy checks to MCP or keep if build-time.

## 4. Verify

- [ ] 4.1 Run `go build ./...` -- no compile errors.
- [ ] 4.2 Run `go test ./...` -- all tests pass.
- [ ] 4.3 Run `go vet ./...` -- no issues.
