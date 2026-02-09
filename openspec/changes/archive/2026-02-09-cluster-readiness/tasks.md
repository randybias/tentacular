## 1. Expand Preflight Checks in pkg/k8s/preflight.go

- [x] 1.1 Refactor `PreflightCheck` to use `SelfSubjectAccessReview` API for RBAC validation instead of listing deployments as a proxy
- [x] 1.2 Implement namespace auto-creation in the `--fix` path using `CoreV1().Namespaces().Create()`
- [x] 1.3 Add secret references check: accept optional list of secret names, verify each exists via `CoreV1().Secrets().Get()` in target namespace
- [x] 1.4 Ensure early termination: if K8s API is unreachable, return immediately with only the API check result
- [x] 1.5 Add RBAC detail: check create/update/delete permissions for Deployments, Services, ConfigMaps, Secrets via `SelfSubjectAccessReview`

## 2. Update CLI Command in pkg/cli/cluster.go

- [x] 2.1 Update `runClusterCheck` to parse workflow spec from current directory (if present) and extract secret references for the preflight check
- [x] 2.2 Add JSON output support: when global `--output json` flag is set, serialize `[]CheckResult` as JSON array instead of text
- [x] 2.3 Update text output to show "auto-created" status when `--fix` successfully creates the namespace
- [x] 2.4 Ensure exit code is non-zero when any check fails

## 3. Verification

- [x] 3.1 Verify `go build ./cmd/pipedreamer/` compiles with the updated preflight and cluster check code
- [x] 3.2 Verify `pipedreamer cluster check --help` shows the `--fix` flag with its description
- [ ] 3.3 Verify that running `pipedreamer cluster check` against a reachable cluster produces pass/fail output for all five checks
- [ ] 3.4 Verify that running `pipedreamer cluster check --output json` produces valid JSON output
- [ ] 3.5 Verify that `--fix` creates a missing namespace when used
