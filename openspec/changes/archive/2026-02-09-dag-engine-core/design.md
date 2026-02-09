## Context

Pipedreamer v2 Change 01 (project-foundation) established the two-component architecture: Go CLI and Deno engine. The engine directory has `types.ts` with `WorkflowSpec`, `Edge`, and basic type scaffolding, plus empty `compiler/` and `executor/` directories. No compilation or execution logic exists yet.

This change implements the core DAG engine: the compiler that turns a `WorkflowSpec` into an executable `CompiledDAG`, and the executor that runs it. The design must support a future swap to Temporal for production orchestration without changing node code or the compiler.

## Goals / Non-Goals

**Goals:**
- Implement a DAG compiler with topological sort (Kahn's algorithm) and stage-based grouping
- Define pluggable `WorkflowExecutor` and `NodeRunner` interfaces
- Implement `SimpleExecutor` for local development and testing (~200 lines)
- Support timeout enforcement, retry with exponential backoff, and fail-fast error propagation
- Enable dynamic node loading via `import()` with caching
- Ensure all data passed between nodes is JSON-serializable (Temporal compatibility)
- Provide comprehensive unit tests for compiler and executor

**Non-Goals:**
- Temporal executor implementation (future change)
- Context system implementation (Change 04)
- HTTP trigger server integration (Change 05)
- Workflow validation beyond edge references (Change 02)
- Distributed execution or multi-worker support
- Persistent state or checkpointing

## Decisions

### Decision 1: Pluggable executor interface
**Choice:** Define `WorkflowExecutor` and `NodeRunner` as separate interfaces in `engine/executor/types.ts`.
**Rationale:** The `WorkflowExecutor` interface (`execute(graph, nodeRunner) -> ExecutionResult`) decouples the orchestration strategy from the DAG structure. `SimpleExecutor` implements it for local dev; `TemporalExecutor` can implement it later using Temporal workflows and activities. The `NodeRunner` interface (`run(nodeId, ctx, input) -> output`) maps directly to Temporal activities, making the swap mechanical. Alternative considered: a single monolithic executor class — rejected because it would require rewriting the entire executor when adding Temporal support.

### Decision 2: Kahn's algorithm for topological sort
**Choice:** Use Kahn's algorithm (BFS-based) rather than DFS-based topological sort.
**Rationale:** Kahn's algorithm naturally detects cycles (if the sorted output has fewer nodes than the input, a cycle exists). It also produces a deterministic ordering when ties are broken alphabetically, which is important for reproducible test output. The queue-based approach maps cleanly to the stage-building step. Alternative considered: DFS with cycle detection via coloring — rejected because it requires a separate cycle detection pass and produces less intuitive orderings.

### Decision 3: Stage-based parallelism with Promise.all
**Choice:** Group nodes into stages where all dependencies are in earlier stages. Execute stages sequentially; nodes within a stage run in parallel via `Promise.all`.
**Rationale:** This is the simplest correct parallel execution model. It avoids the complexity of a fully dynamic scheduler while still exploiting available parallelism in the DAG. In a diamond pattern (A -> B, A -> C, B -> D, C -> D), B and C run in parallel in stage 1. This maps well to Temporal's workflow model where parallel activities are launched via `Promise.all`. Alternative considered: fully dynamic scheduling where each node starts as soon as its dependencies complete — rejected as premature optimization that complicates error handling and debugging.

### Decision 4: JSON-serializable data passing
**Choice:** All data passed between nodes must be JSON-serializable (`unknown` type at the TypeScript level, but documented as JSON-compatible).
**Rationale:** Temporal requires all activity inputs and outputs to be serializable for persistence and replay. Enforcing this constraint from the start in `SimpleExecutor` means node code written for local dev will work unchanged with Temporal. The `SimpleExecutor` passes data in-memory (no serialization overhead), but the contract is established. Alternative considered: allowing arbitrary objects including functions and closures — rejected because it would break when switching to Temporal.

### Decision 5: Fail-fast error propagation (configurable per-workflow)
**Choice:** When a node fails in a stage, stop executing subsequent stages. All nodes in the current stage complete (since they are already running via `Promise.all`), but no new stages start.
**Rationale:** Fail-fast is the safest default for data pipelines where downstream nodes depend on upstream outputs. Running downstream nodes after a failure would produce incorrect results or confusing errors. The `WorkflowConfig.retries` field allows per-workflow retry configuration. Alternative considered: continue-on-error mode — deferred to a future change as a `WorkflowConfig` option.

### Decision 6: Dynamic node loading via import()
**Choice:** Load node modules dynamically using `import()` with a module cache keyed by absolute path.
**Rationale:** Dynamic import enables hot-reload for the `dev` command (Change 05) by cache-busting with a timestamp query parameter. The cache prevents redundant module loading during normal execution. Nodes must export a `default` async function matching the `NodeFunction` type. Alternative considered: pre-bundling all nodes — rejected because it would prevent hot-reload and increase startup time.

## Risks / Trade-offs

- **In-memory data passing limits workflow size** — `SimpleExecutor` holds all node outputs in memory. For workflows with large data payloads, this could cause memory pressure. Acceptable for local dev; Temporal executor will use persistent storage.
- **Stage-based parallelism is suboptimal** — A fully dynamic scheduler could start nodes earlier (e.g., in a complex DAG, some nodes in "stage 3" might have all deps satisfied before all of "stage 2" completes). The simplicity trade-off is worth it for v2.0.
- **No cancellation support** — `setTimeout`-based timeout creates the timer but cannot cancel an in-flight `Promise`. The node continues running after timeout; only the result is discarded. Acceptable for local dev. Temporal provides true cancellation.
- **Exponential backoff in retries blocks the executor** — Retry delays (`100ms * 2^attempt`) block the stage. For long retry sequences this could cause timeouts. Mitigated by the overall timeout enforced per-node.
