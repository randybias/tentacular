# CANCELLED: tntc promote command

> **Status: Cancelled.** Promotion is an agent workflow concept, not a CLI
> command. The agent uses the existing deploy flow with per-env MCP config
> (Phase 2): `tntc deploy --env dev` -> verify health -> `tntc deploy --env prod`.
> The skill docs (tentacular-skill) teach the agent this promotion pattern instead.

## Original Proposal (for reference)

A dedicated `tntc promote --from dev --to prod` command was proposed to make
environment promotion explicit. After review, this was determined to be
unnecessary because:

1. Per-env MCP config (Phase 2) already enables deploying to any environment.
2. The existing `tntc deploy --env <target>` command does the same thing.
3. Promotion is a workflow pattern the agent follows, not a CLI primitive.
4. The skill docs are the right place to encode this pattern.
