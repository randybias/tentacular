## Context

Pipedreamer v2 has a working execution engine: workflows defined in `workflow.yaml` are compiled into a DAG, executed by the `SimpleExecutor`, and nodes interact with external services via `ctx.fetch(service, path)`. The current `ctx.fetch` implementation in `engine/context/mod.ts` loads secrets from `.secrets.yaml` (local dev) or a K8s Secret volume mount (production), injects auth headers directly, and makes outbound HTTP calls from the workflow container.

This works for local development but has security and operational gaps for production deployments: secrets are exposed to node code in-memory, there is no centralized audit trail of outbound calls, no rate limiting, no NetworkPolicy enforcement, and no support for MCP tool calls. The Gateway sidecar architecture addresses all of these by interposing a reverse proxy between workflow containers and external services.

This is a **design-only** change. The deliverable is this architecture document. No code is implemented. Future changes will implement the Gateway based on this design.

## Goals / Non-Goals

**Goals:**
- Define the Gateway sidecar container architecture and its relationship to the workflow pod
- Design a two-tier secrets model that keeps service credentials out of workflow container memory
- Specify the secret injection protocol for outbound HTTP requests
- Define a structured audit logging format for all proxied requests
- Provide a Kubernetes NetworkPolicy template that enforces Gateway-only egress
- Design MCP tool call proxying through the Gateway
- Design rate limiting with token bucket algorithm, configurable per service
- Define a backward-compatible migration path from file-mount secrets to Gateway sidecar

**Non-Goals:**
- Implementing any code (this is design only)
- Designing the Temporal executor integration (separate change)
- Designing multi-cluster or multi-region Gateway topologies
- Designing Gateway high availability or horizontal scaling (single sidecar per pod is sufficient)
- Designing a control plane or Gateway management UI
- mTLS between Gateway and workflow container (loopback interface, same pod)

## Decisions

### Decision 1: Sidecar pattern (not standalone proxy)

**Choice:** The Gateway runs as a sidecar container in the same Kubernetes pod as the workflow engine container.

**Rationale:** The sidecar pattern ensures a 1:1 relationship between workflow execution and Gateway instance. Communication between the workflow container and Gateway happens over localhost (loopback interface within the pod network namespace), eliminating the need for mTLS on that link. The sidecar starts and stops with the workflow pod, so there is no lifecycle management complexity. Alternative considered: a shared Gateway deployment (one Gateway for all workflows) -- rejected because it creates a single point of failure, complicates per-workflow rate limiting, and requires mTLS for inter-pod communication.

### Decision 2: Two-tier secrets model

**Choice:** Separate secrets into two tiers:
- **Tier 1 (Gateway-managed):** Service credentials (API keys, OAuth tokens, mTLS certs) are mounted only into the Gateway container. The workflow container never sees these values. The Gateway injects them into outbound requests based on service name mapping.
- **Tier 2 (Workflow-accessible):** Non-sensitive workflow configuration (feature flags, environment-specific URLs, tuning parameters) remains accessible to the workflow container via `ctx.config` or direct K8s ConfigMap/Secret mount.

**Rationale:** The primary security threat is a compromised or malicious node exfiltrating credentials. By keeping Tier 1 secrets in the Gateway only, even a fully compromised workflow container cannot access service credentials. Tier 2 exists because not all configuration is sensitive -- feature flags, batch sizes, and timeout values do not need Gateway protection. Alternative considered: all secrets through Gateway only -- rejected because it would require proxying even simple config lookups, adding latency for non-sensitive data.

### Decision 3: HTTP reverse proxy with service-name routing

**Choice:** The Gateway exposes an HTTP endpoint on `localhost:9090` inside the pod. The workflow engine's `ctx.fetch(service, path)` sends requests to `http://localhost:9090/{service}{path}`. The Gateway extracts the service name from the URL path prefix, resolves the upstream URL, injects credentials, and forwards the request.

**Rationale:** Using the URL path for service routing is simple and requires no custom headers or protocol. The workflow container does not need to know upstream URLs -- it only knows service names. The Gateway maintains a service registry (loaded from a ConfigMap) that maps service names to upstream base URLs and credential references. Alternative considered: custom `X-Service` header for routing -- rejected because path-based routing is more transparent in logs and debugging. Alternative considered: DNS-based routing (e.g., `service.gateway.local`) -- rejected because it requires DNS configuration in the pod and is harder to debug.

