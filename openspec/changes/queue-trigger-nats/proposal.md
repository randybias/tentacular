## Why

Queue triggers enable event-driven workflows via NATS message subscriptions. The engine currently only responds to HTTP requests. Adding NATS support allows workflows to react to events (e.g., GitHub pushes, CI completions) published to NATS subjects, enabling asynchronous, decoupled workflow execution.

## What Changes

- Add `Subject` field to `Trigger` struct and `"queue"` to valid trigger types
- Validate queue triggers require a subject
- Add `"queue"` to TypeScript Trigger type union, plus `subject` and `name` fields
- Add `@nats-io/transport-deno` dependency to engine
- New `engine/triggers/nats.ts`: NATS subscription manager with TLS + token auth
- Wire up NATS triggers in `engine/main.ts` with graceful shutdown

## Capabilities

### New Capabilities
- `queue-trigger`: Subscribe to NATS subjects for queue-type triggers, executing workflows on message receipt

### Modified Capabilities
<!-- None -->

## Impact

- `pkg/spec/types.go`: Trigger gains Subject field
- `pkg/spec/parse.go`: queue type + subject validation
- `pkg/spec/parse_test.go`: queue trigger tests
- `engine/types.ts`: Trigger type union updated
- `engine/deno.json`: NATS dependency added
- `engine/triggers/nats.ts`: NEW — NATS trigger manager
- `engine/triggers/nats_test.ts`: NEW — unit + integration tests
- `engine/main.ts`: NATS trigger wiring + graceful shutdown
