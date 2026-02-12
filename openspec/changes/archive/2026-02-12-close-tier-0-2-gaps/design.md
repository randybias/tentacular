## Context

The tentacular CLI and example workflows have seven remaining gaps in the tier 0-2 roadmap. These range from a deploy-time bug (preflight secret check) to missing test coverage and documentation drift. All seven are independent, single-site changes with no cross-cutting architectural concerns. The codebase currently passes 100 Go tests and 47 Deno tests.

Key files and current state:

1. **Preflight secret check** (`pkg/cli/deploy.go:112-141`): When local secrets exist (`.secrets.yaml` or `.secrets/`), the deploy command adds the secret name to `secretNames` and calls `client.PreflightCheck()`, which verifies the secret exists in the cluster. But since the secret will be provisioned later in the same deploy (lines 144-151), the check fails on first deploy. The roadmap says this is resolved, but the current code still passes `secretNames` to `PreflightCheck` even when local secrets are present.

2. **Fixture emoji** (`example-workflows/uptime-prober/tests/fixtures/notify-slack.json`): The fixture itself is clean JSON, but the node source (`notify-slack.ts`) embeds Slack emoji shortcodes (`:red_circle:`, `:large_green_circle:`, etc.) in the Block Kit payload. The expected output in the fixture must account for the mock fetch returning `{mock: true}` status, not a real 200.

3. **hn-digest guard** (`example-workflows/hn-digest/nodes/filter-stories.ts:14`): `data.stories` is accessed without a guard. If upstream returns unexpected shape or the node is tested without proper input, this throws a TypeError.

4. **Undeploy ConfigMap** (`pkg/k8s/client.go:275-341`): `DeleteResources()` deletes Service, Deployment, Secret, and CronJobs but not the `<name>-code` ConfigMap created by deploy.

5. **Roadmap** (`docs/roadmap.md`): The preflight secret check item is listed as resolved in the Archive section but the bug still exists in code. Needs re-evaluation after the fix.

6. **Test gaps**: No test verifies that all Cobra subcommands are wired to the root command in `cmd/tntc/main.go`. No Deno test validates fixture loading with the config/secrets fields added to `engine/testing/fixtures.ts`.

7. **Graceful degradation**: Three nodes throw when Postgres credentials are missing (`store-health-data.ts`, `query-health-history.ts`, `store-report.ts`). Two nodes already degrade gracefully (`diff-seps.ts` checks `if (pgPassword)`, `analyze-trends.ts` checks `if (!apiKey)` and falls back to stats-only). The throwing nodes should follow the same pattern.

## Goals / Non-Goals

**Goals:**

- Fix the preflight secret check so first-time deploys with local secrets succeed
- Make all example workflow node tests pass reliably with mock context
- Clean up ConfigMaps on undeploy to prevent resource leaks
- Ensure roadmap.md accurately reflects current codebase state
- Add basic test coverage for Cobra command wiring and Deno fixture loading
- Make external-dep nodes return stub data when credentials are missing in test context
- Update node-development documentation to reflect current patterns

**Non-Goals:**