### Decision 4: Structured JSON audit logging

**Choice:** Every proxied request produces a structured JSON log entry written to the Gateway container's stdout. The log entry includes: timestamp, workflow ID, node ID, service name, HTTP method, path, response status code, latency in milliseconds, request size, response size, and a correlation ID.

**Rationale:** Structured JSON logs are compatible with all major log aggregation systems (Fluentd, Loki, CloudWatch, Datadog). Writing to stdout follows the twelve-factor app pattern and integrates with Kubernetes log collection. The correlation ID enables tracing a request across the workflow engine and Gateway logs. Alternative considered: OpenTelemetry spans -- deferred to a future enhancement; structured JSON is sufficient for v1 and does not require additional dependencies.

### Decision 5: Token bucket rate limiting

**Choice:** The Gateway implements per-service, per-workflow token bucket rate limiting. Limits are configured in `workflow.yaml` under a new `gateway.rate_limits` section. Each service gets an independent bucket with configurable `requests_per_second` and `burst` parameters.

**Rationale:** Token bucket is a well-understood rate limiting algorithm that handles bursty traffic gracefully. Per-service limits prevent one noisy service integration from affecting others. Per-workflow limits prevent a single workflow from exhausting shared API quotas. The burst parameter allows short spikes while enforcing an average rate. Alternative considered: sliding window counter -- rejected because it does not handle bursts as gracefully. Alternative considered: global (cross-workflow) rate limiting -- deferred because it requires coordination between Gateway sidecars, which contradicts the sidecar isolation model.

### Decision 6: MCP proxy via HTTP bridge

**Choice:** MCP tool calls are bridged to HTTP within the Gateway. The workflow engine sends MCP requests as HTTP POST to `http://localhost:9090/_mcp/{tool_name}` with the MCP request body as JSON. The Gateway translates this to the appropriate MCP transport (stdio, SSE, or HTTP), injects credentials, logs the call, and returns the MCP response as HTTP.

**Rationale:** Bridging MCP to HTTP allows the workflow container to use the same `ctx.fetch`-style interface for MCP calls, keeping the node API simple. The Gateway handles the transport complexity (stdio for local tools, SSE for remote MCP servers). This avoids requiring MCP client libraries in the workflow container. Alternative considered: native MCP client in the workflow container -- rejected because it would require mounting MCP credentials into the workflow container, defeating the Tier 1 secrets model.

### Decision 7: Envoy as Gateway implementation

**Choice:** Use Envoy proxy as the Gateway sidecar implementation, configured via static YAML or xDS.

**Rationale:** Envoy is the industry standard for sidecar proxies (Istio, AWS App Mesh). It provides HTTP routing, header injection (via Lua or ext_authz), rate limiting (local rate limit filter), structured access logging (JSON format), and health checking out of the box. Using Envoy avoids building a custom proxy and provides a well-tested, high-performance foundation. The MCP bridge is implemented as a small Lua filter or external processor. Alternative considered: custom Go proxy -- rejected because it would require reimplementing routing, rate limiting, logging, and connection pooling. Alternative considered: Nginx -- rejected because Envoy has better dynamic configuration and richer filter chains.

## Architecture

### Pod Layout

```
+----------------------------------------------------------+
|  Kubernetes Pod                                          |
|                                                          |
|  +-------------------------+  +-----------------------+  |
|  |  Workflow Container     |  |  Gateway Sidecar      |  |
|  |  (Deno engine)          |  |  (Envoy proxy)        |  |
|  |                         |  |                       |  |
|  |  ctx.fetch("github",   |  |  localhost:9090        |  |
|  |    "/repos") ---------> |  |    |                   |  |
|  |                         |  |    +-> Route by svc    |  |
|  |  No Tier 1 secrets      |  |    +-> Inject creds   |  |
|  |  Has Tier 2 config      |  |    +-> Rate limit      |  |
|  |                         |  |    +-> Audit log       |  |
|  |                         |  |    +-> Forward         |  |
|  +-------------------------+  |         |              |  |
|                               |         v              |  |
|  +-------------------------+  |  api.github.com        |  |
|  |  Volumes                |  |                       |  |
|  |                         |  |  Tier 1 secrets       |  |
|  |  tier2-config (shared)  |  |  (Gateway-only mount) |  |
|  +-------------------------+  +-----------------------+  |
+----------------------------------------------------------+
```

