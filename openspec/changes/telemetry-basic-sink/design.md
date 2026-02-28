## Context

The workflow engine exposes a simple `GET /health` returning `{"status":"ok"}`. There is no runtime telemetry -- no event counts, error rates, or execution timing. The MCP server (tentacular-mcp-telemetry) needs to query workflow health to power its `wf_health` tool with Green/Amber/Red classification. This requires both an enhanced health endpoint on the engine side and a NetworkPolicy ingress rule allowing the MCP server (in `tentacular-system` namespace) to reach workflow pods on port 8080.

## Goals / Non-Goals

**Goals:**
- Define a `TelemetrySink` interface that decouples telemetry recording from consumption
- Provide `NoopSink` (zero overhead when telemetry is disabled) and `BasicSink` (in-memory counters + ring buffer)
- Wire telemetry events into executor, server, NATS trigger, and main startup
- Enhance `/health?detail=1` to return a `TelemetrySnapshot` with counters, error rate, uptime, and last-error
- Add NetworkPolicy ingress rule for MCP server health probes from `tentacular-system`

**Non-Goals:**
- Persistent telemetry storage (disk, database)
- Prometheus metrics or OpenTelemetry export
- Distributed tracing or span correlation
- Alerting or threshold-based notifications
- Dashboard or UI for telemetry data

## Decisions

### In-memory ring buffer for BasicSink
BasicSink uses a fixed-size ring buffer (default 1000 entries) for recent events. This bounds memory usage regardless of workflow throughput. Alternative: unbounded array with periodic pruning -- rejected because it creates GC pressure spikes and requires a background timer.

### Factory function for sink creation
`NewTelemetrySink(kind: string)` returns `NoopSink` for `"noop"` and `BasicSink` for `"basic"`. The engine main reads `TELEMETRY_SINK` env var (default `"basic"`). This keeps sink selection out of the executor/server code. Alternative: constructor injection without factory -- rejected because it pushes config logic into main.ts wiring.

### Snapshot on /health?detail=1 (additive, non-breaking)
Plain `GET /health` continues returning `{"status":"ok"}` for Kubernetes liveness probes. Adding `?detail=1` returns the full `TelemetrySnapshot`. This avoids breaking existing health checks. Alternative: separate `/telemetry` endpoint -- rejected to avoid a new NetworkPolicy port and keep the surface area minimal.

### MCP ingress via namespace selector in DeriveIngressRules
Every workflow gets an ingress rule allowing TCP 8080 from `tentacular-system` namespace. This is unconditional (not gated by a contract field) because MCP health probes are infrastructure-level, not workflow-specific. The rule uses `namespaceSelector` with `kubernetes.io/metadata.name: tentacular-system`. Alternative: per-workflow opt-in via contract field -- rejected because health monitoring should be universal.

### Event types as string constants
Events use string type identifiers (`"node-start"`, `"node-complete"`, `"node-error"`, `"request-in"`, `"request-out"`, `"nats-message"`, `"engine-start"`). This keeps the TypeScript interface simple and avoids an enum dependency. Metadata is an optional `Record<string, unknown>` on each event.

## Risks / Trade-offs

- **Memory usage under high throughput**: BasicSink ring buffer is fixed at 1000 entries (~200KB). If a workflow processes thousands of events per second, the buffer wraps quickly and older events are lost. Mitigation: the snapshot includes aggregate counters that survive buffer wraps; the ring buffer is only for recent-event inspection.
- **No persistence across restarts**: All telemetry is lost when the pod restarts. Mitigation: this is acceptable for v1 -- the MCP server queries live health, not historical data.
- **Unconditional MCP ingress rule**: Every workflow pod allows ingress from tentacular-system even if no MCP server is deployed. Mitigation: the rule is harmless if no MCP pod exists; it only opens port 8080 from a specific namespace.
- **Clock drift in uptime**: Uptime is calculated from `Date.now()` at engine start. Container clock skew could affect uptime reporting. Mitigation: Kubernetes nodes sync clocks via NTP; sub-second accuracy is sufficient for health classification.
