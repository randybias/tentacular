## 1. Go CLI Dev Command

- [x] 1.1 Implement `pkg/cli/dev.go` — `tntc dev [dir]` command that resolves workflow directory, validates `workflow.yaml` exists, locates the engine directory, and spawns `deno run engine/main.ts --workflow <path> --port <port> --watch`
- [x] 1.2 Add `--port` flag (default 8080) to the dev command
- [x] 1.3 Implement signal forwarding: capture SIGINT/SIGTERM in Go, forward SIGTERM to Deno child process, wait for clean exit
- [x] 1.4 Implement `findEngineDir()` resolution: check relative to binary, relative to working directory, and common install paths

## 2. File System Watcher

- [x] 2.1 Implement `engine/watcher.ts` — `watchFiles()` function using `Deno.watchFs` with recursive monitoring of the workflow directory
- [x] 2.2 Add extension filter: only trigger on `.ts`, `.js`, `.yaml`, `.json` file changes
- [x] 2.3 Add debounce timer (200ms default) to coalesce rapid file changes into a single reload
- [x] 2.4 Print changed file paths and reload status to console on each trigger

## 3. Hot-Reload Module Loading

- [x] 3.1 Implement cache-busting in `engine/loader.ts` — append `?t=<timestamp>` to dynamic import URLs when `bustCache` is true
- [x] 3.2 Implement `clearModuleCache()` to clear the in-memory module cache map
- [x] 3.3 Wire watcher `onChange` callback in `engine/main.ts` to call `clearModuleCache()` then `loadAllNodes()` with `bustCache=true`
- [x] 3.4 Handle reload errors gracefully: log error to console, keep server running with previously loaded modules

## 4. HTTP Trigger Server

- [x] 4.1 Implement `engine/server.ts` — `startServer()` using `Deno.serve` on the configured port
- [x] 4.2 Implement `POST /run` endpoint: execute workflow through `SimpleExecutor`, return `ExecutionResult` as JSON with appropriate HTTP status (200 on success, 500 on failure)
- [x] 4.3 Implement `GET /run` support (same behavior as POST for convenience)
- [x] 4.4 Implement `GET /health` endpoint: return `{"status":"ok"}` with `application/json` content-type
- [x] 4.5 Return 404 for all unmatched routes

## 5. Engine Main Entrypoint

- [x] 5.1 Wire up `engine/main.ts` to parse `--workflow`, `--port`, and `--watch` flags
- [x] 5.2 Load workflow spec from YAML, compile DAG, print startup trace (workflow name, version, stages, nodes)
- [x] 5.3 Load secrets from `.secrets.yaml` in workflow directory (and `/app/secrets` for K8s compatibility)
- [x] 5.4 Load all node modules, create context and node runner
- [x] 5.5 Start HTTP server and conditionally start file watcher when `--watch` is set

## 6. Integration Testing

- [x] 6.1 Create integration test: start dev server, modify a node file, verify hot-reload triggers within 1 second
- [x] 6.2 Create integration test: curl `localhost:8080/run`, verify JSON response with `success`, `outputs`, and `timing` fields
- [x] 6.3 Create integration test: modify node file, wait for reload, curl `/run`, verify updated node code executes
- [x] 6.4 Create integration test: verify `GET /health` returns `{"status":"ok"}` with status 200
- [x] 6.5 Create integration test: verify graceful shutdown on SIGTERM (process exits cleanly with code 0)