### Request Flow

```
Node code                    Workflow Engine           Gateway Sidecar           External API
    |                             |                         |                        |
    | ctx.fetch("stripe",        |                         |                        |
    |   "/v1/charges",           |                         |                        |
    |   { method: "POST",        |                         |                        |
    |     body: payload })       |                         |                        |
    |                             |                         |                        |
    +---------------------------->|                         |                        |
    |                             | POST localhost:9090     |                        |
    |                             |   /stripe/v1/charges    |                        |
    |                             | X-PD-Workflow: my-wf    |                        |
    |                             | X-PD-Node: charge-node  |                        |
    |                             | X-PD-Correlation: uuid  |                        |
    |                             +------------------------>|                        |
    |                             |                         | 1. Extract service     |
    |                             |                         |    name: "stripe"      |
    |                             |                         | 2. Check rate limit    |
    |                             |                         | 3. Resolve upstream:   |
    |                             |                         |    api.stripe.com      |
    |                             |                         | 4. Inject Tier 1       |
    |                             |                         |    secret: Bearer sk_  |
    |                             |                         | 5. Forward request     |
    |                             |                         +----------------------->|
    |                             |                         |                        |
    |                             |                         |<-----------------------+
    |                             |                         | 6. Log audit entry     |
    |                             |<------------------------+ 7. Return response     |
    |<----------------------------+                         |                        |
    |                             |                         |                        |
```

### ctx.fetch Refactoring

The current `createFetch()` in `engine/context/mod.ts` builds URLs as `https://api.${service}.com${path}` and injects auth headers from `ctx.secrets`. With the Gateway, this changes to:

```typescript
// CURRENT (file-mount secrets):
const url = `https://api.${service}.com${path}`;
headers.set("Authorization", `Bearer ${secrets[service].token}`);
return fetch(url, { ...init, headers });

// FUTURE (Gateway proxy):
const gatewayUrl = `http://localhost:${GATEWAY_PORT}/${service}${path}`;
headers.set("X-PD-Workflow", workflowId);
headers.set("X-PD-Node", nodeId);
headers.set("X-PD-Correlation", crypto.randomUUID());
return fetch(gatewayUrl, { ...init, headers });
```

The node code (`ctx.fetch("stripe", "/v1/charges")`) does not change. The refactoring is entirely within the engine context layer.

### Service Registry

The Gateway loads a service registry from a ConfigMap mounted as a YAML file:

```yaml
# gateway-services.yaml
services:
  github:
    upstream: https://api.github.com
    secret_ref: github-api-token        # K8s Secret name
    auth_type: bearer_token
    auth_field: token                    # key within the K8s Secret

  stripe:
    upstream: https://api.stripe.com
    secret_ref: stripe-api-key
    auth_type: bearer_token
    auth_field: sk_live

  openai:
    upstream: https://api.openai.com
    secret_ref: openai-credentials
    auth_type: bearer_token
    auth_field: api_key

  slack:
    upstream: https://slack.com/api
    secret_ref: slack-bot-token
    auth_type: bearer_token
    auth_field: token
```

Supported `auth_type` values:
- `bearer_token` -- sets `Authorization: Bearer {value}`
- `api_key_header` -- sets `X-API-Key: {value}`
- `basic_auth` -- sets `Authorization: Basic {base64(user:pass)}`
- `custom_header` -- sets a configurable header name with the secret value

### Two-Tier Secrets in Detail

```
Tier 1 (Gateway-only):                    Tier 2 (Workflow-accessible):
+----------------------------+            +----------------------------+
| K8s Secrets                |            | K8s ConfigMap / Secrets    |
|                            |            |                            |
| github-api-token           |            | workflow-config            |
|   token: ghp_xxxx          |            |   batch_size: "100"        |
|                            |            |   feature_flag_x: "true"   |
| stripe-api-key             |            |   retry_delay_ms: "500"    |
|   sk_live: sk_live_xxxx    |            |                            |
|                            |            +----------------------------+
| openai-credentials         |                     |
|   api_key: sk-xxxx         |                     | Volume mount into
|                            |                     | workflow container
+----------------------------+                     |
         |                                          v
         | Volume mount into              +-----------------+
         | Gateway container ONLY         | Workflow        |
         |                                | Container       |
         v                                | ctx.config.     |
