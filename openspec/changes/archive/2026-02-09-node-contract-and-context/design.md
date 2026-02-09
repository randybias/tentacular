## Context

Pipedreamer v2 nodes are TypeScript files that run in a Deno engine. The engine needs a clear contract for how it invokes nodes and what capabilities it provides them. The project foundation (Change 01) established stub types for `Context`, `Logger`, and `NodeFunction` in `engine/types.ts` and placeholder implementations in `engine/context/`. This change fully specifies the contract and Context API so that both node authors and the executor have a stable interface.

## Goals / Non-Goals

**Goals:**
- Define the node function contract: default export, async, receives Context + input, returns output
- Implement the Context object as a plain object with fetch, log, config, and secrets
- Implement secret loading from `.secrets.yaml` files (local dev) and K8s Secret volume mount directories (production)
- Ensure `ctx.fetch(service, path)` automatically injects auth headers from loaded secrets
- Ensure `ctx.log` prefixes all messages with the node ID for traceability
- Export all node-author-facing types from `engine/mod.ts` via the `"pipedreamer"` import map alias

**Non-Goals:**
- Gateway sidecar proxy for `ctx.fetch` calls (future enhancement — currently goes direct)
- Persistent data store or cross-execution state (nodes pass data in-memory only)
- Secret rotation or dynamic secret reloading (secrets loaded once at engine startup)
- Node sandboxing or permission restrictions beyond Deno's built-in `--allow-*` flags
- Retry logic or circuit breaking in `ctx.fetch` (future enhancement)

## Decisions

### Decision 1: Context is a plain object, not a class
**Choice:** Context is created by a factory function `createContext(opts)` that returns a plain object satisfying the `Context` interface.
**Rationale:** Plain objects are simpler to mock in tests (just spread or override properties), serializable for debugging, and avoid `this` binding issues. Node authors interact with it as `ctx.log.info(...)` and `ctx.fetch(...)` — no class methods needed. Alternative considered: a Context class with methods — rejected because it adds complexity without benefit and makes test mocking harder.

### Decision 2: Secrets via file mount, not environment variables
**Choice:** Secrets are loaded from `.secrets.yaml` (a YAML file keyed by service name) for local development, and from a directory of files (K8s Secret volume mount at `/secrets/`) for production.
**Rationale:** Environment variables leak into process listings and child processes. File-based secrets align with K8s Secret volume mount patterns and the "Fortress" deployment model. The `loadSecrets(path)` function auto-detects whether the path is a file or directory and handles both. `.secrets.yaml` is gitignored; `.secrets.yaml.example` is scaffolded by `pipedreamer init`.

### Decision 3: ctx.fetch injects auth headers from secrets automatically
**Choice:** When `ctx.fetch(service, path)` is called, the fetch function looks up `secrets[service]` and injects headers: `token` becomes `Authorization: Bearer <token>`, `api_key` becomes `X-API-Key: <api_key>`.
**Rationale:** Nodes should not handle auth boilerplate. The service name acts as a key into the secrets map. This keeps node code focused on business logic. If no secrets exist for the service, the request proceeds without auth headers — this is not an error.

### Decision 4: Logger prefixes with node ID
**Choice:** `ctx.log` methods prefix output with `[<nodeId>]` followed by the level (INFO, WARN, ERROR, DEBUG).
**Rationale:** When multiple nodes execute (especially in parallel stages), prefixing with the node ID makes log output traceable. Uses `console.log/warn/error/debug` under the hood for compatibility with Deno's built-in logging.

### Decision 5: Service URL resolution is convention-based
**Choice:** `ctx.fetch(service, path)` resolves URLs by convention: if `path` starts with `http`, it is used as-is; otherwise it constructs `https://api.<service>.com<path>`.
**Rationale:** Simple convention that works for most SaaS APIs (e.g., `ctx.fetch("github", "/repos/...")` becomes `https://api.github.com/repos/...`). Nodes can always pass a full URL to override. Future: the Gateway sidecar will intercept these calls and handle routing, at which point this convention becomes the sidecar's responsibility.

## Risks / Trade-offs

- **Convention-based URL resolution is limited** — Services with non-standard API URLs (e.g., `https://hooks.slack.com`) must use full URLs in the path argument. Acceptable because the convention covers the common case and full URL override is always available.
- **Secrets loaded once at startup** — If secrets change, the engine must restart. Acceptable for v2 scope; dynamic reloading can be added later.
- **No fetch retry/circuit-breaking** — Network failures in `ctx.fetch` propagate as-is to the node. The node or executor retry logic must handle this. Acceptable because retry should be at the executor/workflow level, not the fetch level.
- **Auth header injection is opinionated** — Only `token` (Bearer) and `api_key` (X-API-Key) patterns are supported. Custom auth schemes require the node to set headers manually via the `init` parameter. This covers the majority of API auth patterns.
