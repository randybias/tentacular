## Why

MCP clients (Claude, other AI assistants) interacting with tentacular have no way to list or inspect deployed workflows. They must know exact namespaces and pod names. Adding `wf_list` and `wf_describe` MCP tools lets AI assistants discover and reason about workflows using the annotations stamped in Phase 2.

## What Changes

- Add `wf_list` MCP tool: lists all tentacular-managed workflows across namespaces by querying Deployments with `app.kubernetes.io/managed-by: tentacular` label. Supports filtering by namespace, tag, and owner via `tentacular.dev/*` annotations. Returns name, namespace, version, owner, team, environment, ready status, and age.
- Add `wf_describe` MCP tool: returns deep detail for a single workflow by name+namespace. Reads annotations for metadata, then best-effort parses the `-code` ConfigMap to extract node names and trigger descriptions. Returns owner, team, tags, environment, ready/replica status, image, age, nodes, triggers, and tentacular.dev annotations.
- Both tools live in `../tentacular-mcp/pkg/tools/discover.go` with separate `registerDiscoverTools()` registration.

## Capabilities

### New Capabilities
- `wf-list`: MCP tool to list all deployed workflows across managed namespaces with optional filtering by namespace, tag, and owner.
- `wf-describe`: MCP tool to return full workflow detail by name and namespace, parsing both Deployment annotations and the -code ConfigMap.

### Modified Capabilities
<!-- None -->

## Impact

- `../tentacular-mcp/pkg/tools/discover.go`: New file with handler functions, params/result structs, and `registerDiscoverTools()`
- `../tentacular-mcp/pkg/tools/workflow_meta_test.go`: Tests
- `../tentacular-mcp/pkg/tools/register.go`: Register via `registerDiscoverTools()`
- Depends on Phase 2 (stamp-deploy-annotations) for annotation availability on Deployments