+-----------------+                       |   batch_size    |
| Gateway         |                       +-----------------+
| Container       |
| Injects into    |
| outbound reqs   |
+-----------------+
```

### Audit Log Format

Every request proxied through the Gateway produces a structured JSON log entry:

```json
{
  "timestamp": "2026-02-09T14:30:00.123Z",
  "level": "info",
  "type": "gateway_request",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "workflow_id": "data-pipeline",
  "workflow_version": "1.0",
  "node_id": "fetch-orders",
  "service": "stripe",
  "method": "GET",
  "path": "/v1/charges",
  "upstream_url": "https://api.stripe.com/v1/charges",
  "status_code": 200,
  "request_size_bytes": 0,
  "response_size_bytes": 4521,
  "latency_ms": 145,
  "rate_limited": false,
  "rate_limit_remaining": 87,
  "error": null
}
```

For rate-limited requests (HTTP 429 returned to the workflow):

```json
{
  "timestamp": "2026-02-09T14:30:01.456Z",
  "level": "warn",
  "type": "gateway_request",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440001",
  "workflow_id": "data-pipeline",
  "workflow_version": "1.0",
  "node_id": "fetch-orders",
  "service": "stripe",
  "method": "GET",
  "path": "/v1/charges",
  "upstream_url": null,
  "status_code": 429,
  "request_size_bytes": 0,
  "response_size_bytes": 0,
  "latency_ms": 0,
  "rate_limited": true,
  "rate_limit_remaining": 0,
  "error": "rate limit exceeded: stripe (10 req/s, burst 20)"
}
```

For MCP tool calls:

```json
{
  "timestamp": "2026-02-09T14:30:02.789Z",
  "level": "info",
  "type": "gateway_mcp",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440002",
  "workflow_id": "ai-agent-pipeline",
  "workflow_version": "1.0",
  "node_id": "research-agent",
  "mcp_tool": "web-search",
  "mcp_server": "brave-search",
  "mcp_method": "tools/call",
  "status": "success",
  "latency_ms": 823,
  "error": null
}
```

### NetworkPolicy Template

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: pd-workflow-egress
  namespace: pipedreamer
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: workflow
  policyTypes:
    - Egress
  egress:
    # Allow DNS resolution
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53

    # Allow traffic to Gateway sidecar (localhost)
    # Note: NetworkPolicy does not filter loopback traffic by default
    # in most CNI plugins. This rule is for documentation/intent.
    # The real enforcement is: deny all other egress.

    # Allow traffic to Kubernetes API (for health checks, service account)
    - to:
        - ipBlock:
            cidr: 10.0.0.0/8   # Cluster CIDR — adjust per cluster
      ports:
        - protocol: TCP
          port: 443

    # Deny all other egress by omission
    # The workflow container can only reach:
    # 1. DNS for name resolution
    # 2. localhost:9090 (Gateway sidecar) — loopback, not filtered by netpol
    # 3. K8s API server for pod health
    # All external API calls MUST go through the Gateway
```

**Important caveat:** Most Kubernetes CNI plugins (Calico, Cilium, etc.) do not filter loopback traffic via NetworkPolicy. Traffic between containers in the same pod over `localhost` is always allowed. The NetworkPolicy's purpose is to block the workflow container from making **direct** outbound calls to external APIs, forcing all traffic through the Gateway sidecar on localhost.

### Rate Limiting Design

Rate limits are configured in `workflow.yaml`:

```yaml
name: data-pipeline
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch-orders:
    path: nodes/fetch-orders.ts
edges: []
gateway:
  rate_limits:
    stripe:
      requests_per_second: 10
      burst: 20
    github:
      requests_per_second: 30
      burst: 50
    openai:
      requests_per_second: 5
      burst: 10
  default_rate_limit:
    requests_per_second: 20
    burst: 40
```

The Gateway implements token bucket rate limiting:
- Each service gets an independent bucket
- Bucket refills at `requests_per_second` rate
- Maximum tokens in bucket equals `burst`
- When a request arrives and the bucket is empty, the Gateway returns HTTP 429 to the workflow container with a `Retry-After` header
- The workflow engine's retry logic (in `SimpleExecutor`) handles 429s with exponential backoff

