## ADDED Requirements

### Requirement: Deployment and Service manifests include tentacular.dev annotations
Generated K8s Deployment and Service manifests SHALL include `tentacular.dev/*` annotations in their metadata section when workflow metadata is present. Annotations SHALL only be emitted for non-empty values.

#### Scenario: Workflow with full metadata generates annotations
- **WHEN** a workflow has metadata with owner, team, tags, and environment
- **THEN** the Deployment and Service manifests SHALL include annotations: `tentacular.dev/owner`, `tentacular.dev/team`, `tentacular.dev/tags`, `tentacular.dev/environment`

#### Scenario: Workflow without metadata generates no annotations block
- **WHEN** a workflow has no `metadata:` section (nil Metadata)
- **THEN** `buildDeployAnnotations()` SHALL return an empty string and no annotations block SHALL appear in the manifests

### Requirement: Annotation values are derived from WorkflowMetadata fields
Each annotation SHALL be derived from `WorkflowMetadata` fields as follows:

- `tentacular.dev/owner`: from `meta.Owner`
- `tentacular.dev/team`: from `meta.Team`
- `tentacular.dev/tags`: comma-separated from `meta.Tags`
- `tentacular.dev/environment`: from `meta.Environment`

#### Scenario: Tags annotation formatting
- **WHEN** a workflow has tags `["production", "reporting"]`
- **THEN** `tentacular.dev/tags` SHALL equal `"production,reporting"`

### Requirement: Empty values are omitted
Annotations with empty string values SHALL NOT be included in the generated annotation block.

#### Scenario: No metadata owner
- **WHEN** `meta.Owner` is empty
- **THEN** the `tentacular.dev/owner` annotation SHALL NOT appear in the manifest

#### Scenario: Partial metadata
- **WHEN** only `meta.Owner` and `meta.Tags` are set (team and environment empty)
- **THEN** only `tentacular.dev/owner` and `tentacular.dev/tags` annotations SHALL appear
