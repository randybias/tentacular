## 1. Test Fixtures

- [ ] 1.1 Create engine/testing/fixtures/ directory with a two-node linear workflow (workflow.yaml + stub nodes) for success path testing
- [ ] 1.2 Create error-path fixture with a workflow containing a node that throws an exception
- [ ] 1.3 Create multi-node DAG fixture (A -> B, A -> C) for topological ordering validation

## 2. Telemetry Sink E2E Tests

- [ ] 2.1 Add integration test: SimpleExecutor with BasicSink records node-start and node-complete events for two-node workflow (4 total events)
- [ ] 2.2 Add integration test: SimpleExecutor with BasicSink records node-error event with correct metadata for failing node
- [ ] 2.3 Add integration test: SimpleExecutor with BasicSink records events in topological order for DAG fixture
- [ ] 2.4 Add integration test: HTTP server records request-in and request-out events on POST /run
- [ ] 2.5 Add integration test: engine-start event is present and is the first event in the ring buffer

## 3. Health Endpoint E2E Tests

- [ ] 3.1 Add E2E test: start engine server on dynamic port, execute workflow, verify GET /health?detail=1 returns snapshot with total_events > 0, error_rate == 0, uptime_seconds > 0
- [ ] 3.2 Add E2E test: execute failing workflow, verify GET /health?detail=1 returns error_rate > 0 and last_error with message and timestamp
- [ ] 3.3 Add E2E test: verify total_events in snapshot matches expected count after two-node workflow execution
- [ ] 3.4 Add E2E test: verify GET /health returns {"status":"ok"} (backwards compatibility)
- [ ] 3.5 Add E2E test: verify GET /health?detail=0 returns {"status":"ok"}

## 4. NetworkPolicy Integration Tests

- [ ] 4.1 Add test: DeriveIngressRules for webhook workflow includes MCP ingress rule (tentacular-system namespace selector, port 8080) alongside webhook rule
- [ ] 4.2 Add test: DeriveIngressRules for non-webhook workflow includes MCP ingress rule
- [ ] 4.3 Add test: MCP ingress rule is present regardless of trigger type or contract configuration
- [ ] 4.4 Add test: rendered NetworkPolicy YAML contains correct namespaceSelector matchLabels for tentacular-system
- [ ] 4.5 Add test: rendered NetworkPolicy YAML parses as valid Kubernetes NetworkPolicy v1 structure

## 5. Verification

- [ ] 5.1 Run Deno engine tests -- all pass including new E2E tests
- [ ] 5.2 Run `go test ./pkg/spec/...` -- all pass including new integration tests
- [ ] 5.3 Run `go test ./pkg/k8s/...` -- all pass including new YAML rendering tests
