## 1. Telemetry Types and Sink Interface

- [ ] 1.1 Add `TelemetryEvent` and `TelemetrySnapshot` types to `engine/types.ts`
- [ ] 1.2 Create `engine/telemetry.ts` with `TelemetrySink` interface, `NoopSink`, and `BasicSink` (ring buffer, counters, error rate, uptime)
- [ ] 1.3 Add `NewTelemetrySink(kind: string)` factory function in `engine/telemetry.ts`

## 2. Telemetry Wiring

- [ ] 2.1 Wire `TelemetrySink` into `SimpleExecutor` -- record `node-start`, `node-complete`, `node-error` events with node name in metadata
- [ ] 2.2 Wire `TelemetrySink` into `server.ts` -- record `request-in`/`request-out` on `/run`, serve `TelemetrySnapshot` on `GET /health?detail=1`
- [ ] 2.3 Wire `TelemetrySink` into `engine/triggers/nats.ts` -- record `nats-message` event with subject in metadata
- [ ] 2.4 Wire `TelemetrySink` in `engine/main.ts` -- create sink from `TELEMETRY_SINK` env var (default `"basic"`), pass to executor/server/nats, record `engine-start` event

## 3. MCP Ingress NetworkPolicy Rule

- [ ] 3.1 Add unconditional MCP ingress rule in `pkg/spec/derive.go` `DeriveIngressRules()` -- allow TCP 8080 from `tentacular-system` namespace via `namespaceSelector`
- [ ] 3.2 Update `pkg/k8s/netpol.go` to render the new namespace-selector-only ingress rule
- [ ] 3.3 Add tests in `pkg/spec/derive_test.go` for MCP ingress rule (both webhook and non-webhook workflows)
- [ ] 3.4 Add tests in `pkg/k8s/netpol_test.go` for NetworkPolicy YAML output with MCP ingress rule

## 4. Engine Tests

- [ ] 4.1 Create `engine/telemetry_test.ts` -- unit tests for NoopSink, BasicSink (counters, ring buffer wrap, error rate, uptime, last error), and factory
- [ ] 4.2 Add tests for `/health?detail=1` response in server tests
- [ ] 4.3 Verify existing `/health` endpoint returns `{"status":"ok"}` unchanged (backwards compatibility)

## 5. Verification

- [ ] 5.1 Run `go test ./pkg/spec/...` -- all pass
- [ ] 5.2 Run `go test ./pkg/k8s/...` -- all pass
- [ ] 5.3 Run Deno engine tests -- all pass
