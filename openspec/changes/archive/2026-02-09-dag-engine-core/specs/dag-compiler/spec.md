## ADDED Requirements

### Requirement: DAG compilation produces topologically sorted stages
The `compile()` function SHALL accept a `WorkflowSpec` and return a `CompiledDAG` containing topologically sorted stages. Nodes in the same stage have no dependencies on each other and can execute in parallel.

#### Scenario: Single node workflow
- **WHEN** a workflow has one node "a" and no edges
- **THEN** the compiled DAG SHALL have one stage containing ["a"] and nodeOrder ["a"]

#### Scenario: Linear chain (a -> b -> c)
- **WHEN** a workflow has nodes "a", "b", "c" with edges a->b, b->c
- **THEN** the compiled DAG SHALL have three stages: [["a"], ["b"], ["c"]]
- **AND** nodeOrder SHALL be ["a", "b", "c"]

#### Scenario: Fan-out (a -> b, a -> c)
- **WHEN** a workflow has nodes "a", "b", "c" with edges a->b, a->c
- **THEN** stage 0 SHALL contain ["a"]
- **AND** stage 1 SHALL contain ["b", "c"] (in alphabetical order)
- **AND** b and c can execute in parallel

#### Scenario: Fan-in (a -> c, b -> c)
- **WHEN** a workflow has nodes "a", "b", "c" with edges a->c, b->c
- **THEN** stage 0 SHALL contain ["a", "b"] (in alphabetical order)
- **AND** stage 1 SHALL contain ["c"]

#### Scenario: Diamond pattern (a -> b, a -> c, b -> d, c -> d)
- **WHEN** a workflow has nodes "a", "b", "c", "d" with edges a->b, a->c, b->d, c->d
- **THEN** stage 0 SHALL contain ["a"]
- **AND** stage 1 SHALL contain ["b", "c"]
- **AND** stage 2 SHALL contain ["d"]

#### Scenario: Independent nodes with no edges
- **WHEN** a workflow has nodes "a", "b", "c" with no edges
- **THEN** all nodes SHALL be in a single stage (can execute in parallel)

### Requirement: Cycle detection
The compiler SHALL detect cycles in the workflow DAG and throw an error.

#### Scenario: Direct cycle (a -> b -> a)
- **WHEN** a workflow has edges a->b, b->a
- **THEN** `compile()` SHALL throw an Error with message containing "Cycle detected"

#### Scenario: Indirect cycle (a -> b -> c -> a)
- **WHEN** a workflow has edges a->b, b->c, c->a
- **THEN** `compile()` SHALL throw an Error with message containing "Cycle detected"

### Requirement: Edge validation
The compiler SHALL validate that all edges reference defined nodes and reject self-loops.

#### Scenario: Edge references undefined node
- **WHEN** an edge references a node not defined in `spec.nodes`
- **THEN** `compile()` SHALL throw an Error with message containing "undefined node"

#### Scenario: Self-loop detection
- **WHEN** an edge has the same `from` and `to` value
- **THEN** `compile()` SHALL throw an Error with message containing "Self-loop"

### Requirement: Deterministic output
The compiler SHALL produce deterministic output for the same input, with nodes within a stage sorted alphabetically.

#### Scenario: Alphabetical stage ordering
- **WHEN** nodes "zebra" and "alpha" are in the same stage
- **THEN** they SHALL appear as ["alpha", "zebra"] in the stage's nodes array

#### Scenario: Deterministic nodeOrder
- **WHEN** the same `WorkflowSpec` is compiled multiple times
- **THEN** the `nodeOrder` array SHALL be identical each time
