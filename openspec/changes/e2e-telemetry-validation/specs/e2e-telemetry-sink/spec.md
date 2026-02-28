## ADDED Requirements

### Requirement: Executor telemetry event recording
The test suite SHALL verify that BasicSink records node-start, node-complete, and node-error events when a workflow executes through the SimpleExecutor.

#### Scenario: Successful two-node workflow records all events
- **WHEN** a two-node linear workflow executes successfully through SimpleExecutor with a BasicSink
- **THEN** the sink snapshot SHALL contain node-start and node-complete events for each node, and the total event count SHALL equal 4 (2 starts + 2 completes)

#### Scenario: Node error produces node-error event
- **WHEN** a workflow with a failing node executes through SimpleExecutor with a BasicSink
- **THEN** the sink snapshot SHALL contain a node-error event with the failing node name in metadata, and the error rate SHALL be greater than 0

#### Scenario: Multi-node DAG records events in topological order
- **WHEN** a three-node DAG workflow (A -> B, A -> C) executes through SimpleExecutor with a BasicSink
- **THEN** the sink snapshot SHALL contain 6 events (3 starts + 3 completes), and node-start for B and C SHALL appear after node-complete for A in the ring buffer

### Requirement: Server telemetry event recording
The test suite SHALL verify that the HTTP server records request-in and request-out events via BasicSink.

#### Scenario: POST /run records request events
- **WHEN** a POST /run request is sent to a running engine server with a BasicSink
- **THEN** the sink snapshot SHALL contain request-in and request-out events for the /run path

### Requirement: Engine startup event recording
The test suite SHALL verify that engine startup records an engine-start event.

#### Scenario: Engine start event on boot
- **WHEN** the engine main module initializes with a BasicSink
- **THEN** the sink snapshot SHALL contain exactly one engine-start event recorded before any other events
