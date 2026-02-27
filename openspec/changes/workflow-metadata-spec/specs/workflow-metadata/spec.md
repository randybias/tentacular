## ADDED Requirements

### Requirement: Workflow spec supports optional metadata section
The workflow YAML spec SHALL support an optional `metadata:` section at the top level of `workflow.yaml`. When omitted, the workflow SHALL parse and function identically to today.

#### Scenario: Workflow with full metadata
- **WHEN** a workflow.yaml contains a `metadata:` section with owner, team, tags, and environment
- **THEN** the parser SHALL populate the `Metadata` field on the `Workflow` struct with all provided values

#### Scenario: Workflow without metadata
- **WHEN** a workflow.yaml does not contain a `metadata:` section
- **THEN** the `Metadata` field on the `Workflow` struct SHALL be nil and all other parsing SHALL behave unchanged

### Requirement: Metadata owner field
The `metadata:` section SHALL support an optional `owner` field as a free-form string identifying the workflow owner (person, team, or group).

#### Scenario: Owner present
- **WHEN** `metadata.owner` is set to `"platform-team"`
- **THEN** `Workflow.Metadata.Owner` SHALL equal `"platform-team"`

#### Scenario: Owner absent
- **WHEN** `metadata.owner` is not present
- **THEN** `Workflow.Metadata.Owner` SHALL be the zero value (empty string)

### Requirement: Metadata team field
The `metadata:` section SHALL support an optional `team` field as a free-form string identifying the responsible team.

#### Scenario: Team present
- **WHEN** `metadata.team` is set to `"mcp-tracking"`
- **THEN** `Workflow.Metadata.Team` SHALL equal `"mcp-tracking"`

#### Scenario: Team absent
- **WHEN** `metadata.team` is not present
- **THEN** `Workflow.Metadata.Team` SHALL be the zero value (empty string)

### Requirement: Metadata tags field
The `metadata:` section SHALL support an optional `tags` field as a list of strings for categorization and filtering.

#### Scenario: Tags present
- **WHEN** `metadata.tags` contains `["production", "reporting"]`
- **THEN** `Workflow.Metadata.Tags` SHALL be a string slice with those values in order

#### Scenario: Tags absent
- **WHEN** `metadata.tags` is not present
- **THEN** `Workflow.Metadata.Tags` SHALL be nil

### Requirement: Metadata environment field
The `metadata:` section SHALL support an optional `environment` field as a free-form string indicating the target environment (e.g., "dev", "staging", "prod").

#### Scenario: Environment present
- **WHEN** `metadata.environment` is set to `"dev"`
- **THEN** `Workflow.Metadata.Environment` SHALL equal `"dev"`

#### Scenario: Environment absent
- **WHEN** `metadata.environment` is not present
- **THEN** `Workflow.Metadata.Environment` SHALL be the zero value (empty string)
