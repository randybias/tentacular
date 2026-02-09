## Why

Pipedreamer v2 has an engine scaffold (types, directory structure) from Change 01, but no execution capability. Workflows defined in `workflow.yaml` cannot be compiled into an execution plan or run. The DAG engine is the core runtime that turns a workflow spec into executable stages and runs them. Without this, the `dev` command (Change 05), testing framework (Change 07), and all higher-level features are blocked.

The executor interface must be pluggable from day one because the roadmap includes swapping in Temporal for production deployments. Building the abstraction now avoids a costly refactor later.

## What Changes

- **DAG compiler** (`engine/compiler/mod.ts`) — takes a `WorkflowSpec` and produces a `CompiledDAG` with topologically sorted stages. Uses Kahn's algorithm for topological sort. Detects cycles and validates edge references. Groups nodes into parallel execution stages.
- **Executor interfaces** (`engine/executor/types.ts`) — `WorkflowExecutor` and `NodeRunner` interfaces that decouple execution strategy from the DAG structure. Enables Temporal swap later without changing the compiler or node code.
- **SimpleExecutor** (`engine/executor/simple.ts`) — lightweight in-memory executor (~200 lines). Executes stages sequentially, nodes within a stage in parallel via `Promise.all`. Includes timeout enforcement, retry with exponential backoff, fail-fast error propagation, and in-memory data passing between nodes.
- **Dynamic node loader** (`engine/loader.ts`) — loads node modules via `import()` with module caching and cache-busting for hot-reload support. Validates that loaded modules export a default async function.
- **Engine type definitions** (`engine/types.ts`) — `CompiledDAG`, `Stage`, `ExecutionResult`, `ExecutionTiming`, `NodeTiming`, `NodeModule`, `NodeFunction` types that define the execution data model.

## Capabilities

### New Capabilities
- `dag-compiler`: Topological sort via Kahn's algorithm, stage grouping for parallel execution, cycle detection, and edge validation
- `executor-interface`: Pluggable `WorkflowExecutor` and `NodeRunner` interfaces, `SimpleExecutor` implementation with timeout/retry/error handling, dynamic node loading via `import()`

### Modified Capabilities
- `engine-foundation`: Extends `engine/types.ts` with execution-related types (`CompiledDAG`, `Stage`, `ExecutionResult`, `ExecutionTiming`, `NodeTiming`, `NodeModule`) and updates `engine/mod.ts` exports

## Impact

- **New files**: `engine/compiler/mod.ts`, `engine/executor/types.ts`, `engine/executor/simple.ts`, `engine/loader.ts`, `engine/compiler/compiler_test.ts`, `engine/executor/simple_test.ts`
- **Modified files**: `engine/types.ts` (new type definitions), `engine/mod.ts` (new exports)
- **Dependencies**: No new external dependencies; uses Deno std library (`std/path` for `resolve`)
- **Execution lifecycle**: load -> validate -> compile -> execute -> report
- **Subsequent changes that depend on this**: context system (Change 04), dev command (Change 05), testing framework (Change 07)
