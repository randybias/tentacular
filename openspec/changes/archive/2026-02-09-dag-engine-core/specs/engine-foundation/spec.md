## ADDED Requirements

### Requirement: Engine type definitions include execution types
The `engine/types.ts` SHALL export execution-related types in addition to the existing workflow types.

#### Scenario: CompiledDAG type
- **WHEN** `engine/types.ts` is imported
- **THEN** it SHALL export `CompiledDAG` with fields: `workflow` (WorkflowSpec), `stages` (Stage[]), `nodeOrder` (string[])

#### Scenario: Stage type
- **WHEN** `engine/types.ts` is imported
- **THEN** it SHALL export `Stage` with field: `nodes` (string[])

#### Scenario: ExecutionResult type
- **WHEN** `engine/types.ts` is imported
- **THEN** it SHALL export `ExecutionResult` with fields: `success` (boolean), `outputs` (Record<string, unknown>), `errors` (Record<string, string>), `timing` (ExecutionTiming)

#### Scenario: ExecutionTiming type
- **WHEN** `engine/types.ts` is imported
- **THEN** it SHALL export `ExecutionTiming` with fields: `startedAt` (number), `completedAt` (number), `durationMs` (number), `nodeTimings` (Record<string, NodeTiming>)

#### Scenario: NodeTiming type
- **WHEN** `engine/types.ts` is imported
- **THEN** it SHALL export `NodeTiming` with fields: `startedAt` (number), `completedAt` (number), `durationMs` (number)

#### Scenario: NodeModule type
- **WHEN** `engine/types.ts` is imported
- **THEN** it SHALL export `NodeModule` with field: `default` (NodeFunction)

## MODIFIED Requirements

### Requirement: Public API module exports types
The `engine/mod.ts` SHALL export execution-related types and functions for use by the engine internals and node authors.

#### Scenario: Module exports
- **WHEN** `engine/mod.ts` is imported
- **THEN** it SHALL export at minimum: `Context`, `Logger`, `NodeFunction`, `WorkflowSpec`, `ExecutionResult`, `CompiledDAG`, `Stage`