The Envoy local rate limit filter handles this natively:

```yaml
# Envoy rate limit filter configuration (per-route)
http_filters:
  - name: envoy.filters.http.local_ratelimit
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.local_ratelimit.v3.LocalRateLimit
      stat_prefix: gateway_rate_limit
      token_bucket:
        max_tokens: 20
        tokens_per_fill: 10
        fill_interval: 1s
      filter_enabled:
        runtime_key: local_rate_limit_enabled
        default_value:
          numerator: 100
          denominator: HUNDRED
```

### MCP Proxy Design

MCP (Model Context Protocol) tool calls follow the same Gateway pattern as HTTP calls:

```
AI Agent Node                Workflow Engine           Gateway Sidecar           MCP Server
    |                             |                         |                        |
    | ctx.mcp("web-search",      |                         |                        |
    |   { query: "..." })        |                         |                        |
    |                             |                         |                        |
    +---------------------------->|                         |                        |
    |                             | POST localhost:9090     |                        |
    |                             |   /_mcp/web-search      |                        |
    |                             | Content-Type:           |                        |
    |                             |   application/json      |                        |
    |                             | Body: {                 |                        |
    |                             |   "method":             |                        |
    |                             |     "tools/call",       |                        |
    |                             |   "params": {           |                        |
    |                             |     "name":             |                        |
    |                             |       "web-search",     |                        |
    |                             |     "arguments": {      |                        |
    |                             |       "query": "..."    |                        |
    |                             |     }                   |                        |
    |                             |   }                     |                        |
    |                             | }                       |                        |
    |                             +------------------------>|                        |
    |                             |                         | 1. Extract MCP tool    |
    |                             |                         | 2. Resolve MCP server  |
    |                             |                         |    from registry       |
    |                             |                         | 3. Inject credentials  |
    |                             |                         | 4. Forward via         |
    |                             |                         |    appropriate         |
    |                             |                         |    transport           |
    |                             |                         +----------------------->|
    |                             |                         |<-----------------------+
    |                             |                         | 5. Log audit entry     |
    |                             |<------------------------+ 6. Return MCP response |
    |<----------------------------+                         |                        |
```

MCP servers are registered in the service registry with transport type:

```yaml
# gateway-services.yaml (MCP section)
mcp_servers:
  brave-search:
    transport: http
    url: https://mcp.brave.com/v1
    secret_ref: brave-api-key
    auth_type: bearer_token
    auth_field: api_key
    tools:
      - web-search
      - local-search

  filesystem:
    transport: stdio
    command: npx
    args: ["-y", "@anthropic/mcp-filesystem"]
    # No secret needed for local filesystem

  github-mcp:
    transport: sse
    url: https://mcp.github.com/sse
    secret_ref: github-api-token
    auth_type: bearer_token
    auth_field: token
    tools:
      - search-repos
      - get-file
      - create-issue
```

The Context interface gains a new `mcp` method:

```typescript
export interface Context {
  fetch(service: string, path: string, init?: RequestInit): Promise<Response>;
  mcp(tool: string, params: Record<string, unknown>): Promise<McpResult>;
  log: Logger;
  config: Record<string, unknown>;
  secrets: Record<string, Record<string, string>>; // Tier 2 only
}

export interface McpResult {
  content: McpContent[];
  isError?: boolean;
}

export interface McpContent {
  type: "text" | "image" | "resource";
  text?: string;
  data?: string;
  mimeType?: string;
}
```

Under the hood, `ctx.mcp(tool, params)` sends an HTTP POST to `http://localhost:9090/_mcp/{tool}` with the MCP JSON-RPC request body.

### Migration Path

The migration from file-mount secrets to Gateway sidecar is designed to be backward-compatible and incremental:

**Phase 1: Gateway sidecar without NetworkPolicy (backward compatible)**
- Deploy the Gateway sidecar alongside existing workflow containers
- Refactor `ctx.fetch` to route through `localhost:9090` instead of directly to external APIs
- Keep Tier 2 secrets (file-mount) working for `ctx.secrets` access
- Gateway injects Tier 1 credentials for all `ctx.fetch` calls
- No NetworkPolicy yet -- workflow containers can still make direct calls (fallback)
- `ctx.secrets` still available but deprecated for service credentials
- Feature flag `GATEWAY_ENABLED=true` in workflow container enables Gateway routing

