## Context

After Phases 1-5, `pkg/k8s/client.go` (24.9K) contains functions that are now handled by the MCP server. The package needs to be audited and slimmed down. Current files: `client.go`, `profile.go` (22.3K), `preflight.go` (8.2K), `netpol.go` (6.2K), `importmap.go` (11.6K), `kind.go` (2.3K).

## Goals / Non-Goals

**Goals:**
- Remove functions from `pkg/k8s/` that are fully replaced by MCP tools.
- Keep bootstrap functions (NewClient, loadConfig, direct Apply for MCP server deployment).
- Keep build-time functions (importmap resolution, manifest generation).
- Ensure all remaining callers compile and tests pass.

**Non-Goals:**
- Rewriting retained functions.
- Moving retained functions to different packages.
- Changing the public API of retained functions.

## Decisions

### Function-level removal audit
Each function in `pkg/k8s/` is categorized as:
- **Remove**: Replaced by MCP tool (e.g., GetPods -> wf_pods, GetLogs -> wf_logs).
- **Keep-bootstrap**: Needed before MCP server is running (e.g., Apply for MCP server deployment, namespace creation during bootstrap).
- **Keep-build**: Build-time operation (e.g., importmap resolution, netpol generation from contract).

### Test file cleanup
Test files for removed functions are also removed. Tests for retained functions are kept.

### No backwards compatibility shims
Removed functions are deleted completely. No deprecation warnings or re-export stubs. The refactoring is a clean break.

## Risks / Trade-offs

- **Missed callers**: Removing a function that is still called somewhere causes a compile error. Mitigate with thorough grep before removal.
- **Build-time vs deploy-time ambiguity**: Some functions (e.g., netpol generation) blur the line. Decision: if it generates manifests from workflow spec, it is build-time (keep). If it applies manifests to cluster, it is deploy-time (remove).
- **Test coverage gap**: Removing test files reduces coverage. Mitigate by ensuring MCP server tests cover the equivalent functionality.
