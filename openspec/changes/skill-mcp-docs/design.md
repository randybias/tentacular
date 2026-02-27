## Context

The tentacular-skill SKILL.md is the primary reference for AI assistants using tentacular. It currently documents `tntc` CLI commands as the only interaction model. After the refactoring, AI assistants have two interfaces: `tntc` CLI for build-time operations and MCP tools for operational tasks.

## Goals / Non-Goals

**Goals:**
- Document the CLI (build/ship) vs MCP (operate/enforce) boundary clearly.
- Add MCP tool reference: tool names, parameters, return types, usage examples.
- Update the deployment workflow section to reflect MCP-routed deploys.
- Keep the document practical and action-oriented for AI assistants.

**Non-Goals:**
- Documenting MCP protocol internals.
- Providing MCP server installation instructions (that is in tentacular CLI docs).
- Documenting the Go API of pkg/mcp/.

## Decisions

### Separate sections for CLI and MCP
SKILL.md gets two major sections: "Build & Ship (CLI)" and "Operate & Monitor (MCP Tools)". This mirrors the architectural boundary and helps AI assistants choose the right tool.

### MCP tool reference as a subsection
Rather than a separate reference document, MCP tools are documented inline in SKILL.md. Each tool gets: name, description, parameters, example usage. This keeps everything in one file for AI assistant consumption.

### Workflow lifecycle documentation
A new "Workflow Lifecycle" section shows the complete flow: init -> validate -> test -> build -> deploy (via MCP) -> run (via MCP) -> monitor (via MCP). This replaces the current linear CLI-only workflow.

## Risks / Trade-offs

- **Documentation drift**: As MCP tools evolve, SKILL.md must be kept in sync. Mitigate by updating docs as part of every tool change.
- **Document size**: Adding MCP content increases SKILL.md size. Keep descriptions concise and use examples over explanations.
