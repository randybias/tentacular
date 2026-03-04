# Tasks

## Implementation

- [ ] Audit all exports in `pkg/k8s/` for external callers
- [ ] Delete `pkg/k8s/mcp_deploy.go` and `pkg/k8s/mcp_deploy_test.go` (if not already done in Phase 1)
- [ ] Delete `pkg/k8s/mcp_token.go` (if not already done in Phase 1)
- [ ] Delete `pkg/k8s/profile.go` and `pkg/k8s/profile_test.go` (if not already done in Phase 3)
- [ ] Delete `pkg/k8s/preflight.go` and `pkg/k8s/preflight_test.go` if no external callers remain
- [ ] Remove unused functions from `pkg/k8s/client.go` (NewClient, Apply, EnsureNamespace, etc. if unused)
- [ ] Delete `pkg/k8s/e2e_security_test.go` if it tests removed functionality
- [ ] Run `go mod tidy` to drop unused dependencies
- [ ] Remove unused imports from any modified files

## Verification

- [ ] `go build ./...` passes with no errors
- [ ] `go test -count=1 ./...` passes with no failures
- [ ] `go vet ./...` passes
- [ ] No unreferenced exported functions remain in `pkg/k8s/` (verify with grep)
- [ ] `go mod tidy` produces no diff
