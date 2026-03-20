## ADDED Requirements

### Requirement: Deploy supports --group flag
The `tntc deploy` command SHALL accept a `--group` flag that sets the initial group for the deployed tentacle.

#### Scenario: Deploy with explicit group
- **WHEN** the user runs `tntc deploy --group platform-team`
- **THEN** the CLI SHALL pass group=platform-team in WfApplyParams to the MCP server

#### Scenario: Deploy without --group uses server default
- **WHEN** the user runs `tntc deploy` without --group
- **THEN** the CLI SHALL not set the group field, and the MCP server SHALL apply its default

### Requirement: Deploy supports --share flag
The `tntc deploy` command SHALL accept a `--share` flag that sets the mode to the Team preset (0750).

#### Scenario: Deploy with --share
- **WHEN** the user runs `tntc deploy --share`
- **THEN** the CLI SHALL pass the Team preset mode (0750) in WfApplyParams

### Requirement: WfApplyParams includes Group and Share fields
The WfApplyParams struct in `pkg/mcp/tools.go` SHALL include Group (string) and Share (bool) fields.

#### Scenario: Fields serialized in MCP request
- **WHEN** the CLI sends a wf_apply request with group and share set
- **THEN** the JSON payload SHALL include the group and share fields

### Requirement: Builder uses tentacular.io annotations
The builder in `pkg/builder/k8s.go` SHALL use `tentacular.io/*` annotation keys instead of `tentacular.dev/*`.

#### Scenario: Built manifests use new annotation namespace
- **WHEN** the builder generates Kubernetes manifests
- **THEN** all tentacular annotations SHALL use the `tentacular.io/` prefix
