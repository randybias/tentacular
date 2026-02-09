## ADDED Requirements

### Requirement: WorkflowExecutor interface
The `WorkflowExecutor` interface SHALL define a single `execute(graph: CompiledDAG, nodeRunner: NodeRunner): Promise<ExecutionResult>` method that decouples orchestration strategy from DAG structure.

#### Scenario: Interface contract
- **WHEN** a class implements `WorkflowExecutor`
- **THEN** it SHALL accept a `CompiledDAG` and `NodeRunner` and return a `Promise<ExecutionResult>`

### Requirement: NodeRunner interface
The `NodeRunner` interface SHALL define a `run(nodeId: string, ctx: Context, input: unknown): Promise<unknown>` method that loads and executes individual nodes.

#### Scenario: Interface contract
- **WHEN** a class implements `NodeRunner`
- **THEN** it SHALL accept a node ID, execution context, and input, and return the node's output

### Requirement: SimpleExecutor executes single-node workflows
The `SimpleExecutor` SHALL correctly execute a workflow with a single node.

#### Scenario: Single node success
- **WHEN** a compiled DAG has one stage with one node, and the node returns `{ result: "ok" }`
- **THEN** the `ExecutionResult` SHALL have `success: true`
- **AND** `outputs` SHALL contain the node's output keyed by node ID
- **AND** `errors` SHALL be empty

#### Scenario: Single node failure
- **WHEN** a compiled DAG has one node and the node throws an Error
- **THEN** the `ExecutionResult` SHALL have `success: false`
- **AND** `errors` SHALL contain the error message keyed by node ID

### Requirement: SimpleExecutor executes multi-stage workflows
The `SimpleExecutor` SHALL execute stages sequentially, with nodes within a stage running in parallel via `Promise.all`.

#### Scenario: Chain execution (a -> b -> c)
- **WHEN** a three-stage DAG is executed with nodes a, b, c
- **THEN** node "a" SHALL execute first
- **AND** node "b" SHALL receive node "a"'s output as input
- **AND** node "c" SHALL receive node "b"'s output as input
- **AND** all three outputs SHALL be in `ExecutionResult.outputs`

#### Scenario: Parallel execution within a stage
- **WHEN** a stage contains nodes "b" and "c" that each take 50ms
- **THEN** the stage SHALL complete in approximately 50ms (not 100ms)
- **AND** both nodes SHALL receive their respective inputs

#### Scenario: Fan-in input merging
- **WHEN** node "d" has dependencies on both "b" and "c"
- **THEN** node "d" SHALL receive a merged input object `{ b: outputB, c: outputC }`

### Requirement: Timeout enforcement
The `SimpleExecutor` SHALL enforce a configurable per-node timeout, defaulting to 30 seconds.

#### Scenario: Node exceeds timeout
- **WHEN** a node takes longer than the configured timeout
- **THEN** the node SHALL fail with an error message containing "timed out"
- **AND** the error message SHALL include the timeout duration in milliseconds

#### Scenario: Custom timeout
- **WHEN** `SimpleExecutor` is constructed with `{ timeoutMs: 5000 }`
- **THEN** nodes that take longer than 5000ms SHALL be timed out

#### Scenario: Default timeout
- **WHEN** `SimpleExecutor` is constructed with no options
- **THEN** the timeout SHALL default to 30000ms

### Requirement: Retry with exponential backoff
The `SimpleExecutor` SHALL support configurable retry count with exponential backoff (100ms base).

#### Scenario: Retry succeeds on second attempt
- **WHEN** `maxRetries` is 2 and a node fails on the first attempt but succeeds on the second
- **THEN** the `ExecutionResult` SHALL have `success: true`
- **AND** the node's output SHALL be the successful result

#### Scenario: Retry exhausted
- **WHEN** `maxRetries` is 1 and a node fails on both attempts
- **THEN** the node SHALL be marked as failed in `errors`

#### Scenario: Exponential backoff timing
- **WHEN** retries are configured
- **THEN** delay between retries SHALL follow `100ms * 2^attempt` (100ms, 200ms, 400ms, ...)

#### Scenario: No retries by default
- **WHEN** `SimpleExecutor` is constructed with no options
- **THEN** `maxRetries` SHALL default to 0 (no retries)

### Requirement: Fail-fast error propagation
The `SimpleExecutor` SHALL stop executing subsequent stages when a node fails (after the current stage completes).

#### Scenario: Failure stops subsequent stages
- **WHEN** node "a" fails in stage 0 of a two-stage DAG
- **THEN** stage 1 nodes SHALL NOT execute
- **AND** `ExecutionResult.success` SHALL be `false`

#### Scenario: Parallel stage partial failure
- **WHEN** stage 1 has nodes "b" and "c", and "b" fails while "c" succeeds
- **THEN** both "b" and "c" results SHALL be recorded (since they run via `Promise.all`)
- **AND** subsequent stages SHALL NOT execute

### Requirement: Execution timing
The `SimpleExecutor` SHALL record timing information for the overall execution and each node.

#### Scenario: Timing data structure
- **WHEN** a workflow completes (success or failure)
- **THEN** `ExecutionResult.timing` SHALL contain `startedAt`, `completedAt`, and `durationMs`
- **AND** `timing.nodeTimings` SHALL contain per-node `startedAt`, `completedAt`, and `durationMs`

### Requirement: Dynamic node loading via import()
The `loadNode()` function SHALL dynamically load node modules using `import()`, validate the default export, and cache modules.

#### Scenario: Successful node load
- **WHEN** `loadNode("nodes/hello.ts", workflowDir)` is called
- **THEN** the module SHALL be loaded via `import()` and the default export returned
- **AND** the module SHALL be cached for subsequent calls

#### Scenario: Invalid node module
- **WHEN** a loaded module does not export a default function
- **THEN** `loadNode()` SHALL throw an Error with message containing "must export a default async function"

#### Scenario: Cache busting for hot-reload
- **WHEN** `loadNode()` is called with `bustCache = true`
- **THEN** a timestamp query parameter SHALL be appended to the import path
- **AND** the module SHALL be re-loaded even if previously cached

#### Scenario: Load all nodes
- **WHEN** `loadAllNodes()` is called with a workflow's node specs
- **THEN** it SHALL return a `Map<string, NodeFunction>` with all nodes loaded
- **AND** each node SHALL be loaded via `loadNode()`

#### Scenario: Clear module cache
- **WHEN** `clearModuleCache()` is called
- **THEN** all cached modules SHALL be removed
- **AND** subsequent `loadNode()` calls SHALL re-import modules
