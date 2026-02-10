# Node Contract

Every node is a TypeScript file with a single default export:

```typescript
import type { Context } from "pipedreamer";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const resp = await ctx.fetch("github", "/user/repos");
  ctx.log.info("Fetched repos");
  return { repos: await resp.json() };
}
```

## Context API

| Member | Type | Description |
|--------|------|-------------|
| `ctx.fetch(service, path, init?)` | `Promise<Response>` | HTTP request to `https://api.<service>.com<path>` with automatic auth injection from secrets |
| `ctx.log` | `Logger` | Structured logging (`info`, `warn`, `error`, `debug`) prefixed with `[nodeId]` |
| `ctx.config` | `Record<string, unknown>` | Workflow-level config from `config:` in workflow.yaml |
| `ctx.secrets` | `Record<string, Record<string, string>>` | Secrets keyed by service name |

## Auth Injection

`ctx.fetch` automatically adds authorization headers based on matching secrets:

- `secrets[service].token` → `Authorization: Bearer <token>`
- `secrets[service].api_key` → `X-API-Key: <api_key>`

## Testing Nodes

Create a JSON fixture at `tests/fixtures/<node-name>.json`:

```json
{
  "input": { "query": "test" },
  "expected": { "results": [] }
}
```

Run with `pipedreamer test` (all nodes) or `pipedreamer test my-workflow/node-name` (single node).

## Mock Context

The engine provides a mock context for testing (`engine/testing/mocks.ts`). The mock `ctx.fetch` returns `{ mock: true, service, path }` — test fixtures must match this format.

See [architecture.md](architecture.md) for the full context system design including the module loader and import map.
