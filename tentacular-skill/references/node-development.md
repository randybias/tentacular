# Node Development Reference

Guide to writing Tentacular workflow nodes in TypeScript.

## Node Function Signature

Every node must default-export an async function:

```typescript
import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  // Process input, use context, return output
  return { result: "value" };
}
```

- `ctx` -- the Context object providing fetch, logging, config, and secrets.
- `input` -- data from upstream node(s) via edges. `{}` for root nodes (no incoming edges).
- Return value -- passed as input to downstream node(s) via edges.

The engine validates that the default export is a function at load time. If the export is missing or not a function, the engine throws an error.

## Context.fetch

```typescript
ctx.fetch(service: string, path: string, init?: RequestInit): Promise<Response>
```

Makes HTTP requests with automatic URL construction and auth injection.

### URL Construction

- If `path` starts with `http`, it is used as the full URL directly.
- Otherwise, the URL is constructed as `https://api.<service>.com<path>`.

```typescript
// Resolves to: https://api.github.com/repos/owner/repo/issues
const res = await ctx.fetch("github", "/repos/owner/repo/issues");

// Uses the full URL directly
const res = await ctx.fetch("custom", "https://my-service.internal/data");
```

### Auth Injection

Auth headers are injected automatically from `ctx.secrets[service]`:

| Secret Key | Header Added |
|-----------|-------------|
| `token` | `Authorization: Bearer <value>` |
| `api_key` | `X-API-Key: <value>` |

If the service has no entry in secrets, no auth headers are added.

```typescript
// If ctx.secrets.github.token = "ghp_abc123"
// then the request includes: Authorization: Bearer ghp_abc123
const res = await ctx.fetch("github", "/user");
```

### Request Options

The third parameter accepts standard `RequestInit` options:

```typescript
const res = await ctx.fetch("slack", "/api/chat.postMessage", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ channel: "#general", text: "Hello" }),
});
```

Custom headers are merged with injected auth headers. Auth headers are set first, so custom headers can override them if needed.

## Context.log

```typescript
ctx.log.info(msg: string, ...args: unknown[]): void
ctx.log.warn(msg: string, ...args: unknown[]): void
ctx.log.error(msg: string, ...args: unknown[]): void
ctx.log.debug(msg: string, ...args: unknown[]): void
```

Structured logging with automatic `[nodeId]` prefix on all output.

```typescript
ctx.log.info("fetching issues", { repo: "owner/repo" });
// Output: [fetch-issues] INFO fetching issues { repo: "owner/repo" }

ctx.log.error("request failed", error.message);
// Output: [fetch-issues] ERROR request failed 404 Not Found
```

Log levels map to console methods: `info` -> `console.log`, `warn` -> `console.warn`, `error` -> `console.error`, `debug` -> `console.debug`.

## Context.config

```typescript
ctx.config: Record<string, unknown>
```

Read-only access to the `config` section of workflow.yaml. Available to all nodes.

```yaml
# workflow.yaml
config:
  timeout: 60s
  retries: 1
```

```typescript
const timeout = ctx.config.timeout; // "60s"
const retries = ctx.config.retries; // 1
```

## Context.secrets

```typescript
ctx.secrets: Record<string, Record<string, string>>
```

Secrets are loaded from two sources (merged at startup):

1. **Local development**: `.secrets.yaml` file in the workflow directory.
2. **Production**: K8s Secret volume mounted at `/app/secrets`.

```yaml
# .secrets.yaml
github:
  token: "ghp_abc123"
slack:
  api_key: "xoxb-..."
```

```typescript
const githubToken = ctx.secrets.github?.token;     // "ghp_abc123"
const slackKey = ctx.secrets.slack?.api_key;        // "xoxb-..."
```

Secrets are also used by `ctx.fetch` for automatic auth injection (see above).

### Secrets Auto-Parsing (Production)

In production, secrets are mounted as individual files via a Kubernetes Secret volume at `/app/secrets`. The engine's `loadSecretsFromDir()` function (in `engine/context/secrets.ts`) reads each file and populates `ctx.secrets`:

1. Each file in the directory becomes a key in `ctx.secrets` (filename = service name).
2. Hidden files (names starting with `.`) are skipped.
3. Symlinks are followed (Kubernetes mounts Secret data as symlinks).
4. File content is parsed as JSON first. If it parses to an object, the object is used directly as the service's secrets map.
5. If the content is not valid JSON or parses to a non-object value (string, number, etc.), it is stored as `{ value: "<content>" }`.

