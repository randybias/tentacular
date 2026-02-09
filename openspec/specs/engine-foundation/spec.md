## ADDED Requirements

### Requirement: Deno engine type-checks
The engine directory SHALL contain valid TypeScript that passes `deno check main.ts` with strict mode enabled.

#### Scenario: Type check passes
- **WHEN** `deno check main.ts` is executed in the `engine/` directory
- **THEN** the command SHALL exit with code 0 and no type errors

### Requirement: Deno configuration
The `engine/deno.json` SHALL configure TypeScript strict mode, import maps for std library modules, and lint/format settings.

#### Scenario: Import map resolution
- **WHEN** engine code imports from `"std/yaml"`, `"std/path"`, `"std/flags"`, or `"std/assert"`
- **THEN** Deno SHALL resolve these to the pinned std library version defined in `deno.json`

#### Scenario: Pipedreamer module import
- **WHEN** a node file imports `type { Context } from "pipedreamer"`
- **THEN** Deno SHALL resolve this to `engine/mod.ts` via the import map

### Requirement: Engine entrypoint accepts CLI arguments
The `engine/main.ts` SHALL accept `--workflow`, `--port`, and `--watch` flags.

#### Scenario: Required workflow flag
- **WHEN** `engine/main.ts` is run without `--workflow`
- **THEN** it SHALL print usage and exit with code 1

#### Scenario: Default port
- **WHEN** `engine/main.ts` is run with `--workflow` but no `--port`
- **THEN** the HTTP server SHALL listen on port 8080

### Requirement: Public API module exports types
The `engine/mod.ts` SHALL export execution-related types and functions for use by the engine internals and node authors.

#### Scenario: Module exports
- **WHEN** `engine/mod.ts` is imported
- **THEN** it SHALL export at minimum: `Context`, `Logger`, `NodeFunction`, `WorkflowSpec`, `ExecutionResult`, `CompiledDAG`, `Stage`

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
