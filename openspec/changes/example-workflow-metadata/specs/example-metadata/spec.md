## ADDED Requirements

### Requirement: Example workflow includes metadata section
The `sep-tracker` example workflow SHALL include a `metadata:` section demonstrating all available metadata fields.

#### Scenario: sep-tracker has complete metadata
- **WHEN** a user reads `example-workflows/sep-tracker/workflow.yaml`
- **THEN** the file SHALL contain a `metadata:` section with `owner`, `team`, `tags`, and `environment` fields populated with realistic example values

### Requirement: Example metadata parses without errors
The example workflow with metadata SHALL parse successfully with the existing workflow parser.

#### Scenario: Parse example with metadata
- **WHEN** `sep-tracker/workflow.yaml` is loaded by the spec parser
- **THEN** parsing SHALL succeed and `Workflow.Metadata` SHALL be non-nil with all fields populated