- Refactoring the preflight check system (just fixing the ordering bug)
- Adding integration test infrastructure (`--live` mode or real service testing)
- Changing the mock context API or adding filesystem mocking
- Modifying the node contract or Context type signature
- Adding new CLI commands or flags (except ConfigMap cleanup in existing undeploy)
- Immutable versioned ConfigMaps (that's Tier 3, item #9)

## Decisions

### D1: Preflight secret check -- skip secret existence check when local secrets present

When `secretNames` is populated from local secrets discovery, pass an empty slice to `PreflightCheck()` instead. The secret will be created as part of the same `Apply()` call, so checking for pre-existence is incorrect.

**Why not remove the secret check entirely:** The check is valid for `tntc cluster check --fix` and for deploys where no local secrets exist but the workflow references secrets. Only the deploy-with-local-secrets path should skip it.

**Implementation:** In `deploy.go`, change the preflight call to pass `nil` for `secretNames` when local secrets were detected (i.e., when `secretNames` was populated by the `.secrets/` or `.secrets.yaml` detection above). Add a comment explaining the ordering.

### D2: Fixture fixes -- use deterministic expected output matching mock fetch

The notify-slack fixture's `expected` field should match what the node returns when `ctx.fetch` returns a mock response. The mock fetch returns `{mock: true, service, path}` as JSON with status 200 and `ok: true`. So the expected output should be `{"delivered": true, "status": 200}`.

Review the current fixture to confirm it already has the correct expected values. If the fixture is correct, the issue may be that the node does not handle the mock response body correctly -- but since it only checks `response.ok` and `response.status`, this should work.

### D3: hn-digest guard -- defensive input validation

Add a guard at the top of `filter-stories.ts`:
```typescript
const stories = Array.isArray(data.stories) ? data.stories : [];
```

This ensures the node handles unexpected input shapes without throwing, returning `{ stories: [], filtered: 0 }` for invalid input.

### D4: Undeploy ConfigMap deletion -- add to DeleteResources()

Add ConfigMap deletion to `DeleteResources()` in `pkg/k8s/client.go` after the Secret deletion block. The ConfigMap name follows the convention `<name>-code` (same convention used in `GenerateCodeConfigMap()`).

**Why not use label selectors like CronJobs:** The ConfigMap is created with a deterministic name (`<name>-code`), not with tentacular labels, so direct name-based deletion is more reliable. The CronJob path uses labels because CronJob names include the trigger name suffix.

### D5: Cobra dispatch test -- verify subcommand registration

Create a test that instantiates the root command (or reconstructs it from `main.go` patterns) and verifies all expected subcommands are registered. This catches accidental removal of `root.AddCommand()` calls.

**Approach:** The test should call `root.Commands()` and check that each expected command name is present. This is a lightweight structural test, not a functional test of each command.

### D6: Deno fixture test -- validate config/secrets passthrough

Add a Deno test that loads a fixture with `config` and `secrets` fields, creates a mock context from them, and verifies the context has the correct values. This tests the `createMockContext({config, secrets})` path added in the fixture config/secrets feature.

### D7: Graceful degradation pattern -- check-and-return-stub

The three nodes that throw on missing credentials should follow the existing pattern from `diff-seps.ts` and `analyze-trends.ts`:

```typescript
if (!pgPassword) {
  ctx.log.warn("No postgres.password in secrets -- returning stub data");
  return { stored: false, rowId: 0 };
}
```

Each node returns a "not stored" / "empty result" stub that downstream nodes can handle. The warning log makes it clear in test output that the node degraded.

**store-health-data.ts:** Return `{ stored: false, rowId: 0 }`
**query-health-history.ts:** Return `{ records: [], periodStart: <now-24h>, periodEnd: <now>, snapshotCount: 0 }`
**store-report.ts (Postgres portion):** Skip Postgres operations but still attempt Azure blob upload if credentials are present. Return `{ stored: false, snapshotId: 0, reportId: 0, reportUrl: "" }`.

## Risks / Trade-offs

**Preflight check skip may mask genuine missing-secret issues** -- By skipping the secret existence check when local secrets are present, we assume the local secrets will be successfully applied. If the `Apply()` call fails for the Secret manifest specifically, the Deployment will reference a nonexistent secret. Mitigation: the `Apply()` call itself will fail and report the error, so the user still gets feedback. The preflight check was providing a false negative (failing when it should pass), not a useful guardrail.

**Graceful degradation hides real errors in production** -- If credentials are misconfigured in production (not just missing in tests), the nodes will silently return stub data instead of failing loudly. Mitigation: the `ctx.log.warn()` call ensures the degradation is visible in logs. A future enhancement could add a `ctx.env` or `ctx.mode` field to distinguish test vs production context.

**ConfigMap deletion on undeploy is not backwards-compatible for users who want to preserve code** -- Users who undeploy but want to keep the ConfigMap for inspection will lose it. Mitigation: the `--yes` confirmation prompt already exists. The ConfigMap name is deterministic and can be recreated by redeploying. This matches user expectation that "undeploy" removes everything.

**No migration needed** -- All changes affect generated output, test behavior, or documentation. No database schema changes, no API changes, no manifest format changes.
