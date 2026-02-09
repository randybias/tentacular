# node-contract Specification

## Purpose
TBD - created by archiving change node-contract-and-context. Update Purpose after archive.
## Requirements
### Requirement: Node default export contract
Every workflow node file SHALL export a default async function that accepts a Context object and an input value, and returns a Promise resolving to the node's output.

#### Scenario: Valid node module
- **WHEN** a node file at `nodes/my-node.ts` is loaded by the engine
- **THEN** it SHALL have a default export that is an async function

#### Scenario: Node function signature
- **WHEN** the engine invokes a node's default export
- **THEN** it SHALL pass two arguments: `ctx` (a Context object) and `input` (the upstream node's output or trigger payload)

#### Scenario: Node return value
- **WHEN** a node's default export function resolves
- **THEN** the resolved value SHALL be passed as the `input` argument to downstream nodes

#### Scenario: Node with no upstream input
- **WHEN** a node is the first in the DAG (receives trigger payload)
- **THEN** the `input` argument SHALL be the trigger's payload object

### Requirement: NodeFunction type definition
The `NodeFunction` type SHALL be defined as `(ctx: Context, input: unknown) => Promise<unknown>` and exported from the `pipedreamer` module.

#### Scenario: Type import
- **WHEN** a node author writes `import type { NodeFunction } from "pipedreamer"`
- **THEN** the import SHALL resolve successfully via the Deno import map

#### Scenario: Type compatibility
- **WHEN** a node file declares `export default async function run(ctx: Context, input: unknown): Promise<unknown>`
- **THEN** it SHALL satisfy the `NodeFunction` type

### Requirement: NodeModule interface
The engine SHALL expect loaded node modules to conform to a `NodeModule` interface with a `default` property of type `NodeFunction`.

#### Scenario: Dynamic import
- **WHEN** the engine dynamically imports a node file
- **THEN** the imported module SHALL have a `default` property that is a function

#### Scenario: Missing default export
- **WHEN** a node file does not have a default export
- **THEN** the engine SHALL report an error indicating the node file is missing a default export

### Requirement: In-memory data passing
Node outputs SHALL be passed in-memory to downstream nodes without persistence. The executor holds outputs in a `Record<string, unknown>` map keyed by node ID.

#### Scenario: Sequential nodes
- **WHEN** node A returns `{ count: 42 }` and node B depends on node A
- **THEN** node B SHALL receive `{ count: 42 }` as its `input` argument

#### Scenario: Parallel stage outputs
- **WHEN** nodes A and B are in the same parallel stage and node C depends on both
- **THEN** node C SHALL receive a merged object or the output of its direct upstream edge

