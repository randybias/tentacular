## Context

Pipedreamer v2's Go CLI has production code for K8s manifest generation (security contexts, probes, gVisor RuntimeClass), Dockerfile generation, deploy-time secret provisioning, and preflight check serialization. All of this code was written in changes 6-13 but shipped without test coverage. The existing 6 tests only cover `pkg/spec/parse.go`.

## Goals / Non-Goals

**Goals:**
- Test all security-critical fields in generated K8s manifests (pod security context, container security context, capabilities drop, seccomp profile)
- Test conditional RuntimeClass inclusion/omission
- Test Dockerfile structure (base image, copy instructions, entrypoint permissions)
- Test secret provisioning cascade (directory preferred over YAML, fallback, empty states)
- Test JSON serialization of preflight check results including omitempty behavior
- Use the same testing patterns as existing `pkg/spec/parse_test.go` (strings.Contains on output)

**Non-Goals:**
- Integration tests requiring a live K8s cluster
- Mocking the K8s API client (preflight tests only cover serialization, not API calls)
- Testing CLI command registration or flag parsing

## Decisions

### Decision 1: strings.Contains for manifest assertions
**Choice:** Assert K8s manifest content using `strings.Contains` on the raw YAML string output.
**Rationale:** Matches the existing pattern in `pkg/spec/parse_test.go`. The manifests are generated via `fmt.Sprintf` templates, so string matching is sufficient and avoids pulling in a YAML parser as a test dependency. Each test checks for a specific field or section, making failures easy to diagnose.

### Decision 2: Same-package tests for unexported functions
**Choice:** `pkg/cli/deploy_secrets_test.go` uses `package cli` (not `package cli_test`) to access unexported `buildSecretManifest`, `buildSecretFromDir`, `buildSecretFromYAML`.
**Rationale:** These functions are internal to the deploy command and should remain unexported. Testing them directly provides better coverage than testing through the cobra command runner, which would require mocking the K8s client.

### Decision 3: t.TempDir() for filesystem tests
**Choice:** Use `t.TempDir()` for tests that need temporary directories, with automatic cleanup.
**Rationale:** `t.TempDir()` provides automatic cleanup via `t.Cleanup()` — no need for `defer os.RemoveAll()`. This is the standard Go testing pattern for filesystem operations.

### Decision 4: Helper function for test workflows
**Choice:** `makeTestWorkflow(name)` creates a minimal valid `*spec.Workflow` for builder tests.
**Rationale:** Builder functions require a workflow struct but only use the Name field. The helper avoids repeating boilerplate across 18 tests while keeping each test self-documenting through the name parameter.

## Risks / Trade-offs

- **Brittleness to template changes** — Tests that match exact strings in YAML output will break if the template formatting changes (e.g., indentation). This is acceptable because template changes should be intentional and reviewed.
- **No negative testing for manifests** — Tests verify expected fields are present but don't verify absence of unexpected fields (except for RuntimeClass omission and CLI artifacts). Comprehensive negative testing would be over-engineering.
