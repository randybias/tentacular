## 1. Design Document Authoring

- [x] 1.1 Draft Gateway sidecar architecture: pod layout, sidecar pattern rationale, loopback communication, Envoy selection
- [x] 1.2 Draft two-tier secrets model: Tier 1 (Gateway-only service credentials) vs Tier 2 (workflow-accessible config), volume mount isolation
- [x] 1.3 Draft secret injection protocol: path-based service routing, service registry YAML format, supported auth types (bearer_token, api_key_header, basic_auth, custom_header), metadata headers (X-PD-Workflow, X-PD-Node, X-PD-Correlation)
- [x] 1.4 Draft audit logging format: structured JSON schema for HTTP requests, rate-limited requests, and MCP tool calls; stdout output target
- [x] 1.5 Draft NetworkPolicy template: egress rules for DNS, loopback, K8s API; document CNI loopback caveat
- [x] 1.6 Draft MCP proxy design: HTTP bridge pattern, MCP server registry with transport types (http, sse, stdio), Context interface `mcp()` method, McpResult type
- [x] 1.7 Draft rate limiting design: token bucket algorithm, per-service per-workflow buckets, workflow.yaml `gateway.rate_limits` configuration, Envoy local rate limit filter mapping, 429 response with Retry-After header
- [x] 1.8 Draft migration path: four-phase rollout (Gateway without NetworkPolicy, NetworkPolicy enforcement, MCP activation, rate limiting), PD_GATEWAY_ENABLED feature flag, local dev fallback

## 2. Design Integration and Consistency

- [x] 2.1 Verify ctx.fetch refactoring design is backward-compatible with current `engine/context/mod.ts` implementation
- [x] 2.2 Verify MCP Context interface extension (ctx.mcp method) does not break existing Context type contract
- [x] 2.3 Verify rate limit configuration in workflow.yaml is compatible with existing WorkflowConfig type structure
- [x] 2.4 Verify audit log correlation ID approach aligns with existing node ID and workflow ID conventions

## 3. Design Review and Validation

- [x] 3.1 Review that all request flow diagrams are consistent with the architecture section
- [x] 3.2 Review that the service registry YAML format covers all four auth types with examples
- [x] 3.3 Review that the NetworkPolicy template correctly handles the loopback/CNI caveat
- [x] 3.4 Review that the migration phases are ordered correctly with proper dependency between phases
- [x] 3.5 Review risks/trade-offs section covers: Envoy complexity, loopback latency, MCP stdio transport, migration risk, rate limiting accuracy, secret rotation
