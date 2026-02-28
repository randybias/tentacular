## Why

The workflow engine has no runtime observability. When a workflow executes, there is no way to inspect event counts, error rates, or execution timing without attaching a debugger or parsing logs. Adding an in-memory telemetry sink enables the MCP server to query workflow health via the existing `/health` endpoint, powering Green/Amber/Red health classification for the `wf_health` tool.

## What Changes

- Add `TelemetrySink` interface with `Record(event)` and `Snapshot() TelemetrySnapshot` methods
- Implement `NoopSink` (zero-cost default) and `BasicSink` (in-memory counters with ring buffer)
- Add `NewTelemetrySink(kind string)` factory function for sink creation
- Wire telemetry recording into executor (node start/complete/error), server (request in/out), NATS trigger (message received), and main (startup)
- Enhance `/health` endpoint: plain `GET /health` returns `{"status":"ok"}` (unchanged); `GET /health?detail=1` returns a `TelemetrySnapshot` with event counts, error rates, uptime, and last-error info
- Add MCP server ingress NetworkPolicy rule in `DeriveIngressRules` allowing pods from `tentacular-system` namespace to reach workflow pods on port 8080

## Capabilities

### New Capabilities
- `telemetry-sink`: In-memory telemetry sink interface with NoopSink and BasicSink implementations, factory function, and TelemetrySnapshot type
- `health-detail`: Enhanced /health endpoint supporting ?detail=1 query parameter for runtime telemetry snapshots

### Modified Capabilities
- `k8s-deploy`: DeriveIngressRules gains an MCP ingress rule allowing tentacular-system namespace to probe workflow pods on port 8080

## Impact

- `engine/telemetry.ts`: NEW -- TelemetrySink interface, NoopSink, BasicSink, factory
- `engine/telemetry_test.ts`: NEW -- unit tests for sink implementations
- `engine/executor/simple.ts`: Record node-start, node-complete, node-error events
- `engine/server.ts`: Record request-in/request-out events; serve TelemetrySnapshot on /health?detail=1
- `engine/triggers/nats.ts`: Record nats-message-received events
- `engine/main.ts`: Create sink via factory, pass to executor/server/nats, record engine-start event
- `engine/types.ts`: Add TelemetryEvent and TelemetrySnapshot types
- `pkg/spec/derive.go`: Add MCP ingress rule to DeriveIngressRules
- `pkg/spec/derive_test.go`: Test MCP ingress rule generation
- `pkg/k8s/netpol.go`: Handle new namespace-based ingress rule rendering
- `pkg/k8s/netpol_test.go`: Test NetworkPolicy output with MCP ingress
