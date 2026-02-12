## Context

The 8 example workflows in `example-workflows/` are the primary onboarding reference for new developers. With Changes A-C landing (nested secrets YAML, per-workflow namespace, `tntc configure`, secrets management, fixture config/secrets), these examples need to be updated to demonstrate the new features and serve as correct reference implementations.

Current state:
- No workflow.yaml files have a `deployment:` section (T1-5 feature)
- 4 of 8 workflows are missing `.secrets.yaml.example` files (github-digest, hn-digest, pr-digest, and word-counter -- though hn-digest and word-counter don't use secrets)
- Test fixtures for secrets-dependent nodes have no `secrets` field, so tests always hit the "no credentials" early-exit path
- All 8 workflows have version "1.0" already

Secrets usage by workflow (from `ctx.secrets` grep):
- **cluster-health-collector**: `postgres.password`
- **cluster-health-reporter**: `postgres.password`, `anthropic.api_key`, `slack.webhook_url`
- **sep-tracker**: `postgres.password`, `azure.sas_token`, `slack.webhook_url`
- **uptime-prober**: `slack.webhook_url`
- **github-digest**: `slack.webhook_url`
- **pr-digest**: `slack.webhook_url`
- **hn-digest**: none
- **word-counter**: none

## Goals / Non-Goals

**Goals:**

- Add `deployment:` section with namespace to 4 production-targeted workflows
- Create `.secrets.yaml.example` files for github-digest and pr-digest (the two missing ones that actually use secrets)
- Add `secrets` field to test fixtures for nodes that check `ctx.secrets`, enabling those nodes to exercise their credential-dependent code paths
- Validate all workflows pass `tntc validate` after changes

**Non-Goals:**

- Changing node logic or behavior in any workflow
- Adding `deployment:` section to general-purpose/test workflows (github-digest, pr-digest, hn-digest, word-counter)
- Creating `.secrets.yaml.example` for hn-digest or word-counter (they don't use secrets)
- Updating fixture `expected` values -- the mock fetch still returns mock responses, so `delivered: false` etc. remains correct even with secrets present
- Adding `config` fields to fixtures (no workflows currently use `ctx.config`)

## Decisions

### D1: Namespace naming convention

Use `pd-` prefix (short for the project name) followed by the workflow name or logical group:
- `pd-cluster-health` for both cluster-health-collector and cluster-health-reporter (shared namespace for the same data pipeline)
- `pd-sep-tracker` for sep-tracker
- `pd-uptime-prober` for uptime-prober

General-purpose workflows (github-digest, pr-digest, hn-digest) and the test workflow (word-counter) omit the `deployment:` section entirely, relying on the namespace cascade fallback.

### D2: .secrets.yaml.example format

Follow the existing pattern from uptime-prober's `.secrets.yaml.example`: commented header explaining the purpose, then commented YAML showing the required structure. The `tntc secrets init` command (T2-7) will uncomment these lines when creating `.secrets.yaml`.

### D3: Fixture secrets -- minimal additions

Only add `secrets` to fixtures for nodes that actually read `ctx.secrets`. The secret values should be plausible test values (not real credentials). The `expected` output should NOT change because the mock `ctx.fetch()` still returns mock responses regardless of whether secrets are present -- the node just takes a different code branch (webhook POST path vs. early exit).

Fixtures to update:
- `uptime-prober/tests/fixtures/notify-slack.json` -- add `secrets.slack.webhook_url`
- `github-digest/tests/fixtures/notify.json` -- add `secrets.slack.webhook_url`
- `pr-digest/tests/fixtures/notify-slack.json` -- add `secrets.slack.webhook_url`
- `sep-tracker/tests/fixtures/notify.json` -- add `secrets.slack.webhook_url`
- `sep-tracker/tests/fixtures/store-report.json` -- add `secrets.postgres.password` and `secrets.azure.sas_token`
- `sep-tracker/tests/fixtures/diff-seps.json` -- add `secrets.postgres.password`
- `cluster-health-collector/tests/fixtures/store-health-data.json` -- add `secrets.postgres.password`
- `cluster-health-reporter/tests/fixtures/query-health-history.json` -- add `secrets.postgres.password`
- `cluster-health-reporter/tests/fixtures/send-report.json` -- add `secrets.slack.webhook_url`
- `cluster-health-reporter/tests/fixtures/analyze-trends.json` -- add `secrets.anthropic.api_key`

### D4: No migration needed

All changes are to example/reference content. No deployed workflows are affected. No rollback concern.

## Risks / Trade-offs

**Fixture expected values may need adjustment** -- If adding secrets causes a node to take a different code path that produces a structurally different output, the `expected` field may need updating. This should be caught by running `deno test` after the fixture updates. Mitigation: run the test suite after each fixture update and adjust expected values as needed.

**hn-digest and word-counter have no secrets** -- These two workflows don't reference `ctx.secrets` at all, so they get no `.secrets.yaml.example` and no fixture changes. This is intentional, not an oversight.
