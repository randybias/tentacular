## Context

The Deno engine loads secrets from multiple sources with a specific merge order: explicit --secrets flag wins outright, otherwise .secrets/ directory provides the base, .secrets.yaml merges on top, and /app/secrets (K8s volume mount) merges on top of everything. This logic was inline in main.ts and had zero test coverage.

## Goals / Non-Goals

**Goals:**
- Extract secrets cascade into a standalone, testable function
- Test all cascade paths: explicit, directory-only, yaml-only, both sources, empty, merge behavior
- Preserve identical runtime behavior after refactor
- Keep the API simple: one function, one options object

**Non-Goals:**
- Changing the cascade order or merge semantics
- Testing the underlying loadSecrets function (already has 6 tests)
- Testing /app/secrets K8s volume mount path (not available in test environment)

## Decisions

### Decision 1: Export resolveSecrets with options object
**Choice:** `resolveSecrets(opts: { explicitPath?, workflowDir })` returns `Promise<SecretsConfig>`.
**Rationale:** A single function with an options object is the simplest API. The caller (main.ts) passes the parsed flags directly. The function encapsulates all cascade logic.

### Decision 2: Reuse existing loadSecrets internally
**Choice:** cascade.ts imports and calls loadSecrets() for each source.
**Rationale:** loadSecrets already handles YAML files, directories, missing files, hidden files, and JSON parsing. The cascade module only needs to orchestrate the calling order and merge behavior.

### Decision 3: Test with real filesystem via Deno.makeTempDir
**Choice:** Tests create real temp directories and files, matching the pattern in secrets_test.ts.
**Rationale:** The function interacts with the filesystem through loadSecrets, so filesystem-based tests provide accurate coverage. Mocking would test implementation details rather than behavior.

## Risks / Trade-offs

- **/app/secrets path untestable in test environment** — The K8s volume mount path `/app/secrets` is checked by resolveSecrets but cannot be tested locally. This is the same limitation as the existing inline code. The behavior is covered by the loadSecrets unit tests which test directory loading generically.
