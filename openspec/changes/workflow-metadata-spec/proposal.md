## Why

Workflow metadata (owner, team, tags, environment) is not part of the workflow spec today. MCP tools that list and describe workflows have no structured data to return beyond name/version/description. Adding an optional `metadata` section to the workflow spec enables rich, filterable workflow catalogs without breaking existing workflows.

## What Changes

- Add `WorkflowMetadata` struct to `pkg/spec/types.go` with fields: Owner, Team, Tags, Environment
- Add `Metadata *WorkflowMetadata` field to the `Workflow` struct (pointer, omitempty -- fully optional)
- Parse and validate metadata during workflow loading (tags as string slice)
- No changes to existing fields or behavior -- all metadata fields are optional

## Capabilities

### New Capabilities
- `workflow-metadata`: Optional metadata section in workflow.yaml supporting owner, team, tags, and environment. All fields optional, backwards-compatible.

### Modified Capabilities
<!-- None -- this is purely additive -->

## Impact

- `pkg/spec/types.go`: New `WorkflowMetadata` struct, new field on `Workflow`
- `pkg/spec/parse.go`: Metadata deserialization (automatic via YAML tags, optional validation)
- `pkg/spec/parse_test.go`: Tests for metadata parsing with and without metadata present
- Example workflows: Can optionally add metadata sections (separate change)
