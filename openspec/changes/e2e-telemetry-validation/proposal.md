## Why

The telemetry sink, health detail endpoint, and MCP ingress NetworkPolicy have been implemented but lack end-to-end validation. Unit tests exist for individual components, but there are no integration or E2E tests that verify the full chain: workflow execution produces telemetry events, the /health?detail=1 endpoint returns accurate snapshots, and the MCP ingress NetworkPolicy allows tentacular-system pods to probe workflow pods. Without E2E validation, regressions in the telemetry pipeline will go undetected.

## What Changes

- Add E2E test suite for the telemetry sink: verify BasicSink records events correctly across executor, server, and NATS trigger wiring points
- Add E2E test for the /health?detail=1 endpoint: start a workflow engine, execute a workflow, then verify the snapshot contains accurate event counts, error rates, and timing
- Add E2E test for /health (no detail) backwards compatibility: ensure plain health check still returns {"status":"ok"}
- Add integration test for MCP ingress NetworkPolicy: verify DeriveIngressRules produces the tentacular-system namespace selector and that rendered NetworkPolicy YAML matches expected structure
- Add a test workflow fixture for telemetry validation scenarios (success path, error path, multi-node DAG)

## Capabilities

### New Capabilities
- `e2e-telemetry-sink`: End-to-end test suite validating telemetry event recording across all wiring points (executor, server, NATS trigger) with BasicSink
- `e2e-health-endpoint`: End-to-end test suite for /health and /health?detail=1 endpoints verifying snapshot accuracy after workflow execution
- `e2e-netpol-validation`: Integration test suite verifying MCP ingress NetworkPolicy rule generation and rendered YAML output

### Modified Capabilities
<!-- None -->

## Impact

- `engine/testing/`: New test fixtures for telemetry E2E scenarios (success, error, multi-node workflows)
- `engine/telemetry_test.ts`: Extend with integration-level tests covering full event flow through executor and server
- `engine/server.ts` tests: Add /health?detail=1 E2E tests with running engine instance
- `pkg/spec/derive_test.go`: Extend with cross-cutting NetworkPolicy integration tests
- `pkg/k8s/netpol_test.go`: Extend with full YAML rendering validation for MCP ingress
- CI configuration: May need test target for E2E suite execution
