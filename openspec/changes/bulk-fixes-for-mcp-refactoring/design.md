## Context

The MCP server refactoring moved deployment orchestration from the CLI to the MCP server. During end-to-end testing, four issues were found that prevent correct operation in PSA-restricted (baseline/restricted) namespaces. All four are in `pkg/builder/k8s.go` manifest generation.

## Goals / Non-Goals

**Goals:**
- Fix import map path so Deno engine resolves modules correctly
- Make proxy-prewarm and CronJob trigger pods PSA-compliant
- Generate egress NetworkPolicy for trigger pods
- Fix secrets volume mount condition to work with contract dependencies

**Non-Goals:**
- Refactoring the manifest generation architecture
- Adding new deployment capabilities
- Changing the MCP server itself (separate repo)

## Decisions

### Import map path fix
The ConfigMap-based code delivery mounts code at `/app/` and the engine entrypoint is `/app/mod.ts`. The import map was incorrectly referencing `./engine/mod.ts` (the old source tree layout). Change to `./mod.ts`.

### PSA security context pattern
Use the same security context pattern as the main workflow container: `runAsNonRoot: true`, `runAsUser: 65534` (nobody), `allowPrivilegeEscalation: false`, `capabilities: {drop: ["ALL"]}`. Apply to both proxy-prewarm initContainer and CronJob trigger pod spec.

### Trigger egress NetworkPolicy
CronJob trigger pods need to reach the workflow Service on port 8080. Generate a NetworkPolicy with podSelector matching trigger pod labels and egress to the workflow Service. This follows the existing default-deny + allow pattern.

### Secrets check fix
The current code checks `if len(spec.Secrets) > 0` to decide whether to mount the secrets volume. With contract dependencies, secrets are derived from `spec.Contract.Dependencies` instead. Change the check to look at contract dependencies that have secrets.

## Design Decisions

### Dual trigger NetworkPolicy implementation (import cycle constraint)

There are two implementations of trigger NetworkPolicy generation:

1. `GenerateTriggerNetworkPolicy` in `pkg/k8s/netpol.go` — public, used by external callers such as `deploy.go`.
2. A private equivalent inside `pkg/builder/k8s.go` — used internally by `GenerateK8sManifests()`.

Both produce equivalent output. The duplication exists because `pkg/builder` imports `pkg/spec` and `pkg/k8s` imports `pkg/builder` (for the `builder.Manifest` type). If `pkg/builder` were to import `pkg/k8s`, it would create a circular import. The correct long-term fix is to extract the `Manifest` type into a separate `pkg/types` package so both `pkg/builder` and `pkg/k8s` can import it without a cycle, but that refactoring is out of scope for this change.

## Risks / Trade-offs

- **Trigger NetworkPolicy adds another manifest**: Small increase in generated manifest count. Acceptable given the security benefit of explicit egress rules.
- **PSA security context on curl image**: `curlimages/curl` runs as non-root by default, so adding explicit securityContext is redundant but required for PSA validation.
