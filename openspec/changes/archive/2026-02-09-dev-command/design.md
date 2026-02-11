## Context

Tentacular uses a two-component architecture: a Go CLI binary for management operations and a Deno/TypeScript engine for workflow execution. The project foundation (Change 01) established the CLI stubs and engine directory structure. The DAG engine (Changes 02-04) implemented the compiler, executor, context, and loader. The `tntc dev` command is Change 05 — it wires everything together into a local development server that AI agents use for rapid iteration.

The existing codebase already has stub implementations of `pkg/cli/dev.go`, `engine/server.ts`, `engine/watcher.ts`, and `engine/loader.ts`. This change specifies the requirements and design decisions that govern how these components work together.

## Goals / Non-Goals

**Goals:**
- Provide a single `tntc dev` command that starts a complete local development environment
- Hot-reload node modules on file change without restarting the server
- Expose an HTTP endpoint for triggering workflow runs programmatically (agent-friendly)
- Print execution trace output (DAG structure, node timing, data flow) to console
- Load secrets from a local `.secrets.yaml` file for development
- Graceful shutdown on SIGINT/SIGTERM with clean child process termination

**Non-Goals:**
- Remote debugging or IDE integration
- Hot-reloading `workflow.yaml` changes (requires restart; only node source files are hot-reloaded)
- TLS or authentication on the dev server (local-only, plaintext)
- Multi-workflow serving (one dev server per workflow)
- Production deployment (handled by `tntc build` and `tntc deploy` in Change 06)

## Decisions

### Decision 1: Go CLI spawns Deno child process
**Choice:** The `tntc dev` command in Go spawns `deno run engine/main.ts --watch` as a child process with explicit Deno permission flags (`--allow-net`, `--allow-read`, `--allow-write=/tmp`, `--allow-env`).
**Rationale:** This maintains the clean separation between Go CLI (management) and Deno engine (execution). The Go process handles signal forwarding and lifecycle management, while the Deno process handles the actual workflow execution. The explicit permission flags enforce Deno's security sandbox even in development.

### Decision 2: Deno.watchFs for file watching with debounce
**Choice:** Use `Deno.watchFs` with recursive monitoring and a 200ms debounce timer. Only `.ts`, `.js`, `.yaml`, and `.json` file extensions trigger reloads.
**Rationale:** `Deno.watchFs` is Deno's built-in file system watcher — no external dependency needed. The debounce prevents cascading reloads when editors write multiple files atomically (e.g., save + format). The extension filter avoids reloading on irrelevant file changes (`.git`, editor swap files, etc.). The 200ms debounce is fast enough for interactive development but avoids thrashing.

### Decision 3: Cache-busting dynamic imports for hot-reload
**Choice:** Clear the in-memory module cache and re-import node modules with `?t=<timestamp>` query parameter appended to the `file://` import URL.
**Rationale:** Deno caches modules by URL. Appending a unique timestamp makes each import URL unique, forcing V8 to re-evaluate the module source. This is simpler and more reliable than trying to invalidate Deno's internal module cache. The module cache map in `loader.ts` is also cleared so stale references are not retained.

### Decision 4: HTTP server with /run and /health endpoints
**Choice:** Use `Deno.serve` to expose two endpoints: `POST /run` (also accepts GET for convenience) triggers workflow execution and returns the `ExecutionResult` as JSON; `GET /health` returns `{"status":"ok"}`.
**Rationale:** `Deno.serve` is Deno's built-in HTTP server — zero dependencies. The `/run` endpoint is the primary interface for AI agents: `curl localhost:8080/run` triggers execution and returns structured results. The `/health` endpoint allows readiness checks. Accepting GET on `/run` simplifies agent tooling (no need to construct POST bodies for simple triggers).

### Decision 5: Dev-prod parity through shared entrypoint
**Choice:** The same `engine/main.ts` file serves as the entrypoint in both development (`--watch` flag) and production (container ENTRYPOINT without `--watch`). The file watcher is only activated when `--watch` is passed.
**Rationale:** This ensures that workflow behavior is identical in dev and production. The only difference is the presence of the file watcher and hot-reload machinery, which is gated behind the `--watch` flag. This eliminates the class of bugs where workflows work in dev but fail in production due to different execution paths.

### Decision 6: Secrets loaded from .secrets.yaml
**Choice:** The engine loads secrets from `.secrets.yaml` in the workflow directory during startup. In production, it also checks `/app/secrets` for Kubernetes volume mounts and merges them.
**Rationale:** Developers need access to API keys and credentials during local development. The `.secrets.yaml` file mirrors the production secrets interface (`ctx.secrets`) so node code does not need conditional logic. The file is listed in `.gitignore` to prevent accidental commits. The `.secrets.yaml.example` file (scaffolded by `tntc init`) documents the expected structure.

## Risks / Trade-offs

- **Deno dependency** -- The dev server requires Deno to be installed locally. Mitigated by clear error messages when Deno is not found, and documentation pointing to `deno.land` for installation.
- **Hot-reload scope limited to node source** -- Changes to `workflow.yaml` (adding/removing nodes, changing edges) require a server restart. This is acceptable because DAG structure changes are infrequent compared to node logic changes, and re-compiling the DAG mid-execution could lead to inconsistent state.
- **No concurrent run protection** -- Multiple simultaneous `/run` requests execute independently. This is acceptable for local development where a single agent drives the loop sequentially. Production deployments would need request queuing (out of scope for this change).
- **File watcher platform differences** -- `Deno.watchFs` behavior varies slightly across macOS (FSEvents), Linux (inotify), and Windows (ReadDirectoryChangesW). The debounce timer mitigates most platform-specific quirks (e.g., macOS emitting duplicate events).
