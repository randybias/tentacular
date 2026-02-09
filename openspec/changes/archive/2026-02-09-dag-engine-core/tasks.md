## 1. Engine Type Definitions

- [x] 1.1 Update `engine/types.ts` with execution types: `CompiledDAG`, `Stage`, `ExecutionResult`, `ExecutionTiming`, `NodeTiming`, `NodeModule`, `NodeFunction`
- [x] 1.2 Update `engine/mod.ts` to export new execution types (`CompiledDAG`, `Stage`, `ExecutionResult`)

## 2. DAG Compiler

- [x] 2.1 Implement `engine/compiler/mod.ts` with `compile(spec: WorkflowSpec): CompiledDAG` function
- [x] 2.2 Implement `validateEdges()` — verify all edge references point to defined nodes, reject self-loops
- [x] 2.3 Implement `topologicalSort()` using Kahn's algorithm with alphabetical tie-breaking for deterministic output
- [x] 2.4 Implement `buildStages()` — group sorted nodes into parallel execution stages based on dependency depths
- [x] 2.5 Implement cycle detection (Kahn's algorithm: sorted.length !== nodeNames.length means cycle)

## 3. Executor Interfaces

- [x] 3.1 Define `WorkflowExecutor` interface in `engine/executor/types.ts` with `execute(graph, nodeRunner): Promise<ExecutionResult>`
- [x] 3.2 Define `NodeRunner` interface in `engine/executor/types.ts` with `run(nodeId, ctx, input): Promise<unknown>`

## 4. SimpleExecutor

- [x] 4.1 Implement `engine/executor/simple.ts` — `SimpleExecutor` class implementing `WorkflowExecutor`
- [x] 4.2 Implement stage-based execution: iterate stages sequentially, nodes within a stage via `Promise.all`
- [x] 4.3 Implement input resolution: single dependency passes output directly, multiple dependencies merge into keyed object
- [x] 4.4 Implement `executeWithTimeout()` — per-node timeout enforcement via `setTimeout` racing with node execution
- [x] 4.5 Implement `executeWithRetry()` — retry with exponential backoff (100ms * 2^attempt), configurable max retries (default 0)
- [x] 4.6 Implement fail-fast error propagation — stop executing subsequent stages when a node fails
- [x] 4.7 Implement execution timing — record `startedAt`, `completedAt`, `durationMs` for overall execution and per-node

## 5. Dynamic Node Loader

- [x] 5.1 Implement `engine/loader.ts` with `loadNode(nodePath, workflowDir, bustCache?)` function
- [x] 5.2 Implement module cache keyed by absolute path, with `clearModuleCache()` for hot-reload
- [x] 5.3 Implement cache-busting via timestamp query parameter on import path
- [x] 5.4 Implement `loadAllNodes()` — load all nodes from a workflow spec, return `Map<string, NodeFunction>`
- [x] 5.5 Validate loaded modules export a `default` function, throw descriptive error if not

## 6. Unit Tests

- [x] 6.1 Write `engine/compiler/compiler_test.ts` — test topological sort for linear, fan-out, fan-in, diamond patterns
- [x] 6.2 Write compiler tests for cycle detection (direct and indirect cycles)
- [x] 6.3 Write compiler tests for edge validation (undefined nodes, self-loops)
- [x] 6.4 Write `engine/executor/simple_test.ts` — test single node execution (success and failure)
- [x] 6.5 Write executor tests for multi-stage chain execution with data passing
- [x] 6.6 Write executor tests for parallel execution within stages
- [x] 6.7 Write executor tests for timeout enforcement
- [x] 6.8 Write executor tests for retry with exponential backoff
- [x] 6.9 Write executor tests for fail-fast error propagation across stages

## 7. Verification

- [x] 7.1 Verify `deno check engine/compiler/mod.ts` passes with no type errors
- [x] 7.2 Verify `deno check engine/executor/simple.ts` passes with no type errors
- [x] 7.3 Verify `deno test engine/` passes all compiler and executor tests
- [x] 7.4 Verify `SimpleExecutor` is under ~200 lines of implementation code
