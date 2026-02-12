## Why

The 8 example workflows need to be updated to use the new features from Changes A-C: per-workflow deployment namespaces, `.secrets.yaml.example` files for secrets scaffolding, and fixture config/secrets for meaningful testing. Since backwards compatibility is not required, this is a clean sweep to bring all examples up to current standards.

## What Changes

- **Add `deployment:` section to workflow.yaml files** -- 4 workflows get explicit namespaces (`pd-cluster-health`, `pd-sep-tracker`, `pd-uptime-prober`). 4 general-purpose/test workflows omit it.
  - `cluster-health-collector/workflow.yaml` -- `deployment.namespace: pd-cluster-health`
  - `cluster-health-reporter/workflow.yaml` -- `deployment.namespace: pd-cluster-health`
  - `sep-tracker/workflow.yaml` -- `deployment.namespace: pd-sep-tracker`
  - `uptime-prober/workflow.yaml` -- `deployment.namespace: pd-uptime-prober`
  - `github-digest`, `pr-digest`, `hn-digest`, `word-counter` -- no namespace (general purpose / test)
- **Create missing `.secrets.yaml.example` files** -- `github-digest` (needs `slack.webhook_url`) and `pr-digest` (needs `github.token`, `anthropic.api_key`, `slack.webhook_url`).
- **Update test fixtures with secrets** -- Add `secrets` field to fixtures for secrets-dependent nodes (e.g., `uptime-prober/tests/fixtures/notify-slack.json` gets `secrets.slack.webhook_url`), exercising the new fixture config/secrets support from T2-8.

## Capabilities

### New Capabilities

(none -- all changes are updates to existing example content)

### Modified Capabilities

- `workflow-spec`: Example workflows demonstrate the new `deployment.namespace` field.

## Impact

- **Code**: `example-workflows/*/workflow.yaml` (8 files), `example-workflows/*/tests/fixtures/*.json` (select fixtures)
- **New files**: `example-workflows/github-digest/.secrets.yaml.example`, `example-workflows/pr-digest/.secrets.yaml.example`
- **Tests**: No new Go/Deno tests. Updated fixtures serve as integration test data.
- **Dependencies**: Depends on Changes A (nested YAML), B (per-workflow namespace), and C (fixture config/secrets) being complete.
- **Breaking**: None. Example workflows are reference implementations, not consumed as libraries.
