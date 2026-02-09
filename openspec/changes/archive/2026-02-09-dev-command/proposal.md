## Why

Pipedreamer v2 workflows are developed by AI agents that need a tight edit-test loop: modify a node file, trigger a run, see results, iterate. Without a built-in dev server, agents must manually restart the engine after every file change, losing seconds per iteration. The `pipedreamer dev` command provides a local development server with file watching, hot-reload, and an HTTP trigger endpoint so that the loop reduces to: edit file, curl /run, read output.

## What Changes

- **`pipedreamer dev [dir]` CLI command** (`pkg/cli/dev.go`): Go command that resolves the workflow directory, validates `workflow.yaml` exists, and spawns `deno run engine/main.ts --watch` with appropriate Deno permission flags. Supports `--port` flag (default 8080). Handles graceful shutdown on SIGINT/SIGTERM by forwarding the signal to the child process.
- **File watcher** (`engine/watcher.ts`): Uses `Deno.watchFs` to recursively monitor the workflow directory for `.ts`, `.js`, `.yaml`, and `.json` file changes. Debounces rapid changes (200ms default) to avoid excessive reloads.
- **Hot-reload** (`engine/loader.ts`): On file change, clears the module cache and re-imports all node modules using cache-busting query parameters (`?t=<timestamp>`) on dynamic imports. The workflow spec is not re-parsed on file change (restart required for `workflow.yaml` changes).
- **HTTP trigger server** (`engine/server.ts`): Local HTTP server exposing `POST /run` (triggers workflow execution and returns JSON result), `GET /health` (returns `{"status":"ok"}`), and 404 for all other routes. Uses `Deno.serve` for the HTTP listener.
- **Console output**: Engine prints execution trace on startup (DAG structure, stage composition), and on each `/run` request emits node timing, data flow, and pass/fail status.
- **Local secrets file** (`.secrets.yaml`): Engine loads secrets from `.secrets.yaml` in the workflow directory, providing the same `ctx.secrets` interface that production uses with K8s volume mounts.

## Capabilities

### New Capabilities
- `dev-command`: Local development server with file watching, hot-reload, HTTP trigger endpoint, execution trace output, and local secrets file support.

### Modified Capabilities
_(none)_

## Impact

- **New files**: `engine/watcher.ts`, `engine/server.ts` (fully implemented from stubs)
- **Modified files**: `pkg/cli/dev.go` (filled in from stub), `engine/main.ts` (wired up watcher and server), `engine/loader.ts` (cache-busting for hot-reload)
- **Dependencies**: No new dependencies; uses existing Deno std library (`Deno.watchFs`, `Deno.serve`) and Go cobra CLI
- **Dev-prod parity**: The same `engine/main.ts` entrypoint runs in both dev mode (with `--watch`) and production (without `--watch`), ensuring identical execution semantics
