## Context

The `tntc` CLI communicates with the MCP server for all cluster operations. It currently has no permissions-related commands and uses `tentacular.dev/*` annotations in the builder. The MCP server is adding authz enforcement and new permissions tools that the CLI must surface.

## Goals / Non-Goals

**Goals:**
- Add `tntc permissions` command group with get/set/chmod/chgrp subcommands
- Add --group and --share flags to `tntc deploy`
- Extend `tntc whoami` to display group membership
- Migrate annotation references in the builder

**Non-Goals:**
- Implementing authz evaluation logic in the CLI (that's MCP server's job)
- Local permission caching (CLI always queries MCP server)
- Interactive permission editor

## Decisions

### 1. Permissions as a top-level command group

**Rationale:** `tntc permissions get/set/chmod/chgrp` follows the pattern of existing command groups (cluster, catalog). chmod/chgrp are convenience wrappers around set with familiar POSIX names.

**Alternative considered:** `tntc authz` as the command name. Rejected because "permissions" is more user-friendly and matches what the commands actually do.

### 2. chmod/chgrp as aliases for set

**Rationale:** Users familiar with POSIX systems expect `chmod` and `chgrp` commands. These are thin wrappers: `tntc permissions chmod 0750 my-tentacle` calls permissions_set with mode=0750, and `tntc permissions chgrp platform-team my-tentacle` calls permissions_set with group=platform-team.

### 3. --share flag as a convenience for --group + mode preset

**Rationale:** `tntc deploy --share` is shorthand for setting mode to the Team preset (0750). This covers the most common sharing use case without requiring the user to know numeric modes.

## Risks / Trade-offs

- **[MCP server dependency]** permissions commands fail if MCP server doesn't have permissions tools. -> Mitigation: CLI shows a clear error if the tool is not available.
- **[Annotation migration in builder]** Changing annotation keys in the builder means tentacles deployed with the new CLI won't have old-format annotations. -> Mitigation: MCP server reads both formats, so mixed deployments work.
