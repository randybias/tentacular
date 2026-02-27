## 1. Bootstrap Phase (Direct K8s)

- [ ] 1.1 Identify operations in `pkg/cli/cluster.go` that must run before MCP server exists.
- [ ] 1.2 Embed MCP server deployment manifests in CLI (Deployment, Service, RBAC) using `embed.FS`.
- [ ] 1.3 Implement bootstrap: apply MCP server manifests, wait for readiness, verify health.

## 2. Configuration Phase (MCP-Routed)

- [ ] 2.1 Add `cluster_install` MCP tool in `../tentacular-mcp/pkg/tools/clusterops.go`.
- [ ] 2.2 Implement `handleClusterInstall()`: ensure standard namespaces, verify gVisor, apply default network policies.
- [ ] 2.3 Make `cluster_install` idempotent (safe to re-run).

## 3. CLI Integration

- [ ] 3.1 Refactor `pkg/cli/cluster.go` to split into bootstrap and configure phases.
- [ ] 3.2 Bootstrap phase: direct K8s calls for MCP server deployment.
- [ ] 3.3 Configure phase: MCP client calls for post-bootstrap setup.
- [ ] 3.4 Refactor `tntc cluster check` to use `mcp.Client.ClusterHealth()` when available.

## 4. Tests

- [ ] 4.1 Test bootstrap phase with mock K8s client.
- [ ] 4.2 Test configuration phase with mock MCP client.
- [ ] 4.3 Test cluster check with MCP routing.
- [ ] 4.4 Run `go test ./...` in both repos -- all pass.
