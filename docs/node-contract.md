# Node Contract

Every node is a TypeScript file with a single default export:

```typescript
import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const gh = ctx.dependency("github-api");
  const resp = await gh.fetch!("/user/repos", {
    headers: { "Authorization": `Bearer ${gh.secret}` },
  });
  ctx.log.info("Fetched repos");
  return { repos: await resp.json() };
}
```

## Context API

| Member | Type | Description |
|--------|------|-------------|
| `ctx.dependency(name)` | `(string) => DependencyConnection` | **Primary API.** Returns connection metadata and resolved secret for a declared contract dependency. HTTPS deps include `fetch(path, init?)` URL builder (no auth injection). |
| `ctx.log` | `Logger` | Structured logging (`info`, `warn`, `error`, `debug`) prefixed with `[nodeId]` |
| `ctx.config` | `Record<string, unknown>` | Workflow-level config from `config:` in workflow.yaml. Business-logic only. |
| `ctx.fetch(service, path, init?)` | `Promise<Response>` | **Legacy.** Flagged as contract violation when contract is present. Use `ctx.dependency()` instead. |
| `ctx.secrets` | `Record<string, Record<string, string>>` | **Legacy.** Flagged as contract violation when contract is present. Use `ctx.dependency().secret` instead. |

## Auth Pattern

`dep.fetch()` builds the URL but does not inject auth. Nodes handle auth explicitly using `dep.secret` and `dep.authType`:

```typescript
const gh = ctx.dependency("github-api");
// gh.authType is any string (e.g., "bearer-token", "api-key", "hmac-sha256")
const resp = await gh.fetch!("/repos/owner/repo", {
  headers: { "Authorization": `Bearer ${gh.secret}` },
});
```

## Testing Nodes

Create a JSON fixture at `tests/fixtures/<node-name>.json`:

```json
{
  "input": { "query": "test" },
  "expected": { "results": [] }
}
```

Run with `tntc test` (all nodes) or `tntc test my-workflow/node-name` (single node).

## Mock Context

The engine provides a mock context for testing (`engine/testing/mocks.ts`). Mock `ctx.dependency()` returns metadata with mock secret values and records access for drift detection. HTTPS mock deps return `{ mock: true, dependency, path }` from `fetch()`.

See [architecture.md](architecture.md) for the full context system design including the module loader and import map.
