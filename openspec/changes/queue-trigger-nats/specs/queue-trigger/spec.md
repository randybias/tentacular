## ADDED Requirements

### Requirement: Queue trigger type in Go spec
The `Trigger` struct SHALL have a `Subject` field. The parser SHALL accept `"queue"` as a valid trigger type. Queue triggers MUST have a non-empty `subject` field.

#### Scenario: Valid queue trigger
- **WHEN** a workflow has a trigger with `type: queue` and `subject: events.github.push`
- **THEN** parsing succeeds and the trigger's Subject field equals "events.github.push"

#### Scenario: Queue trigger missing subject
- **WHEN** a workflow has a trigger with `type: queue` and no subject
- **THEN** parsing returns a validation error about missing subject

### Requirement: Queue trigger type in TypeScript
The `Trigger` interface SHALL include `"queue"` in its type union. It SHALL have optional `subject` and `name` fields.

#### Scenario: TypeScript Trigger accepts queue type
- **WHEN** a Trigger object has `type: "queue"` and `subject: "events.push"`
- **THEN** it is valid according to the TypeScript type system

### Requirement: NATS trigger manager
The engine SHALL provide a `startNATSTriggers()` function that connects to NATS and subscribes to subjects for each queue trigger. It SHALL return a handle with a `close()` method for graceful shutdown.

#### Scenario: NATS connection with TLS and token
- **WHEN** startNATSTriggers is called with a valid NATS URL and token
- **THEN** it connects using TLS (system trust store) and token authentication

#### Scenario: Message triggers workflow execution
- **WHEN** a message arrives on a subscribed NATS subject
- **THEN** the message payload is parsed as JSON and passed as input to the workflow executor

#### Scenario: Request-reply support
- **WHEN** a NATS message has a reply subject set
- **THEN** the workflow execution result is sent back as a reply

#### Scenario: Graceful shutdown drains subscriptions
- **WHEN** close() is called on the trigger handle
- **THEN** NATS subscriptions are drained and the connection is closed

### Requirement: NATS wiring in main.ts
The engine main.ts SHALL start NATS triggers after the HTTP server if queue triggers exist in the spec. It SHALL read `nats_url` from config and `nats.token` from secrets.

#### Scenario: Missing NATS config skips gracefully
- **WHEN** queue triggers exist but nats_url or nats.token is missing
- **THEN** the engine logs a warning and skips NATS trigger setup

#### Scenario: Signal handlers drain NATS
- **WHEN** SIGTERM or SIGINT is received
- **THEN** the engine drains NATS connections before exiting
