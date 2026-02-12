## Context

Two gaps exist in the developer workflow:

1. **Secrets management** -- Developers deploying a workflow have no tooling to verify secrets are provisioned. The only feedback is runtime failure when a node tries to read `ctx.secrets.slack.webhook_url` and gets undefined. There's also no way to share secrets across workflows -- each workflow must duplicate its `.secrets.yaml` even when multiple workflows use the same Slack webhook.

2. **Test fixture limitations** -- The `createMockContext()` in `engine/testing/mocks.ts:19-60` provides `config: {}` and `secrets: {}` by default. While it accepts `overrides?: Partial<Context>`, the test runner at `engine/testing/runner.ts:113` calls `createMockContext()` with no arguments. The `TestFixture` interface at `engine/testing/fixtures.ts:3-6` only has `input` and `expected` -- no config or secrets fields. This means all node tests hit the "no secrets" code path.

The `buildSecretFromYAML()` function in `deploy.go` (modified by T0-1 to handle `map[string]interface{}`) is the natural integration point for shared secret resolution, since it already processes `.secrets.yaml` before generating K8s manifests.

## Goals / Non-Goals

**Goals:**

- `tntc secrets check` scans node source for `ctx.secrets` patterns and reports provisioning gaps
- `tntc secrets init` scaffolds `.secrets.yaml` from `.secrets.yaml.example`
- `$shared.<name>` syntax in `.secrets.yaml` resolves from repo-root `.secrets/` directory
- Test fixtures support `config` and `secrets` fields passed through to mock context
- Shared secret resolution integrates into existing `buildSecretFromYAML()` flow

**Non-Goals:**

- Encrypted secrets storage (secrets remain plaintext files, gitignored)
- Remote secrets backends (Vault, AWS Secrets Manager, etc.)
- Automatic secret rotation or expiry
- Config field in fixtures affecting test behavior (config is passed through but not validated)

## Decisions

### D1: Regex-based secrets scanning for `secrets check`

Scan `nodes/*.ts` files for patterns matching `ctx\.secrets\??\.\w+` to extract service names. This captures both `ctx.secrets.slack` and `ctx.secrets?.slack` patterns.

**Why regex over AST parsing:** The patterns are simple and consistent across all nodes. An AST parser (TypeScript compiler API) would be a heavy dependency for pattern matching that a regex handles adequately. False positives are unlikely given the specific `ctx.secrets` prefix.

### D2: Shared secrets convention using `$shared.` prefix

String values in `.secrets.yaml` starting with `$shared.` are resolved to files in `<repo-root>/.secrets/`. The repo root is found by walking up from the workflow directory looking for `.git/` or `go.mod`.

Resolution happens inside `buildSecretFromYAML()` after YAML unmarshaling and before K8s Secret manifest generation. This means shared secrets participate in the T0-1 nested YAML handling automatically -- if a shared secret file contains JSON, it's parsed into a nested object.

**Why `$shared.` prefix:** Clear, explicit, unlikely to collide with actual secret values. The `$` prefix is conventional for variable substitution.

**Why repo-root `.secrets/` dir:** Matches the existing per-workflow `.secrets/` dir convention but at repo scope. Files in this directory are gitignored along with other secrets.

### D3: Fixture config/secrets as optional fields with spread override

Extend `TestFixture` interface with optional `config` and `secrets` fields. In the test runner, pass these to `createMockContext()` as the overrides parameter. The existing spread operator in `mocks.ts:56` (`...overrides`) handles the merge.

**Why not a separate fixture format:** Adding optional fields to the existing interface is backwards compatible. Existing fixtures without `config`/`secrets` continue to work identically -- `fixture.config ?? {}` and `fixture.secrets ?? {}` provide safe defaults.

## Risks / Trade-offs

**Regex scanning can miss dynamic secret access patterns** -- If a node uses `ctx.secrets[dynamicKey]`, the regex won't catch it. This is acceptable because the coding convention uses direct property access (`ctx.secrets.service.key`), and `secrets check` is advisory, not blocking.

**`$shared.` resolution requires repo root discovery** -- The `findRepoRoot()` function walks up directories looking for `.git/` or `go.mod`. This could fail in unusual directory structures. If no repo root is found, shared secrets are silently skipped (no error), which is the correct fallback for standalone workflow directories.

**Shared secrets introduce a coupling between workflow and repo-root `.secrets/` directory** -- A workflow using `$shared.slack` breaks if moved to a different repo without the shared secrets directory. This is acceptable because shared secrets are a convenience for mono-repo setups; standalone workflows should use per-workflow `.secrets.yaml` with actual values.
