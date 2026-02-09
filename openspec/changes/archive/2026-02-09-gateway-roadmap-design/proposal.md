## Why

Pipedreamer v2 currently handles secrets via file-mount (`.secrets.yaml` for local dev, K8s Secret volume mount for production). The `ctx.fetch(service, path)` call in `engine/context/mod.ts` directly injects auth headers from these file-mounted secrets and makes outbound HTTP calls from the workflow container itself. This approach has fundamental limitations:

1. **Secrets in container memory** -- Node code has direct access to raw secret values via `ctx.secrets`. A compromised or poorly-written node can exfiltrate credentials.
2. **No centralized audit trail** -- Outbound API calls happen directly from the workflow container. There is no single point to log which workflow accessed which external service, when, and with what result.
3. **No rate limiting** -- Workflow nodes can make unlimited outbound calls. A bug in a loop can hammer an external API and exhaust rate limits or incur costs.
4. **No MCP proxy** -- The roadmap includes Model Context Protocol (MCP) tool access for AI agent workflows. MCP calls need the same auth injection, audit logging, and rate limiting as HTTP calls but use a different protocol.
5. **NetworkPolicy gap** -- Without a gateway, there is no clean way to enforce that workflow containers can ONLY reach external services through a controlled proxy.

A Gateway sidecar architecture addresses all five problems by interposing a reverse proxy between workflow containers and external services. This change produces a comprehensive design document that defines the architecture, protocols, data formats, and migration path. No code is implemented -- the design document IS the deliverable.

## What Changes

- **Gateway sidecar architecture design** -- A comprehensive design document covering the Gateway container that runs as a sidecar alongside each workflow pod, acting as an outbound reverse proxy for all external API calls and MCP tool invocations.
- **Two-tier secrets model** -- Design for separating secrets into two tiers: (1) service credentials injected by the Gateway proxy into outbound requests (never exposed to workflow code), and (2) workflow-level config accessible via direct Vault/K8s Secret access for non-sensitive configuration.
- **Secret injection protocol** -- Specification of how the Gateway intercepts `ctx.fetch(service, path)` calls, maps service names to upstream URLs, and injects the appropriate credentials from its own secret store.
- **Audit logging format** -- Structured JSON log format for every outbound request routed through the Gateway, capturing workflow ID, node ID, service, path, method, status code, latency, and timestamp.
- **NetworkPolicy template** -- Kubernetes NetworkPolicy that restricts workflow container egress to only the Gateway sidecar, ensuring all external traffic is proxied.
- **MCP proxy design** -- How MCP tool calls from AI agent nodes are routed through the Gateway with the same auth injection and audit logging as HTTP calls.
- **Rate limiting design** -- Token bucket rate limiting per service per workflow, configurable in `workflow.yaml`.
- **Migration path** -- Step-by-step migration from the current file-mount secrets approach to the Gateway sidecar model, maintaining backward compatibility.

## Capabilities

### New Capabilities
- `gateway-sidecar-design`: Architecture design document for the Gateway sidecar covering reverse proxy, secret injection, audit logging, NetworkPolicy, MCP proxy, rate limiting, and migration path

### Modified Capabilities
_(none -- this is a design-only change, no code modifications)_

## Impact

- **New files**: Design document artifacts only (proposal, design, specs, tasks in `openspec/changes/gateway-roadmap-design/`)
- **Modified files**: None -- no code changes
- **Dependencies**: None -- design only
- **Downstream changes**: Future implementation changes will reference this design for Gateway sidecar implementation, `ctx.fetch` refactoring, MCP proxy integration, and NetworkPolicy deployment
