## 1. proxy_status Tool

- [ ] 1.1 Add `ProxyStatusParams` struct with optional Namespace field (default: `tentacular-system`).
- [ ] 1.2 Add `ProxyStatusResult` struct: Ready (bool), Replicas, Image, StorageMode, PVCSize, Endpoint, Age.
- [ ] 1.3 Implement `handleProxyStatus()`: query esm-sh Deployment and Service, report status.
- [ ] 1.4 Handle missing proxy gracefully (return `installed: false` instead of error).

## 2. proxy_reconcile Tool

- [ ] 2.1 Add `ProxyReconcileParams` struct: Namespace (optional), Image (optional), Storage (optional: emptydir/pvc), PVCSize (optional).
- [ ] 2.2 Add `ProxyReconcileResult` struct: Action (created/updated/unchanged), Namespace, Image, StorageMode.
- [ ] 2.3 Implement proxy manifest generation in MCP server (Deployment, Service, NetworkPolicy for esm.sh), mirroring `k8s.GenerateModuleProxyManifests()`.
- [ ] 2.4 Implement `handleProxyReconcile()`: generate desired manifests, compare against live state, apply patches for drift, create if missing.
- [ ] 2.5 Ensure NetworkPolicy includes egress to jsr.io:443, registry.npmjs.org:443, cdn.deno.land:443 and ingress from all namespaces on port 8080.
- [ ] 2.6 Handle storage mode: emptydir (default) or PVC with configurable size.

## 3. Registration

- [ ] 3.1 Create `registerProxyTools()` function in `proxy.go`.
- [ ] 3.2 Add `registerProxyTools(srv, client)` call in `register.go`.

## 4. CLI Integration

- [ ] 4.1 Update `pkg/cli/cluster.go` to route `--module-proxy` through `mcp.Client.ProxyReconcile()` (MCP mandatory per ADR-3).
- [ ] 4.2 Pass CLI flags (image, storage, pvc-size, namespace) as `ProxyReconcileParams`.
- [ ] 4.3 Bootstrap exception: during `tntc cluster install`, the proxy is installed via direct K8s as part of the pre-MCP bootstrap phase (Phase 5). This is not a general fallback -- it only applies when the MCP server itself is being deployed for the first time.

## 5. Tests

- [ ] 5.1 Create `proxy_test.go` in tentacular-mcp: test proxy_status with proxy present and absent.
- [ ] 5.2 Test proxy_reconcile: create from scratch, update drifted image, no-op when unchanged.
- [ ] 5.3 Test manifest generation matches expected Deployment, Service, NetworkPolicy specs.
- [ ] 5.4 Run `go test ./pkg/tools/...` -- all pass.
- [ ] 5.5 Run `go test ./...` -- all pass.
