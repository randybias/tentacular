## Context

Generated Deployment manifests in `pkg/builder/k8s.go` already include pod-level and container-level security hardening (runAsNonRoot, seccompProfile, readOnlyRootFilesystem, drop ALL capabilities). However, the default ServiceAccount token is still automounted. Workflow pods never call the Kubernetes API, so this token is an unnecessary attack surface.

## Goals / Non-Goals

**Goals:**
- Eliminate unnecessary Kubernetes API access from workflow pods by disabling ServiceAccount token automount

**Non-Goals:**
- Changing ServiceAccount assignment (we rely on the default SA)
- Adding RBAC policies for workflow pods (not needed since they never call the API)

## Decisions

1. **Add `automountServiceAccountToken: false` at pod spec level** -- This is the standard K8s field for disabling token mount. Placing it at pod spec level (not container level) is the correct location per the K8s API. It goes alongside the existing `securityContext` block.

## Risks / Trade-offs

- [Risk] Future use cases may need K8s API access from workflow pods. --> Mitigation: This is a template change in Go code; it can be reverted or made conditional via `DeployOptions` if needed. Current design philosophy is that workflows are sealed pods with declared dependencies only.
