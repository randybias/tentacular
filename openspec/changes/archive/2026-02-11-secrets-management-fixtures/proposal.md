## Why

Developers have no way to check whether required secrets are provisioned before deploying, leading to runtime failures. There is no scaffolding to help initialize secret files from examples. Shared secrets (e.g., a single Slack webhook used by multiple workflows) must be duplicated per workflow. Additionally, the test framework's mock context always provides empty config and secrets, preventing meaningful testing of nodes that branch on secret availability. These Tier 2 items close the secrets management and testing gaps.

## What Changes

- **T2-7: Secrets management (Phase A + B)** -- New `tntc secrets` command with two subcommands:
  - `tntc secrets check <workflow-dir>` -- Scans node source files for `ctx.secrets` access patterns, compares against locally provisioned secrets (`.secrets.yaml` or `.secrets/` dir), reports gaps.
  - `tntc secrets init <workflow-dir>` -- Copies `.secrets.yaml.example` to `.secrets.yaml`, uncommenting example values.
  - **Shared secrets pool (Phase B)** -- Convention: `.secrets/` directory at repo root contains shared secret files. Workflow `.secrets.yaml` can reference them with `$shared.<name>` syntax, resolved during `buildSecretFromYAML()`. New files: `pkg/cli/secrets.go`, `pkg/cli/secrets_test.go`. Modified: `cmd/tntc/main.go`, `pkg/cli/deploy.go`.
- **T2-8: Fixture config/secrets support** -- Extend the `TestFixture` interface in `engine/testing/fixtures.ts` to include optional `config` and `secrets` fields. Pass these through to `createMockContext()` in the test runner. This allows test fixtures to provide secrets so nodes take the "has credentials" code path instead of always hitting the early-exit "no secrets" path. Modified: `engine/testing/fixtures.ts`, `engine/testing/runner.ts`.

## Capabilities

### New Capabilities

- `secrets-management`: The `tntc secrets check` and `tntc secrets init` commands, plus the `$shared.<name>` shared secrets pool convention and resolution logic.

### Modified Capabilities

- `k8s-deploy`: `buildSecretFromYAML()` resolves `$shared.<name>` references from the repo-root `.secrets/` directory before generating the K8s Secret manifest.
- `testing-framework`: Test fixtures support optional `config` and `secrets` fields, passed through to mock context, enabling fixture-driven testing of secrets-dependent nodes.

## Impact

- **Code**: `pkg/cli/deploy.go`, `engine/testing/fixtures.ts`, `engine/testing/runner.ts`
- **New files**: `pkg/cli/secrets.go`, `pkg/cli/secrets_test.go`
- **Tests**: `pkg/cli/secrets_test.go` (8 new tests)
- **APIs**: New `tntc secrets` command with `check` and `init` subcommands. Extended `TestFixture` interface (additive, non-breaking).
- **Dependencies**: None beyond stdlib.
- **Breaking**: None. All additions are optional and additive.
