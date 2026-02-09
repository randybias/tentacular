## Why

Pipedreamer v2 has all the core components (engine, compiler, executor, context, testing framework, CLI commands) but no reference workflow that demonstrates how they work together. Without a working end-to-end example, developers have no concrete starting point, and the project cannot validate that `pipedreamer validate`, `pipedreamer dev`, and `pipedreamer test` actually work with a real workflow. The github-digest example serves as both documentation and integration smoke test.

## What Changes

- **New `examples/github-digest/` workflow directory** containing a complete 3-node DAG that fetches GitHub repository data, summarizes it, and sends a notification
- **`fetch-repos` node** (`nodes/fetch-repos.ts`) — an external API source node that uses `ctx.fetch` to call the GitHub REST API, demonstrating capability declarations and secrets access for authentication
- **`summarize` node** (`nodes/summarize.ts`) — a pure transform node that takes the fetched repo data and produces a digest summary, demonstrating input/output data passing between nodes
- **`notify` node** (`nodes/notify.ts`) — a sink node that uses `ctx.fetch` to send the digest to a webhook endpoint (e.g., Slack), demonstrating the final stage of a pipeline
- **`workflow.yaml`** — a complete v2 workflow spec with name, version, triggers (manual + cron), 3 nodes with capability declarations, edges forming a linear DAG (`fetch-repos -> summarize -> notify`), and workflow config (timeout, retries)
- **Test fixtures** (`tests/fixtures/fetch-repos.json`, `tests/fixtures/summarize.json`) — JSON files with `{ "input": ..., "expected": ... }` format for node-level testing via `pipedreamer test`
- **Documentation comments** in all TypeScript files showing idiomatic node authoring patterns (typed inputs/outputs, context usage, error handling)

## Capabilities

### New Capabilities
- `example-workflow`: Reference github-digest workflow with 3 TypeScript nodes (source, transform, sink), test fixtures, workflow.yaml, and documentation comments demonstrating all v2 features

### Modified Capabilities
_(none — this adds example content, no engine or CLI requirement changes)_

## Impact

- **New files**: `examples/github-digest/workflow.yaml`, `examples/github-digest/nodes/fetch-repos.ts`, `examples/github-digest/nodes/summarize.ts`, `examples/github-digest/nodes/notify.ts`, `examples/github-digest/tests/fixtures/fetch-repos.json`, `examples/github-digest/tests/fixtures/summarize.json`
- **Dependencies**: No new dependencies; all nodes use the existing `pipedreamer` module import and Deno std library
- **Verification**: `pipedreamer validate examples/github-digest` must pass, `pipedreamer test examples/github-digest` must pass with fixtures, `pipedreamer dev examples/github-digest` must start the dev server
- **Developer experience**: Serves as the canonical "how to build a workflow" reference for onboarding
