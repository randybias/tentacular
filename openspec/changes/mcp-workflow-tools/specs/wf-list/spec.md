## ADDED Requirements

### Requirement: wf_list MCP tool lists deployed workflows
The MCP server SHALL expose a `wf_list` tool that returns a list of all tentacular-managed workflows by querying K8s Deployments with label `app.kubernetes.io/managed-by: tentacular`.

#### Scenario: List all workflows across namespaces
- **WHEN** `wf_list` is called with no filters
- **THEN** it SHALL return all tentacular-managed Deployments across all namespaces, each with name, namespace, version, owner, team, environment, ready status, and age extracted from labels and annotations

#### Scenario: Filter by namespace
- **WHEN** `wf_list` is called with `namespace: "production"`
- **THEN** it SHALL return only workflows deployed in the `production` namespace

### Requirement: wf_list supports tag filtering
The `wf_list` tool SHALL accept an optional `tag` parameter to filter workflows by tag.

#### Scenario: Filter by tag
- **WHEN** `wf_list` is called with `tag: "reporting"`
- **THEN** it SHALL return only workflows whose `tentacular.dev/tags` annotation contains `"reporting"` in its comma-separated list

### Requirement: wf_list supports owner filtering
The `wf_list` tool SHALL accept an optional `owner` parameter to filter workflows by owner.

#### Scenario: Filter by owner
- **WHEN** `wf_list` is called with `owner: "platform-team"`
- **THEN** it SHALL return only workflows whose `tentacular.dev/owner` annotation equals `"platform-team"`

### Requirement: wf_list returns structured result
Each workflow entry in the `wf_list` result SHALL include: name, namespace, version, owner, team, environment, ready (bool), and age (duration string).

#### Scenario: Result structure
- **WHEN** `wf_list` returns a workflow entry
- **THEN** the entry SHALL be a JSON object with fields: `name`, `namespace`, `version`, `owner`, `team`, `environment`, `ready`, `age`
