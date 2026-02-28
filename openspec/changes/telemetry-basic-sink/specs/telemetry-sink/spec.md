## ADDED Requirements

### Requirement: TelemetrySink interface
The system SHALL define a `TelemetrySink` interface with `record(event: TelemetryEvent): void` and `snapshot(): TelemetrySnapshot` methods. All telemetry consumers SHALL program against this interface.

#### Scenario: Interface contract
- **WHEN** any component holds a TelemetrySink reference
- **THEN** it SHALL be able to call `record()` to emit events and `snapshot()` to read current state

### Requirement: NoopSink implementation
The system SHALL provide a `NoopSink` that implements TelemetrySink with zero-cost no-op methods. `record()` SHALL do nothing. `snapshot()` SHALL return an empty TelemetrySnapshot with all counters at zero.

#### Scenario: NoopSink record is zero-cost
- **WHEN** `record()` is called on NoopSink
- **THEN** the call SHALL return immediately with no allocations or side effects

#### Scenario: NoopSink snapshot returns empty state
- **WHEN** `snapshot()` is called on NoopSink
- **THEN** it SHALL return a TelemetrySnapshot with totalEvents=0, errorCount=0, and empty recentEvents

### Requirement: BasicSink implementation
The system SHALL provide a `BasicSink` that implements TelemetrySink with in-memory counters and a fixed-size ring buffer (default capacity 1000). `record()` SHALL increment counters and append to the ring buffer. `snapshot()` SHALL return a TelemetrySnapshot with current counter values, computed error rate, uptime, and recent events from the ring buffer.

#### Scenario: BasicSink records and counts events
- **WHEN** 5 events are recorded including 2 with type "node-error"
- **THEN** `snapshot()` SHALL return totalEvents=5 and errorCount=2

#### Scenario: BasicSink ring buffer wraps at capacity
- **WHEN** more than 1000 events are recorded
- **THEN** `snapshot().recentEvents` SHALL contain only the most recent 1000 events
- **AND** `snapshot().totalEvents` SHALL reflect the true total count

#### Scenario: BasicSink computes error rate
- **WHEN** 100 events are recorded with 10 node-error events
- **THEN** `snapshot().errorRate` SHALL equal 0.1

#### Scenario: BasicSink tracks uptime
- **WHEN** BasicSink is created and then snapshot() is called after some time
- **THEN** `snapshot().uptimeMs` SHALL be greater than zero and reflect elapsed time since creation

#### Scenario: BasicSink tracks last error
- **WHEN** a node-error event is recorded with metadata containing an error message
- **THEN** `snapshot().lastError` SHALL contain the error message and timestamp

### Requirement: TelemetryEvent type
The system SHALL define a `TelemetryEvent` type with fields: `type` (string), `timestamp` (number, epoch ms), and optional `metadata` (Record<string, unknown>). Valid event types SHALL include: `"engine-start"`, `"node-start"`, `"node-complete"`, `"node-error"`, `"request-in"`, `"request-out"`, `"nats-message"`.

#### Scenario: Event structure
- **WHEN** a TelemetryEvent is created
- **THEN** it SHALL have a `type` string, a `timestamp` number, and an optional `metadata` object

### Requirement: TelemetrySnapshot type
The system SHALL define a `TelemetrySnapshot` type with fields: `totalEvents` (number), `errorCount` (number), `errorRate` (number 0-1), `uptimeMs` (number), `lastError` (string | null), `lastErrorAt` (number | null), `recentEvents` (TelemetryEvent[]).

#### Scenario: Snapshot structure
- **WHEN** `snapshot()` is called on any TelemetrySink
- **THEN** the result SHALL conform to the TelemetrySnapshot type with all required fields

### Requirement: Factory function
The system SHALL provide `NewTelemetrySink(kind: string): TelemetrySink`. It SHALL return `NoopSink` for `"noop"`, `BasicSink` for `"basic"`, and `BasicSink` as default for any unrecognized kind.

#### Scenario: Factory returns correct sink type
- **WHEN** `NewTelemetrySink("noop")` is called
- **THEN** it SHALL return a NoopSink instance

#### Scenario: Factory defaults to BasicSink
- **WHEN** `NewTelemetrySink("unknown")` is called
- **THEN** it SHALL return a BasicSink instance
