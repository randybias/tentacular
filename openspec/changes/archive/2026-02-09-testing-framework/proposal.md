## Why

Tentacular workflows are composed of TypeScript nodes wired into a DAG. Developers need a way to test nodes in isolation with fixture data, run full pipeline tests with mocked external calls, and get clear pass/fail/timing reports -- all from the CLI. Without a built-in testing framework, workflow authors must roll their own test harnesses, leading to inconsistent quality practices and slower iteration.

## What Changes

- **Node test runner** (`engine/testing/runner.ts`): Loads a workflow spec, discovers fixture files under `tests/fixtures/<node>.json`, dynamically imports each node function, invokes it with the fixture input and a mock context, and asserts outputs match expected values.
- **Pipeline test runner** (same file, `--pipeline` flag): Compiles the full DAG, loads all node modules, creates a mock context, and executes end-to-end through the SimpleExecutor with mocked fetch calls.
- **Test fixture format** (`engine/testing/fixtures.ts`): JSON files with `{ "input": ..., "expected": ... }` loaded from `tests/fixtures/`.
- **Mock Context** (`engine/testing/mocks.ts`): Provides a `createMockContext()` that captures log calls and returns mock fetch responses, enabling isolated node testing without real HTTP calls.
- **CLI test command** (`pkg/cli/test.go`): `tntc test [dir][/<node>]` spawns `deno run engine/testing/runner.ts` with appropriate flags. Supports `--pipeline` flag for full DAG testing.
- **Test report output**: Clear pass/fail per test with timing in milliseconds, summary count, and non-zero exit code on any failure.

## Capabilities

### New Capabilities
- `testing-framework`: Node-level and pipeline-level test runner with fixture loading, mock context, CLI integration, and structured test reporting.

### Modified Capabilities
_(none)_

## Impact

- **New files**: `engine/testing/runner.ts`, `engine/testing/mocks.ts`, `engine/testing/fixtures.ts`
- **Modified files**: `pkg/cli/test.go` (filled in from stub)
- **Dependencies**: No new dependencies; uses existing Deno std library and engine modules (compiler, executor, loader, context)
- **Convention**: Test fixtures live in `<workflow>/tests/fixtures/<nodename>.json`
