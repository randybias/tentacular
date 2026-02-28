## Context

The telemetry-basic-sink change added three components: (1) BasicSink in-memory telemetry recording, (2) /health?detail=1 endpoint exposing TelemetrySnapshot, (3) MCP ingress NetworkPolicy rule allowing tentacular-system to probe workflow pods. Each has unit tests, but no tests verify the full integration chain. The MCP server's wf_health tool depends on all three working together -- a regression in any component breaks the health monitoring workflow silently.

## Goals / Non-Goals

**Goals:**
- Validate the full telemetry event recording chain: executor node events -> BasicSink -> /health?detail=1 snapshot
- Verify /health backwards compatibility (plain health check unchanged)
- Verify MCP ingress NetworkPolicy rule is generated for all workflow types and renders correctly
- Provide test fixtures (success, error, multi-node DAGs) reusable by downstream E2E suites

**Non-Goals:**
- Testing the MCP server's wf_health tool (that belongs to the MCP repo)
- Live cluster NetworkPolicy enforcement testing (requires running K8s)
- Performance benchmarking of BasicSink under load
- Testing NATS trigger integration (requires running NATS server)

## Decisions

### 1. Test structure: extend existing test files vs. new E2E directory

**Decision**: Extend existing test files (`telemetry_test.ts`, `derive_test.go`, `netpol_test.go`) with clearly labeled E2E/integration test sections rather than creating a separate E2E directory.

**Rationale**: The existing test infrastructure (Deno test runner, Go test) already handles these files. A separate E2E directory adds build configuration complexity with no benefit at this scale. Tests are co-located with the code they validate.

### 2. Engine integration tests use real HTTP server

**Decision**: Spawn a real Deno HTTP server in tests for /health?detail=1 validation rather than mocking the server layer.

**Rationale**: The value of E2E tests is verifying real HTTP request/response cycles. Mocking the server defeats the purpose. The Deno test runner supports async HTTP server lifecycle cleanly.

### 3. Test fixtures as static workflow YAML + TypeScript node stubs

**Decision**: Create minimal test workflow fixtures in engine/testing/ with pre-built workflow.yaml and stub node modules for telemetry scenarios.

**Rationale**: Reusable across test suites. Static fixtures are deterministic and easy to reason about. Three fixtures cover the key paths: success (2-node linear), error (node that throws), multi-node DAG (3+ nodes with dependencies).

## Risks / Trade-offs

- [Deno test server port conflicts] -> Use dynamic port assignment (port 0) in test setup
- [Test fixtures drift from real workflow format] -> Fixtures use the same workflow.yaml schema validated by the compiler; compiler changes break fixture tests early (desired behavior)
- [NetworkPolicy YAML snapshot tests are brittle] -> Compare structural equality (parsed YAML) not string equality