**Phase 2: Enforce NetworkPolicy**
- Apply the `pd-workflow-egress` NetworkPolicy
- Workflow containers can no longer make direct external calls
- All traffic must go through Gateway sidecar
- `ctx.secrets` no longer contains Tier 1 service credentials
- Remove deprecated `ctx.secrets` service credential fields
- Add health check endpoint on Gateway (`localhost:9090/_health`)

**Phase 3: MCP proxy activation**
- Deploy MCP server registry to Gateway
- Add `ctx.mcp()` method to Context interface
- AI agent nodes use `ctx.mcp(tool, params)` for MCP tool calls
- MCP calls get the same audit logging and rate limiting as HTTP calls

**Phase 4: Rate limiting activation**
- Enable rate limiting in Gateway configuration
- Add `gateway.rate_limits` section to `workflow.yaml` schema
- Validate rate limit config in `pipedreamer validate`

### Local Development Mode

For local development (`pipedreamer dev`), the Gateway sidecar is not required. The `ctx.fetch` implementation detects whether the Gateway is available:

```typescript
const GATEWAY_PORT = Deno.env.get("PD_GATEWAY_PORT") || "9090";
const GATEWAY_ENABLED = Deno.env.get("PD_GATEWAY_ENABLED") === "true";

function createFetch(opts: FetchOptions) {
  if (GATEWAY_ENABLED) {
    return createGatewayFetch(opts);  // Route through Gateway
  }
  return createDirectFetch(opts);     // Current behavior (file-mount secrets)
}
```

This means:
- `pipedreamer dev` (local) -- uses file-mount secrets, direct HTTP calls (current behavior)
- `pipedreamer deploy` (K8s) -- Gateway sidecar is injected, `PD_GATEWAY_ENABLED=true` is set

## File Layout

This is a design-only change. The artifacts are:

- `openspec/changes/gateway-roadmap-design/proposal.md` -- Motivation and scope
- `openspec/changes/gateway-roadmap-design/design.md` -- This document (comprehensive architecture)
- `openspec/changes/gateway-roadmap-design/specs/gateway-sidecar-design/spec.md` -- Capability requirements
- `openspec/changes/gateway-roadmap-design/tasks.md` -- Design authoring and review tasks

Future implementation changes will produce code in:
- `engine/context/gateway.ts` -- Gateway-aware fetch implementation
- `engine/context/mcp.ts` -- MCP proxy client
- `deploy/gateway/` -- Envoy configuration, Dockerfile, Kubernetes manifests
- `pkg/spec/types.go` -- Gateway config types in workflow spec
- `pkg/k8s/` -- NetworkPolicy and sidecar injection templates

## Risks / Trade-offs

- **Envoy complexity** -- Envoy is powerful but has a steep configuration learning curve. Mitigated by using a minimal static configuration with only the required filters (routing, header injection, rate limiting, access logging). No dynamic xDS needed for the sidecar pattern.
- **Loopback performance** -- All outbound requests add a localhost hop through Envoy. This adds approximately 0.1-0.5ms latency per request, which is negligible compared to external API latency (typically 50-500ms).
- **MCP stdio transport in sidecar** -- Running stdio-based MCP servers inside the Gateway container requires process management. This adds container image size and complexity. Mitigated by supporting HTTP/SSE transports first; stdio transport is a Phase 3 enhancement.
- **Migration risk** -- Switching from direct HTTP to Gateway-proxied HTTP changes the network path. Bugs in service registry configuration could cause outages. Mitigated by Phase 1's fallback mode (no NetworkPolicy enforcement) and feature flag rollout.
- **Rate limiting accuracy** -- Token bucket in a sidecar only limits per-workflow-instance. If multiple pods run the same workflow, total API usage is the sum of per-pod limits. For global rate limiting, a centralized rate limit service (e.g., Redis-backed) would be needed. Deferred to a future enhancement.
- **Secret rotation** -- When Tier 1 secrets rotate in K8s, the Gateway must pick up new values. Envoy supports file-based secret discovery (SDS) with inotify-based hot reloading, so secret rotation does not require pod restarts.
