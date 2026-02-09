## Context

Pipedreamer v2 workflows consist of TypeScript node functions wired into a DAG defined in `workflow.yaml`. The engine already has a compiler (`engine/compiler/mod.ts`) that produces a `CompiledDAG`, a `SimpleExecutor` (`engine/executor/simple.ts`) that runs stages in order, a loader (`engine/loader.ts`) that dynamically imports node modules, and a context factory (`engine/context/mod.ts`). The CLI (`cmd/pipedreamer/main.go`) already registers a `test` command stub via `pkg/cli/test.go`.

This change adds a testing subsystem in `engine/testing/` that reuses all of the above to provide node-level and pipeline-level testing, plus a CLI integration that spawns the Deno test runner.

## Goals / Non-Goals

**Goals:**
- Enable testing individual nodes in isolation using fixture inputs and a mock context
- Enable testing full pipeline execution through the compiled DAG with mocked external calls
- Provide a simple JSON fixture format that is easy to author by hand or generate
- Integrate with the CLI as `pipedreamer test [dir][/<node>]`
- Output clear pass/fail results with timing

**Non-Goals:**
- Integration testing against real external services (future enhancement)
- Code coverage measurement
- Snapshot testing or visual diff testing
- Parallel test execution across multiple workflows

## Decisions

### Decision 1: Deno-side test runner (not Go)
**Choice:** The test runner is implemented in TypeScript at `engine/testing/runner.ts`, invoked by the Go CLI via `deno run`.
**Rationale:** Tests must load and execute TypeScript node modules. The Deno runtime is the only component that can dynamically import `.ts` files. The Go CLI acts as a launcher, passing the workflow path and flags to the Deno process.

### Decision 2: Three test levels
**Choice:** Node testing (default), pipeline testing (`--pipeline`), integration testing (future, not implemented).
**Rationale:** Node testing is the most common need -- verify a single function with known inputs. Pipeline testing validates the full DAG wiring. Integration testing (with real HTTP) is deferred as it requires network access and service credentials.

### Decision 3: Fixture-based testing
**Choice:** Test inputs/outputs are defined as JSON files in `tests/fixtures/<nodename>.json`.
**Rationale:** JSON fixtures are language-agnostic, easy to version-control, and can be auto-generated. The format is `{ "input": <value>, "expected": <value> }` where `expected` is optional (omitting it means "just don't crash").

### Decision 4: Mock Context with captured logs and mock fetch
**Choice:** `createMockContext()` returns a `Context` where `fetch` returns mock responses and `log` captures entries to an array.
**Rationale:** Nodes call `ctx.fetch(service, path)` for external APIs. In tests, these must not make real HTTP calls. The mock returns a default `{ mock: true }` response or a pre-configured response. Log capture enables assertions on logging behavior.

## Architecture

```
pipedreamer test myworkflow/fetch-data
        |
        v
  pkg/cli/test.go  (Go CLI)
        |
        | spawns: deno run engine/testing/runner.ts
        |           --workflow ./myworkflow/workflow.yaml
        |           --node fetch-data
        v
  engine/testing/runner.ts  (Deno)
        |
        +-- Parses workflow.yaml
        +-- Loads node module via engine/loader.ts
        +-- Loads fixtures from tests/fixtures/fetch-data*.json
        +-- Creates mock context via engine/testing/mocks.ts
        +-- Executes node function with fixture input
        +-- Compares output to expected (if provided)
        +-- Prints pass/fail/timing report
```

For pipeline mode (`--pipeline`):
```
  engine/testing/runner.ts --pipeline
        |
        +-- Compiles DAG via engine/compiler/mod.ts
        +-- Loads all node modules
        +-- Creates mock context
        +-- Executes full DAG via engine/executor/simple.ts
        +-- Reports pipeline pass/fail/timing
```

## File Layout

- `engine/testing/runner.ts` -- Main test runner entry point (node + pipeline modes)
- `engine/testing/mocks.ts` -- Mock Context factory, mock fetch response helper
- `engine/testing/fixtures.ts` -- Fixture loading and discovery
- `pkg/cli/test.go` -- Go CLI command that spawns the Deno test runner

## Risks / Trade-offs

- **Process spawn overhead**: Each `pipedreamer test` invocation spawns a Deno process. For fast iteration, this adds ~200ms startup. Acceptable for now; future optimization could use a long-running Deno process.
- **Fixture naming convention**: Fixtures are matched by filename prefix (`<nodename>*.json`). If two nodes share a prefix (e.g., `fetch` and `fetch-data`), fixtures could collide. Mitigated by using the full node name as prefix.
- **Mock context limitations**: The `getLogs()` helper currently returns an empty array due to closure scoping. The logs are captured in the mock but not easily accessible externally. This is a known limitation to be improved.
