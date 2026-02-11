## Context

Tentacular deploys workflow containers to Kubernetes using gVisor sandboxing (the "Fortress" deployment pattern). The existing codebase already has a `pkg/k8s/client.go` with a client-go wrapper, a stub `PreflightCheck` method in `pkg/k8s/preflight.go`, and a `cluster check` cobra subcommand in `pkg/cli/cluster.go`. This change completes the preflight check implementation to cover all required validations with clear remediation output.

## Goals / Non-Goals

**Goals:**
- Implement a comprehensive preflight check suite that validates K8s cluster readiness before deployment
- Return structured `CheckResult` results with pass/fail status and actionable remediation messages
- Support a `--fix` flag that auto-creates the target namespace when missing
- Support both human-readable text output and JSON output (via global `--output` flag)
- Validate secret references from workflow specs resolve to existing K8s Secrets
- Provide early return on fatal failures (e.g., K8s API unreachable skips remaining checks)

**Non-Goals:**
- Auto-installing gVisor or configuring RuntimeClasses (remediation only provides guidance)
- Auto-creating RBAC roles or bindings (remediation tells the user what permissions are needed)
- Health-checking individual nodes or workloads already running in the cluster
- Validating the workflow spec itself (that is the `validate` command's job)
- Implementing `cluster` subcommands beyond `check` (e.g., no `cluster status` in this change)

## Decisions

### Decision 1: CheckResult struct pattern
**Choice:** Each preflight check returns a `CheckResult{Name, Passed, Remediation}` struct. The full suite returns `[]CheckResult`.
**Rationale:** This pattern is already in place in `pkg/k8s/preflight.go`. It keeps check logic decoupled from output formatting. The CLI layer iterates the results and formats output. JSON output serializes the slice directly. This also makes it straightforward to add new checks later without changing the output logic.

### Decision 2: Ordered sequential checks with early termination
**Choice:** Checks run sequentially in a fixed order: (1) API reachability, (2) gVisor RuntimeClass, (3) namespace existence, (4) RBAC permissions, (5) secret references. If the API is unreachable, remaining checks are skipped.
**Rationale:** All checks depend on the K8s API being reachable, so continuing after an API failure produces misleading errors. The fixed order groups checks from infrastructure-level (API, RuntimeClass) to namespace-level (namespace, RBAC) to application-level (secrets), which is the natural debugging order.

### Decision 3: --fix flag limited to namespace creation
**Choice:** The `--fix` flag only auto-creates the target namespace. It does not auto-fix gVisor, RBAC, or secrets.
**Rationale:** Namespace creation is a safe, idempotent operation. Auto-creating RBAC roles could inadvertently grant excessive permissions. Auto-installing gVisor requires node-level access. Secrets contain sensitive data that should not be auto-generated. Limiting `--fix` to namespace creation balances convenience with safety.

### Decision 4: RBAC check via SelfSubjectAccessReview
**Choice:** Use the `SelfSubjectAccessReview` API to check whether the current identity can create/update/delete Deployments, Services, ConfigMaps, and Secrets in the target namespace.
**Rationale:** The current implementation attempts to list deployments as a proxy for RBAC validation. `SelfSubjectAccessReview` is the correct K8s API for permission checks -- it tells you exactly what the current identity can do without requiring actual resources to exist. This avoids false negatives when the namespace is empty.

### Decision 5: Secret reference validation
**Choice:** Accept an optional workflow spec path or parse secrets from the current directory's `workflow.yaml`. For each secret reference in the spec, check whether a K8s Secret with that name exists in the target namespace.
**Rationale:** Secret reference mismatches are a common deployment failure. Checking at preflight time prevents containers from crashing at startup due to missing secret mounts. This check is optional -- if no workflow spec is provided or found, the check is skipped with an informational message.

## Risks / Trade-offs

- **RBAC SelfSubjectAccessReview requires RBAC read permissions** -- If the service account cannot perform access reviews, this check itself fails. Remediation message guides the user to verify their kubeconfig context.
- **Secret reference check depends on workflow spec parsing** -- This introduces a dependency on the spec parser from `pkg/spec/`. If the parser is not yet implemented, this check is skipped gracefully.
- **gVisor check assumes RuntimeClass name is "gvisor"** -- If clusters use a different name (e.g., "runsc"), this check fails. Could be made configurable in future but hardcoded for now since "gvisor" is the conventional name.
- **--fix namespace creation uses default labels** -- The auto-created namespace gets no special labels or annotations. Production clusters may require specific labels (e.g., for network policies). Remediation message notes this limitation.
