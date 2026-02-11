## ADDED Requirements

### Requirement: Gateway sidecar architecture defined
The design document SHALL define a Gateway sidecar container architecture where an Envoy-based reverse proxy runs alongside the workflow engine container in the same Kubernetes pod.

#### Scenario: Sidecar pod layout
- **GIVEN** a workflow deployed to Kubernetes
- **WHEN** the pod specification is examined
- **THEN** it SHALL contain two containers: the workflow engine (Deno) and the Gateway sidecar (Envoy)
- **AND** the Gateway SHALL listen on localhost:9090

#### Scenario: Loopback communication
- **GIVEN** the Gateway sidecar and workflow container in the same pod
- **WHEN** the workflow container sends a request to localhost:9090
- **THEN** the Gateway SHALL receive and process the request without requiring mTLS

### Requirement: Two-tier secrets model defined
The design document SHALL define a two-tier secrets model separating service credentials (Tier 1) from workflow configuration (Tier 2).

#### Scenario: Tier 1 secrets isolation
- **GIVEN** the two-tier secrets model
- **WHEN** a Tier 1 secret (e.g., API key, OAuth token) is examined
- **THEN** it SHALL be mounted only into the Gateway sidecar container
- **AND** the workflow container SHALL NOT have access to the raw secret value

#### Scenario: Tier 2 config accessibility
- **GIVEN** the two-tier secrets model
- **WHEN** a Tier 2 configuration value (e.g., feature flag, batch size) is examined
- **THEN** it SHALL be accessible to the workflow container via `ctx.config`

#### Scenario: Tier separation rationale
- **GIVEN** a compromised or malicious workflow node
- **WHEN** it attempts to access service credentials
- **THEN** no Tier 1 secrets SHALL be available in the workflow container's memory or filesystem

### Requirement: Secret injection protocol defined
The design document SHALL specify how the Gateway injects Tier 1 credentials into outbound HTTP requests based on service name routing.

#### Scenario: Path-based service routing
- **GIVEN** a `ctx.fetch("stripe", "/v1/charges")` call from node code
- **WHEN** the workflow engine sends the request to the Gateway
- **THEN** the request SHALL be sent to `http://localhost:9090/stripe/v1/charges`
- **AND** the Gateway SHALL extract "stripe" as the service name from the path prefix

#### Scenario: Credential injection
- **GIVEN** a service registry mapping "stripe" to upstream `api.stripe.com` with a bearer token
- **WHEN** the Gateway forwards the request to the upstream
- **THEN** it SHALL inject the `Authorization: Bearer {token}` header using the Tier 1 secret
- **AND** the upstream URL SHALL be `https://api.stripe.com/v1/charges`

#### Scenario: Service registry configuration
- **GIVEN** the Gateway service registry
- **WHEN** the registry is examined
- **THEN** each service entry SHALL include: service name, upstream URL, K8s Secret reference, auth type (bearer_token, api_key_header, basic_auth, custom_header), and auth field

#### Scenario: Metadata headers
- **WHEN** the workflow engine sends a request through the Gateway
- **THEN** it SHALL include metadata headers: `X-PD-Workflow`, `X-PD-Node`, and `X-PD-Correlation`

### Requirement: Audit logging format defined
The design document SHALL define a structured JSON audit log format for every proxied request.

#### Scenario: Successful request log entry
- **GIVEN** a successful HTTP request proxied through the Gateway
- **WHEN** the audit log entry is examined
- **THEN** it SHALL contain: timestamp, level, type ("gateway_request"), correlation_id, workflow_id, workflow_version, node_id, service, method, path, upstream_url, status_code, request_size_bytes, response_size_bytes, latency_ms, rate_limited (false), rate_limit_remaining, and error (null)

#### Scenario: Rate-limited request log entry
- **GIVEN** a request that exceeds the rate limit
- **WHEN** the audit log entry is examined
- **THEN** it SHALL have type "gateway_request", rate_limited set to true, status_code 429, and an error message describing the rate limit exceeded

#### Scenario: MCP tool call log entry
- **GIVEN** an MCP tool call proxied through the Gateway
- **WHEN** the audit log entry is examined
- **THEN** it SHALL have type "gateway_mcp" and include: mcp_tool, mcp_server, mcp_method, status, and latency_ms

#### Scenario: Log output target
- **WHEN** the Gateway produces audit logs
- **THEN** they SHALL be written to stdout as JSON lines (one JSON object per line)

