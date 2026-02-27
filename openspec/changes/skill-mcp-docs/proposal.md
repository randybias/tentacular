## Why

The tentacular-skill SKILL.md teaches AI assistants how to use `tntc` CLI commands. After the refactoring, the operational model changes: the MCP server is the control plane, and the CLI is the data plane. The skill documentation needs to reflect this new architecture so AI assistants understand the correct interaction model.

## What Changes

- Update SKILL.md in tentacular-skill to document the MCP server as the primary interface for cluster operations.
- Add a section explaining the CLI (build/ship) vs MCP server (operate/enforce) boundary.
- Document that AI assistants should prefer MCP tools (`wf_list`, `wf_describe`, `wf_run`, `wf_deploy`, etc.) for operational tasks.
- Document the `tntc` CLI commands that remain relevant (init, validate, test, build, dev) as the build-time workflow.
- Update deployment instructions to reflect MCP-routed deploy flow.
- Add MCP tool reference section listing available tools and their parameters.

## Capabilities

### New Capabilities
- Skill documentation for MCP-based operational model.
- MCP tool reference section in SKILL.md.

### Modified Capabilities
- Updated deployment workflow documentation reflecting MCP routing.
- Updated command reference distinguishing build-time (CLI) from operate-time (MCP) commands.

## Impact

- `../tentacular-skill/SKILL.md`: Major update to reflect MCP-first operational model.
- `../tentacular-skill/references/`: May add MCP tool reference documents.
- Depends on all prior phases being finalized so documentation is accurate.
