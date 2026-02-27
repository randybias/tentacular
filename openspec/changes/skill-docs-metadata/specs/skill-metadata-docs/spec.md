## ADDED Requirements

### Requirement: workflow-spec.md documents metadata section
The workflow spec reference SHALL include the `metadata` field in the top-level fields table and SHALL have a dedicated Metadata subsection describing all sub-fields.

#### Scenario: Top-level fields table includes metadata
- **WHEN** a user reads the top-level fields table in workflow-spec.md
- **THEN** the table SHALL include a row for `metadata` with type `object`, required `No`, and description indicating optional workflow metadata

#### Scenario: Metadata subsection documents all fields
- **WHEN** a user reads the Metadata subsection
- **THEN** it SHALL document `owner` (string), `team` (string), `tags` (string array), and `environment` (string) with descriptions and an example YAML block

### Requirement: SKILL.md documents wf_list and wf_describe tools
The SKILL.md file SHALL document the `wf_list` and `wf_describe` MCP tools in its MCP tools section.

#### Scenario: wf_list documented
- **WHEN** a user reads the MCP tools section of SKILL.md
- **THEN** it SHALL include `wf_list` with its parameters (namespace, tag, owner) and return fields

#### Scenario: wf_describe documented
- **WHEN** a user reads the MCP tools section of SKILL.md
- **THEN** it SHALL include `wf_describe` with its parameters (name, namespace) and return fields
