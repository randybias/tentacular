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

- `ctx` -- the Context object providing dependency resolution, logging, config, and secrets.
- `input` -- data from upstream node(s) via edges. `{}` for root nodes (no incoming edges).
- Return value -- passed as input to downstream node(s) via edges.

The engine validates that the default export is a function at load time. If the export is missing or not a function, the engine throws an error.

## Context.dependency (Primary API)

```typescript
ctx.dependency(name: string): DependencyConnection
```

Returns connection metadata for a declared contract dependency. This is the primary API for accessing external services.

### DependencyConnection Fields

| Field | Type | Description |
|-------|------|-------------|
| `protocol` | `string` | Dependency protocol (any string; known: `https`, `postgresql`, `nats`, `blob`) |
| `host` | `string` | Dependency host |
| `port` | `number` | Dependency port (protocol defaults applied) |
| `authType` | `string \| undefined` | Auth mechanism from contract (any string, e.g., `bearer-token`, `api-key`, `hmac-sha256`) |
| `secret` | `string \| undefined` | Resolved secret value |
| `database` | `string \| undefined` | Database name (postgresql) |
| `user` | `string \| undefined` | Database user (postgresql) |
| `subject` | `string \| undefined` | NATS subject |
| `container` | `string \| undefined` | Blob container |
| `fetch` | `(path, init?) => Promise<Response> \| undefined` | URL builder for HTTPS deps (no auth injection) |

### HTTPS Dependencies

HTTPS dependencies include a `fetch()` convenience method that builds the URL as `https://<host>:<port><path>`. It does not inject auth headers -- nodes must handle auth explicitly.

```typescript
const gh = ctx.dependency("github-api");

// fetch() builds the URL, but auth must be set explicitly:
const res = await gh.fetch!("/repos/owner/repo/issues", {
  headers: { "Authorization": `Bearer ${gh.secret}` },
});
```

### Non-HTTPS Dependencies

For non-HTTPS protocols (postgresql, nats, etc.), use the connection metadata to build connections:

```typescript
import { Client } from "jsr:@db/postgres@0.19.5";

const pg = ctx.dependency("postgres");
const connStr = `postgresql://${pg.user}:${pg.secret}@${pg.host}:${pg.port}/${pg.database}`;
const client = new Client(connStr);
await client.connect();
```

### Auth Patterns

`dep.authType` is any string describing the auth mechanism. Nodes use it with `dep.secret` to set auth:

```typescript
// Bearer token
const gh = ctx.dependency("github-api");
// gh.authType === "bearer-token"
await gh.fetch!("/repos/org/repo", {
  headers: { "Authorization": `Bearer ${gh.secret}` },
});

// API key
const svc = ctx.dependency("my-service");
// svc.authType === "api-key"
await svc.fetch!("/data", {
  headers: { "X-API-Key": svc.secret! },
});

// SAS token (appended to URL query)
const blob = ctx.dependency("azure-blob");
// blob.authType === "sas-token"
await blob.fetch!(`/container/file.html?${blob.secret}`, {
  method: "PUT",
  body: htmlContent,
});

// Custom auth (e.g., HMAC signature)
const api = ctx.dependency("webhook-target");
// api.authType === "hmac-sha256"
const sig = computeHmac(api.secret!, body);
await api.fetch!("/hook", {
  method: "POST",
  headers: { "X-Signature": sig },
  body,
});
```

### Undeclared Dependency Error

Calling `ctx.dependency()` with a name not in the contract throws an error:

```typescript
// Throws: Dependency "unknown" not declared in contract.
// Add it to workflow.yaml contract.dependencies.
ctx.dependency("unknown");
```

## Context.fetch (Legacy)

```typescript
ctx.fetch(service: string, path: string, init?: RequestInit): Promise<Response>
```

**Legacy API.** Flagged as a contract violation when a contract is present. Use `ctx.dependency()` instead.

Makes HTTP requests with automatic URL construction and auth injection.

### URL Construction

- If `path` starts with `http`, it is used as the full URL directly.
- Otherwise, the URL is constructed as `https://api.<service>.com<path>`.

```typescript
// Resolves to: https://api.github.com/repos/owner/repo/issues
const res = await ctx.fetch("github", "/repos/owner/repo/issues");
```

### Auth Injection

Auth headers are injected automatically from `ctx.secrets[service]`:

| Secret Key | Header Added |
|-----------|-------------|
| `token` | `Authorization: Bearer <value>` |
| `api_key` | `X-API-Key: <value>` |

If the service has no entry in secrets, no auth headers are added.

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

## Context.secrets (Legacy)

```typescript
ctx.secrets: Record<string, Record<string, string>>
```

**Legacy API.** Flagged as a contract violation when a contract is present. Use `ctx.dependency("name").secret` instead.

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
  const gh = ctx.dependency("github-api");
  const res = await gh.fetch!("/repos/owner/repo", {
    headers: { "Authorization": `Bearer ${gh.secret}` },
  });
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

  const gh = ctx.dependency("github-api");
  if (!gh.secret) {
    ctx.log.warn("no github credentials, returning empty");
    return { repo, count: 0, issues: [], source: "mock" };
  }

  const res = await gh.fetch!(`/repos/${repo}/issues?state=open`, {
    headers: { "Authorization": `Bearer ${gh.secret}` },
  });
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

Nodes can use `jsr:@db/postgres` for database access. Use `ctx.dependency()` to get connection metadata:

```typescript
import { Client } from "jsr:@db/postgres@0.19.5";
import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const pg = ctx.dependency("postgres");
  if (!pg.secret) {
    ctx.log.warn("no postgres credentials, skipping");
    return { skipped: true, reason: "no credentials" };
  }

  const connStr = `postgresql://${pg.user}:${pg.secret}@${pg.host}:${pg.port}/${pg.database}`;
  const client = new Client(connStr);
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

## LLM API Patterns

### OpenAI Chat Completions

When calling OpenAI's chat completions API from nodes:

```typescript
const openai = ctx.dependency("openai-api");
const resp = await globalThis.fetch("https://api.openai.com/v1/chat/completions", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${openai.secret}`,
  },
  body: JSON.stringify({
    model: "gpt-5.2",
    messages: [
      { role: "system", content: "You are a helpful assistant." },
      { role: "user", content: "Summarize this data..." },
    ],
    temperature: 0.3,
    max_completion_tokens: 2000,
  }),
});
```

**Critical: `max_completion_tokens` vs `max_tokens`.**
Newer OpenAI models (gpt-5 and later) require
`max_completion_tokens`. The legacy `max_tokens`
parameter returns a 400 error on these models. Always
use `max_completion_tokens` for gpt-5, gpt-5.1,
gpt-5.2, and newer. The older `max_tokens` only works
with legacy models (gpt-4, gpt-4-turbo, gpt-3.5).