```
/app/secrets/
  github    ->  {"token": "ghp_abc123"}           => ctx.secrets.github.token = "ghp_abc123"
  slack     ->  {"api_key": "xoxb-...", "webhook_url": "https://..."}
                                                   => ctx.secrets.slack.api_key = "xoxb-..."
  simple    ->  my-plain-text-value                => ctx.secrets.simple.value = "my-plain-text-value"
```

The Go CLI's `buildSecretFromYAML()` handles the reverse: nested YAML maps in `.secrets.yaml` are JSON-serialized into K8s Secret `stringData` entries, so the engine's JSON parsing reads them back correctly.

## Data Passing Between Nodes

Node outputs flow to downstream nodes through edges defined in workflow.yaml.

### Single Dependency

When a node has exactly one incoming edge, it receives the upstream node's return value directly as its `input`:

```yaml
edges:
  - from: fetch-data
    to: transform
```

```typescript
// fetch-data returns:
return { items: [1, 2, 3] };

// transform receives as input:
// { items: [1, 2, 3] }
```

### Multiple Dependencies

When a node has multiple incoming edges, the inputs are merged into a keyed object where each key is the upstream node name:

```yaml
edges:
  - from: fetch-users
    to: merge
  - from: fetch-orders
    to: merge
```

```typescript
// fetch-users returns:
return { users: ["alice", "bob"] };

// fetch-orders returns:
return { orders: [101, 102] };

// merge receives as input:
// {
//   "fetch-users": { users: ["alice", "bob"] },
//   "fetch-orders": { orders: [101, 102] }
// }
```

### Root Nodes

Nodes with no incoming edges receive an empty object `{}` as input.

## Error Handling

If a node throws an error, the executor catches it and records the error. Execution of the current stage fails, and no subsequent stages run (fail-fast behavior).

If `retries` is configured in workflow.yaml, failed nodes are retried with exponential backoff (100ms, 200ms, 400ms, ...).

```typescript
export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const res = await ctx.fetch("github", "/repos/owner/repo");
  if (!res.ok) {
    throw new Error(`GitHub API error: ${res.status} ${res.statusText}`);
  }
  return await res.json();
}
```

## Complete Node Example

A node that fetches GitHub issues, filters them, and returns a summary:

```typescript
import type { Context } from "tentacular";

interface Issue {
  number: number;
  title: string;
  labels: Array<{ name: string }>;
  created_at: string;
}

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const repo = ctx.config.repo as string ?? "owner/repo";
  ctx.log.info("fetching open issues", { repo });

  const res = await ctx.fetch("github", `/repos/${repo}/issues?state=open`);
  if (!res.ok) {
    ctx.log.error("failed to fetch issues", { status: res.status });
    throw new Error(`GitHub API returned ${res.status}`);
  }

  const issues: Issue[] = await res.json();
  ctx.log.info(`found ${issues.length} open issues`);

  const summary = issues.map((issue) => ({
    number: issue.number,
    title: issue.title,
    labels: issue.labels.map((l) => l.name),
    age_days: Math.floor(
      (Date.now() - new Date(issue.created_at).getTime()) / 86_400_000
    ),
  }));

  return { repo, count: summary.length, issues: summary };
}
```

## Database Patterns

### Postgres via `@db/postgres`

Nodes can use `jsr:@db/postgres` for database access. Import it in the node file and connect using credentials from `ctx.secrets`:

```typescript
import { Client } from "jsr:@db/postgres@0.19.5";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const dbUrl = ctx.secrets.postgres?.connection_string;
  if (!dbUrl) {
    ctx.log.warn("no postgres credentials, skipping");
    return { skipped: true, reason: "no credentials" };
  }

  const client = new Client(dbUrl);
  await client.connect();
  try {
    const result = await client.queryObject("SELECT * FROM snapshots LIMIT 10");
    return { rows: result.rows };
  } finally {
    await client.end();
  }
}
```

**Important:** `@db/postgres` auto-parses JSONB columns to JavaScript objects. Do not call `JSON.parse()` on values from JSONB columns -- they are already objects, and double-parsing will throw an error or produce incorrect results.
