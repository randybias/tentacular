## ADDED Requirements

### Requirement: wf_describe MCP tool returns workflow detail
The MCP server SHALL expose a `wf_describe` tool that returns deep detail for a single workflow identified by name and namespace.

#### Scenario: Describe a workflow with annotations and ConfigMap
- **WHEN** `wf_describe` is called with `name: "sep-tracker"` and `namespace: "pd-sep-tracker"`
- **THEN** it SHALL return metadata from Deployment annotations plus node names and trigger descriptions enriched from the `-code` ConfigMap

#### Scenario: Describe a workflow with missing ConfigMap
- **WHEN** `wf_describe` is called and the `-code` ConfigMap does not exist
- **THEN** it SHALL return annotation-based metadata without nodes or triggers (best-effort, non-fatal)

### Requirement: wf_describe returns structured result
The `wf_describe` result SHALL include: name, namespace, version, owner, team, tags (string slice), environment, ready (bool), replicas, ready_replicas, image, age, nodes (string slice of node names), triggers (string slice of trigger descriptions), and annotations (map of tentacular.dev/* annotations).

#### Scenario: Full result structure
- **WHEN** `wf_describe` returns successfully with both annotations and ConfigMap available
- **THEN** the result SHALL include all annotation fields plus `nodes` (names from ConfigMap), `triggers` (type + schedule descriptions from ConfigMap), and `annotations` (filtered to tentacular.dev/* keys only)

### Requirement: wf_describe requires name and namespace
The `wf_describe` tool SHALL require both `name` and `namespace` parameters.

#### Scenario: Missing namespace
- **WHEN** `wf_describe` is called without a `namespace` parameter
- **THEN** it SHALL return an error via guard namespace check
