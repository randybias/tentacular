## ADDED Requirements

### Requirement: Health detail endpoint returns accurate snapshot
The test suite SHALL verify that GET /health?detail=1 returns a TelemetrySnapshot reflecting actual engine state after workflow execution.

#### Scenario: Snapshot after successful workflow execution
- **WHEN** a workflow executes successfully and GET /health?detail=1 is called
- **THEN** the response SHALL be HTTP 200 with a JSON body containing total_events > 0, error_rate == 0, uptime_seconds > 0, and last_error == null

#### Scenario: Snapshot after failed workflow execution
- **WHEN** a workflow with a failing node executes and GET /health?detail=1 is called
- **THEN** the response SHALL be HTTP 200 with a JSON body containing error_rate > 0 and last_error with a non-null message and timestamp

#### Scenario: Snapshot event counts match actual execution
- **WHEN** a two-node workflow executes and GET /health?detail=1 is called
- **THEN** the total_events field SHALL equal the sum of all recorded telemetry events (engine-start + node events + request events)

### Requirement: Plain health endpoint backwards compatibility
The test suite SHALL verify that GET /health (without detail parameter) continues to return the original response format.

#### Scenario: Plain health check response unchanged
- **WHEN** GET /health is called (no query parameters)
- **THEN** the response SHALL be HTTP 200 with body exactly {"status":"ok"}

#### Scenario: Health check with detail=0 returns plain response
- **WHEN** GET /health?detail=0 is called
- **THEN** the response SHALL be HTTP 200 with body exactly {"status":"ok"}
