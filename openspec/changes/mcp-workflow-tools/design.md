## Context

The tentacular-mcp server already has workflow tools (`wf_pods`, `wf_logs`, `wf_events`, `wf_trigger`) registered via `registerWorkflowTools()`. The new discovery tools are in a separate `discover.go` file with `registerDiscoverTools()` to keep the tool groups distinct. Test stubs exist in `workflow_meta_test.go`.

## Goals / Non-Goals

**Goals:**
- `wf_list`: Scan Deployments across namespaces with `app.kubernetes.io/managed-by: tentacular` label
- `wf_describe`: Deep-dive a single workflow by reading annotations + `-code` ConfigMap
- Support filtering by namespace, tag, owner in `wf_list`
- Follow existing MCP tool handler pattern (params struct, result struct, guard checks)

**Non-Goals:**
- Workflow modification or management via MCP
- Cross-cluster workflow discovery
- Returning raw workflow.yaml or node source code (too large for MCP responses)

## Decisions

### Separate registerDiscoverTools() registration
Discovery tools (`wf_list`, `wf_describe`) are registered via `registerDiscoverTools()` in `discover.go`, separate from existing `registerWorkflowTools()`. This keeps operational tools (pods, logs, events, trigger) distinct from discovery/catalog tools.

Alternative: Add to `registerWorkflowTools()` -- mixes concerns.

### Annotation-first for wf_list, ConfigMap enrichment for wf_describe
`wf_list` reads only Deployment labels and annotations for fast listing. `wf_describe` additionally parses the `-code` ConfigMap best-effort to extract node names and trigger descriptions. ConfigMap parsing is non-fatal -- if missing, annotation-based data is still returned.

Alternative: Parse ConfigMap for every workflow in list -- too slow.

### wf_list returns operational status
Each list entry includes `ready` (bool) and `age` (duration string) derived from Deployment status, in addition to metadata fields. This gives MCP clients a quick health overview.

### Filter by annotation values (client-side)
Tag and owner filtering in `wf_list` checks `tentacular.dev/tags` and `tentacular.dev/owner` annotation values. Tag matching uses `containsTag()` helper to check comma-separated list membership. K8s does not support annotation-based label selectors so filtering is client-side.

### Namespace parameter behavior
If namespace is empty, `wf_list` lists across all namespaces. If specified, scoped to that namespace only with guard check.

## Risks / Trade-offs

- **Client-side filtering** -- Large clusters with many workflows may be slow. Acceptable for expected scale (tens of workflows, not thousands).
- **ConfigMap parsing** -- `wf_describe` assumes `-code` ConfigMap exists and contains `workflow.yaml`. Graceful error if missing.
