## Context

The Deployment template in `pkg/builder/k8s.go` creates an emptyDir volume for `/tmp` with no size limit. Deno requires a writable `/tmp` directory, but the volume should be bounded to prevent resource exhaustion on the node.

## Goals / Non-Goals

**Goals:**
- Bound the `/tmp` emptyDir volume to 512Mi to prevent unbounded disk usage

**Non-Goals:**
- Making the size limit configurable (can be added later via `DeployOptions` if needed)
- Changing the mount path or permissions

## Decisions

1. **512Mi size limit** -- Generous for temporary file operations (Deno cache artifacts, temp data), while preventing multi-GB storage abuse. This is a safe default; it can be made configurable later if workflows need more.

2. **YAML format change** -- The volume definition changes from single-line `emptyDir: {}` to multi-line format with `sizeLimit` nested under `emptyDir`.

## Risks / Trade-offs

- [Risk] Some workflows may legitimately need more than 512Mi of temp storage. --> Mitigation: 512Mi is generous for typical workflow operations. If needed, can be made configurable via `DeployOptions` in a future change.
