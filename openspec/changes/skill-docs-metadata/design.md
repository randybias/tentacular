## Context

The tentacular-skill repo provides SKILL.md (used by AI assistants for workflow development guidance) and references/workflow-spec.md (complete spec reference). Both need updating to include the metadata section and new MCP tools.

## Goals / Non-Goals

**Goals:**
- Add `metadata` to the top-level fields table in workflow-spec.md
- Add a Metadata subsection with field descriptions and example YAML
- Document `wf_list` and `wf_describe` in SKILL.md MCP tools section

**Non-Goals:**
- Rewriting existing documentation
- Adding tutorials or guides (reference docs only)

## Decisions

### Minimal, targeted edits
Add metadata to the existing table and add a new subsection. Do not reorganize or rewrite existing content. This keeps the diff small and review easy.

### Include complete YAML example
Show a metadata block with all fields populated so users can copy-paste.

## Risks / Trade-offs

- **Docs may lag code** -- Mitigated by making this a blocking Phase 5 that depends on Phases 1-3.
