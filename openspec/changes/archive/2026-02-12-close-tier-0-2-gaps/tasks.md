## 1. Phase 1: Preflight Secret Check Bug Fix

- [ ] 1.1 In `pkg/cli/deploy.go:125`, pass `nil` instead of `secretNames` to `client.PreflightCheck()` when local secrets were detected (the `.secrets/` or `.secrets.yaml` blocks above populated `secretNames`). Add a comment explaining that the secret will be auto-provisioned later in the same deploy.
- [ ] 1.2 Verify the preflight check still runs its other checks (K8s API reachable, gVisor RuntimeClass, namespace exists, RBAC permissions) regardless of whether secrets are skipped.
- [ ] 1.3 Update the Archive entry in `docs/roadmap.md` for "Preflight Secret Provisioning Ordering" to reflect the actual fix applied.

## 2. Phase 2: Quick-Win Fixture and Guard Fixes

- [ ] 2.1 Review `example-workflows/uptime-prober/tests/fixtures/notify-slack.json` -- verify the expected output matches what `notify-slack.ts` returns when `ctx.fetch` provides a mock response (mock fetch returns `ok: true, status: 200` equivalent). Fix if mismatched.
- [ ] 2.2 In `example-workflows/hn-digest/nodes/filter-stories.ts:14`, add a guard: `const stories = Array.isArray((input as any).stories) ? (input as any).stories : [];` so the node returns `{ stories: [], filtered: 0 }` for unexpected input instead of throwing.

## 3. Phase 3: Undeploy ConfigMap Cleanup

- [ ] 3.1 In `pkg/k8s/client.go` `DeleteResources()`, add ConfigMap deletion after the Secret deletion block. Delete `<name>-code` using `c.clientset.CoreV1().ConfigMaps(namespace).Delete()`. Skip silently on NotFound (same pattern as other resources).
- [ ] 3.2 Add the deletion to the `deleted` slice as `"ConfigMap/<name>-code"` for output reporting.
- [ ] 3.3 Update `tentacular-skill/SKILL.md` undeploy row to remove the note about ConfigMap not being deleted.

## 4. Phase 4: Roadmap Verification

- [ ] 4.1 Verify all items in the `docs/roadmap.md` Archive section are actually resolved in the current codebase.
- [ ] 4.2 Verify all open items (Tier 2-4) are correctly placed and have accurate descriptions.
- [ ] 4.3 Move the "Preflight Secret Provisioning Ordering" archive entry description to accurately match the fix from Phase 1 (skip preflight secret check when local secrets will be auto-provisioned).

## 5. Phase 5: Minor Test Gaps

- [ ] 5.1 Create a Go test (in `cmd/tntc/` or `pkg/cli/`) that instantiates the root Cobra command and verifies all expected subcommands are registered: init, validate, dev, test, build, deploy, status, run, logs, list, undeploy, cluster, configure, secrets, visualize.
- [ ] 5.2 Create a Deno test (in `engine/testing/`) that validates `loadFixture()` correctly parses a fixture with `config` and `secrets` fields, and that `createMockContext({config, secrets})` makes those values accessible via `ctx.config` and `ctx.secrets`.
- [ ] 5.3 Run `go test ./...` and verify the new Cobra test passes along with all existing tests.
- [ ] 5.4 Run `cd engine && deno test --allow-read --allow-write=/tmp --allow-net --allow-env` and verify the new fixture test passes along with all existing tests.

## 6. Phase 6: Graceful Degradation for External-Dep Nodes

- [ ] 6.1 In `example-workflows/cluster-health-collector/nodes/store-health-data.ts`, replace the `throw new Error("Missing postgres.password secret")` with `ctx.log.warn("No postgres.password in secrets -- returning stub"); return { stored: false, rowId: 0 };`
- [ ] 6.2 In `example-workflows/cluster-health-reporter/nodes/query-health-history.ts`, replace the `throw new Error("Missing postgres.password secret")` with `ctx.log.warn("No postgres.password in secrets -- returning empty history"); return { records: [], periodStart: new Date(Date.now() - 86400000).toISOString(), periodEnd: new Date().toISOString(), snapshotCount: 0 };`
- [ ] 6.3 In `example-workflows/sep-tracker/nodes/store-report.ts`, replace the `throw new Error("Missing postgres.password secret")` with a warn log and skip the Postgres block, proceeding to Azure blob upload check. Return `{ stored: false, snapshotId: 0, reportId: 0, reportUrl: "" }` when both Postgres and Azure are unavailable.
- [ ] 6.4 Add or update test fixtures for the three modified nodes to test the no-credentials path (fixture with empty secrets, expected stub output).
- [ ] 6.5 Verify `diff-seps.ts` and `analyze-trends.ts` already degrade gracefully (no changes needed -- just confirm).

## 7. Phase 7: Documentation Accuracy

- [ ] 7.1 Update `tentacular-skill/SKILL.md` to document the graceful degradation pattern (check secrets, warn, return stub) for nodes with external dependencies.
- [ ] 7.2 Update `tentacular-skill/references/testing-guide.md` to include examples of fixture files with `config` and `secrets` fields, and the expected behavior when credentials are missing.
- [ ] 7.3 Verify `docs/testing.md` test counts are accurate after adding new tests in Phase 5.

## 8. Verification

- [ ] 8.1 Run `go test ./pkg/...` -- all tests pass
- [ ] 8.2 Run `go test ./cmd/...` -- new Cobra dispatch test passes
- [ ] 8.3 Run `cd engine && deno test --allow-read --allow-write=/tmp --allow-net --allow-env` -- all 47+ tests pass
- [ ] 8.4 Run `tntc test example-workflows/uptime-prober` -- all fixture tests pass
- [ ] 8.5 Run `tntc test example-workflows/hn-digest` -- all fixture tests pass
- [ ] 8.6 Confirm no regressions in existing workflows: `tntc validate` on all example workflows
