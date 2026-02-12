## 1. T2-8: Fixture Config/Secrets Support

- [x] 1.1 In `engine/testing/fixtures.ts`, extend the `TestFixture` interface to add optional fields: `config?: Record<string, unknown>` and `secrets?: Record<string, Record<string, string>>`
- [x] 1.2 In `engine/testing/runner.ts` line 113, change `const ctx = createMockContext()` to `const ctx = createMockContext({ config: fixture.config ?? {}, secrets: fixture.secrets ?? {} })`
- [x] 1.3 Verify that existing fixtures (no config/secrets fields) still work correctly with the `?? {}` defaults

## 2. T2-7 Phase A: Secrets Check Command

- [x] 2.1 Create `pkg/cli/secrets.go` with `NewSecretsCmd()` returning a cobra command with `check` and `init` subcommands
- [x] 2.2 Implement `runSecretsCheck()`: scan `nodes/*.ts` files with regex `ctx\.secrets\??\.\w+`, extract service names, compare against `.secrets.yaml` and `.secrets/` dir entries
- [x] 2.3 Implement `runSecretsInit()`: copy `.secrets.yaml.example` to `.secrets.yaml`, uncomment lines by stripping `# ` prefix, error if target exists without `--force`
- [x] 2.4 In `cmd/tntc/main.go`, register `cli.NewSecretsCmd()` with `root.AddCommand()`

## 3. T2-7 Phase B: Shared Secrets Pool

- [x] 3.1 In `pkg/cli/secrets.go` (or `deploy.go`), implement `findRepoRoot(dir string) string` that walks up from dir looking for `.git/` or `go.mod`
- [x] 3.2 Implement `resolveSharedSecrets(secrets map[string]interface{}, workflowDir string) error` that scans for `$shared.` prefixed values and resolves from `<repo-root>/.secrets/<name>`
- [x] 3.3 In `buildSecretFromYAML()` (deploy.go), call `resolveSharedSecrets()` after YAML unmarshaling but before generating the K8s Secret manifest. This builds on T0-1's `map[string]interface{}` type change.

## 4. Tests

- [x] 4.1 Create `pkg/cli/secrets_test.go` with `TestSecretsCheckFindsRequiredSecrets` -- regex extraction from sample node source
- [x] 4.2 Add `TestSecretsCheckReportsGaps` -- missing secrets reported correctly
- [x] 4.3 Add `TestSecretsCheckAllProvisioned` -- happy path with all secrets covered
- [x] 4.4 Add `TestSecretsInitCreatesFile` -- copies and uncomments example file
- [x] 4.5 Add `TestSecretsInitRefusesOverwrite` -- errors when `.secrets.yaml` exists
- [x] 4.6 Add `TestResolveSharedSecrets` -- `$shared.slack` resolves to file content
- [x] 4.7 Add `TestResolveSharedSecretsMissing` -- errors on missing shared file
- [x] 4.8 Add `TestResolveSharedSecretsNoRepoRoot` -- gracefully skips when no repo root

## 5. Verification

- [x] 5.1 Run `go test ./pkg/cli/ -run TestSecrets` -- all 8 secrets tests pass
- [x] 5.2 Run `go test ./pkg/...` -- no regressions
- [x] 5.3 Run `cd engine && deno test --allow-read --allow-net testing/` -- engine tests pass with updated fixture interface
