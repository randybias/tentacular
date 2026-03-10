# Exoskeleton Phase 1 - CLI (tentacular)

## Problem
The CLI's spec parser requires all protocol-specific fields (host, port, database, etc.) for dependencies. Exoskeleton-managed dependencies (tentacular-*) have these fields filled by the MCP server at deploy time, so the CLI must not require them.

## Solution
1. Skip protocol-specific field validation for deps named `tentacular-*`
2. Add `s3` as a valid protocol alias alongside `blob`
3. Treat `tentacular-*` deps as no-op in `DeriveEgressRules` and `DeriveDenoFlags` (MCP enriches these)

## Scope
- `pkg/spec/parse.go` - validation skip + s3 protocol
- `pkg/spec/derive.go` - skip tentacular-* in egress and deno flag derivation
- Unit tests for all changes
