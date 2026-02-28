## ADDED Requirements

### Requirement: Health endpoint detail mode
The server SHALL support a `detail` query parameter on `GET /health`. When `detail=1` is present, the response SHALL include the full TelemetrySnapshot from the configured TelemetrySink. When `detail` is absent or not `"1"`, the response SHALL remain `{"status":"ok"}` (backwards compatible).

#### Scenario: Plain health check unchanged
- **WHEN** `GET /health` is called without query parameters
- **THEN** the response SHALL be `{"status":"ok"}` with HTTP 200

#### Scenario: Detailed health returns telemetry snapshot
- **WHEN** `GET /health?detail=1` is called
- **THEN** the response SHALL be a JSON object containing `status`, `totalEvents`, `errorCount`, `errorRate`, `uptimeMs`, `lastError`, `lastErrorAt`, and `recentEvents` fields with HTTP 200

#### Scenario: Detail mode includes status field
- **WHEN** `GET /health?detail=1` is called
- **THEN** the response SHALL include `"status": "ok"` alongside the telemetry fields

### Requirement: Executor telemetry wiring
The executor SHALL record telemetry events for node lifecycle: `"node-start"` when a node begins execution, `"node-complete"` when a node finishes successfully, and `"node-error"` when a node fails. Each event SHALL include the node name in metadata.

#### Scenario: Node execution records start and complete events
- **WHEN** a node executes successfully
- **THEN** the executor SHALL record a `"node-start"` event followed by a `"node-complete"` event, both with `metadata.node` set to the node name

#### Scenario: Node failure records error event
- **WHEN** a node execution fails
- **THEN** the executor SHALL record a `"node-start"` event followed by a `"node-error"` event with `metadata.node` and `metadata.error` set

### Requirement: Server telemetry wiring
The server SHALL record `"request-in"` when an HTTP request arrives at `/run` and `"request-out"` when the response is sent. Events SHALL include the request path in metadata.

#### Scenario: Run request records in/out events
- **WHEN** `POST /run` is called and completes
- **THEN** the server SHALL have recorded one `"request-in"` and one `"request-out"` event

### Requirement: NATS trigger telemetry wiring
The NATS trigger handler SHALL record a `"nats-message"` event when a NATS message is received. The event SHALL include the subject in metadata.

#### Scenario: NATS message records telemetry event
- **WHEN** a NATS message is received on subject "test.subject"
- **THEN** a `"nats-message"` event SHALL be recorded with `metadata.subject` set to `"test.subject"`

### Requirement: Engine startup telemetry
The main entry point SHALL record an `"engine-start"` event when the engine starts up.

#### Scenario: Engine start records event
- **WHEN** the engine starts
- **THEN** an `"engine-start"` event SHALL be recorded with the workflow name in metadata
