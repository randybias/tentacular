## Why

The tentacular-skill SKILL.md and references/workflow-spec.md must reflect the new `metadata:` section so that AI assistants using the skill have accurate, up-to-date workflow spec documentation. Without this update, the skill will not know about metadata fields or the new MCP tools.

## What Changes

- Update `../tentacular-skill/references/workflow-spec.md` to document the `metadata` top-level field and its sub-fields (owner, team, tags, environment)
- Update `../tentacular-skill/SKILL.md` to mention the new `wf_list` and `wf_describe` MCP tools in the relevant section
- Add example YAML snippet showing metadata usage

## Capabilities

### New Capabilities
- `skill-metadata-docs`: Documentation updates to tentacular-skill for metadata section and new MCP tools.

### Modified Capabilities
<!-- None -->

## Impact

- `../tentacular-skill/references/workflow-spec.md`: Add metadata section to top-level fields table and add detailed metadata subsection
- `../tentacular-skill/SKILL.md`: Add wf_list and wf_describe to MCP tools documentation
- Depends on Phases 1-3 being finalized (metadata spec, annotations, MCP tools)
