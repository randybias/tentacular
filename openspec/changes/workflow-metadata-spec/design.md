## Context

The `Workflow` struct in `pkg/spec/types.go` currently has Name, Version, Description, Triggers, Nodes, Edges, Config, Deployment, and Contract. There is no structured metadata for ownership, categorization, or discoverability. MCP tools need this data to provide rich workflow listings and filtering.

## Goals / Non-Goals

**Goals:**
- Add an optional `metadata` section to the workflow spec with owner, team, tags, and environment
- Maintain full backwards compatibility -- workflows without metadata continue to work unchanged
- Provide structured data that downstream consumers (MCP tools, annotations) can use

**Non-Goals:**
- Metadata validation beyond basic type checks (no registry of valid tags)
- Metadata-based access control or policy enforcement
- Schema versioning for the metadata block itself

## Decisions

### Pointer field with omitempty
`Metadata *WorkflowMetadata` uses a pointer so zero-value (nil) is cleanly omitted in YAML output and clearly distinguishes "not present" from "present but empty." Alternative: embedded struct -- would serialize empty fields.

### Flat struct, no nesting
All metadata fields are top-level within the `metadata:` block. Tags is a string slice, all other fields are plain strings. Alternative: nested sub-objects for contacts/links -- unnecessary complexity for this use case.

### Operational fields: team and environment
Team and environment are the most useful metadata for MCP filtering and operational context. Alternative: category, repository, links -- deferred; can be added later if needed.

## Risks / Trade-offs

- **Inconsistent metadata across workflows** -- No enforcement of required fields. Mitigated by documentation and examples showing best practices.
- **Tag proliferation** -- Free-form tags may diverge across teams. Mitigated by MCP tools showing existing tags for consistency.
