## Why

The Dockerfile ENTRYPOINT currently uses a broad `--allow-net` flag, granting the Deno runtime unrestricted network access. Since contract dependencies already declare which hosts and ports a workflow needs, we can derive precise `--allow-net=host:port,...` flags from the contract. This enforces network policy at the Deno process level in addition to K8s NetworkPolicy, providing defense-in-depth. Workflows without a contract keep the current permissive `--allow-net`.

## What Changes

- Add a new `DeriveDenoFlags` function in `pkg/spec/derive.go` that produces Deno permission flags from a workflow's contract
- Modify the Deployment template in `pkg/builder/k8s.go` to inject `command` and `args` when contract-derived flags are available, overriding the Dockerfile ENTRYPOINT
- Add comprehensive tests for flag derivation logic and K8s manifest generation with dynamic flags

## Capabilities

### New Capabilities
- `dynamic-deno-flags`: Derive Deno `--allow-net` permission flags from contract dependencies, inject as container command/args in Deployment

### Modified Capabilities
<!-- None -->

## Impact

- `pkg/spec/derive.go`: New `DeriveDenoFlags` function
- `pkg/spec/derive_test.go`: Tests for `DeriveDenoFlags` with various dependency configurations
- `pkg/builder/k8s.go`: Accept contract in `GenerateK8sManifests` or `DeployOptions`, inject `command`/`args` into Deployment container spec
- `pkg/builder/k8s_test.go`: Tests for Deployment with and without dynamic flags
- `docs/architecture.md`: Document Deno-level permission hardening
- `docs/node-contract.md`: Document how contract dependencies affect runtime permissions
- `docs/workflow-spec.md`: Note that contract drives Deno flags
- `docs/secrets.md`: Note security hardening interaction
- `docs/roadmap.md`: Update security hardening status
- `docs/testing.md`: Note testing of permission flags
