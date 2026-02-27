## Context

K8s Deployments currently carry only `app.kubernetes.io/*` labels. MCP tools must parse the `-code` ConfigMap to get workflow details. Annotations are the standard K8s mechanism for attaching non-identifying metadata to resources, and are queryable via the K8s API.

## Goals / Non-Goals

**Goals:**
- Stamp metadata fields as annotations on generated Deployment and Service manifests
- Only emit annotations for non-empty values
- Return empty string when metadata is nil (no annotations block at all)

**Non-Goals:**
- Derived/structural annotations (node-count, pipeline, triggers, dependencies) -- deferred
- Annotations on CronJob or ConfigMap resources
- Annotation-based routing or policy enforcement

## Decisions

### tentacular.dev/ annotation prefix
Use `tentacular.dev/` as the annotation key prefix. This follows K8s convention for domain-scoped annotations and avoids collision with other tools.

### Metadata passthrough only (no derived annotations)
Only stamp fields directly from `WorkflowMetadata`: owner, team, tags, environment. Structural info (node count, pipeline, triggers, dependencies) is available via the `-code` ConfigMap and `wf_describe` MCP tool. This keeps the annotation builder simple and deterministic.

Alternative: Also derive annotations from triggers, edges, contract -- adds complexity and non-determinism (e.g., deployed-at timestamps) with limited benefit since `wf_describe` already parses the ConfigMap.

### Annotations on both Deployment and Service
Both resources get the annotation block so MCP tools can read metadata from either resource type.

### String-only annotation values
All values are strings (K8s requirement). Tags use comma separation. This keeps parsing simple for MCP consumers.

## Risks / Trade-offs

- **Annotation size limits** -- K8s limits total annotation size to 256KB. Four string fields are trivially small.
- **No structural annotations** -- MCP tools must parse ConfigMap for node/edge/trigger detail. Acceptable since `wf_describe` already does this.