### Requirement: NetworkPolicy template defined
The design document SHALL provide a Kubernetes NetworkPolicy template that restricts workflow container egress to only the Gateway sidecar and essential cluster services.

#### Scenario: DNS access
- **GIVEN** the NetworkPolicy is applied
- **WHEN** the workflow container attempts DNS resolution
- **THEN** UDP and TCP port 53 traffic SHALL be allowed

#### Scenario: Direct external access blocked
- **GIVEN** the NetworkPolicy is applied
- **WHEN** the workflow container attempts to connect directly to an external API (e.g., api.github.com)
- **THEN** the connection SHALL be denied by the NetworkPolicy

#### Scenario: Gateway loopback access
- **GIVEN** the NetworkPolicy is applied
- **WHEN** the workflow container connects to localhost:9090 (Gateway sidecar)
- **THEN** the connection SHALL be allowed (loopback traffic is not filtered by NetworkPolicy)

### Requirement: MCP proxy design defined
The design document SHALL define how MCP tool calls are proxied through the Gateway with the same auth injection and audit logging as HTTP calls.

#### Scenario: MCP HTTP bridge
- **GIVEN** a node calling `ctx.mcp("web-search", { query: "..." })`
- **WHEN** the request is sent to the Gateway
- **THEN** it SHALL be sent as HTTP POST to `http://localhost:9090/_mcp/web-search` with the MCP request as JSON body

#### Scenario: MCP server registry
- **GIVEN** the Gateway MCP server registry
- **WHEN** the registry is examined
- **THEN** each MCP server entry SHALL include: server name, transport type (http, sse, stdio), URL or command, credential references, and list of available tools

#### Scenario: MCP Context interface extension
- **GIVEN** the design for the Context interface
- **WHEN** MCP support is added
- **THEN** the Context interface SHALL include a `mcp(tool: string, params: Record<string, unknown>): Promise<McpResult>` method

#### Scenario: MCP transport abstraction
- **GIVEN** the Gateway MCP proxy
- **WHEN** it receives an MCP request
- **THEN** the Gateway SHALL handle the transport (HTTP, SSE, or stdio) transparently to the workflow container

### Requirement: Rate limiting design defined
The design document SHALL define token bucket rate limiting per service per workflow, configurable in `workflow.yaml`.

#### Scenario: Rate limit configuration
- **GIVEN** a `workflow.yaml` with a `gateway.rate_limits` section
- **WHEN** the rate limit config for a service is examined
- **THEN** it SHALL specify `requests_per_second` and `burst` parameters

#### Scenario: Default rate limit
- **GIVEN** a service not listed in `gateway.rate_limits`
- **WHEN** a request is made to that service
- **THEN** the `default_rate_limit` SHALL apply if configured

#### Scenario: Rate limit enforcement
- **GIVEN** a service with rate limit of 10 req/s and burst of 20
- **WHEN** more than 20 requests arrive in a burst
- **THEN** the Gateway SHALL return HTTP 429 with a `Retry-After` header for excess requests

#### Scenario: Independent service buckets
- **GIVEN** rate limits configured for "stripe" and "github"
- **WHEN** the stripe rate limit is exhausted
- **THEN** the github rate limit SHALL NOT be affected

### Requirement: Migration path defined
The design document SHALL define a phased migration path from file-mount secrets to Gateway sidecar with backward compatibility.

#### Scenario: Phase 1 backward compatibility
- **GIVEN** Phase 1 of the migration (Gateway deployed, no NetworkPolicy)
- **WHEN** a workflow uses `ctx.fetch`
- **THEN** requests SHALL route through the Gateway when `PD_GATEWAY_ENABLED=true`
- **AND** the workflow SHALL fall back to direct HTTP when the env var is not set

#### Scenario: Phase 2 enforcement
- **GIVEN** Phase 2 of the migration (NetworkPolicy applied)
- **WHEN** a workflow container attempts direct external access
- **THEN** the connection SHALL be blocked by NetworkPolicy

#### Scenario: Local development unchanged
- **GIVEN** local development mode (`tntc dev`)
- **WHEN** the Gateway is not available
- **THEN** `ctx.fetch` SHALL use the current file-mount secrets behavior (direct HTTP with injected credentials)

#### Scenario: Feature flag rollout
- **GIVEN** the migration feature flag
- **WHEN** `PD_GATEWAY_ENABLED` environment variable is examined
- **THEN** it SHALL control whether `ctx.fetch` routes through the Gateway or uses direct HTTP
