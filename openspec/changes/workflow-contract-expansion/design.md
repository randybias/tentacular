## Context

Tentacular's current model separates DAG structure (`workflow.yaml`) from operational dependency details hidden in node code (`ctx.fetch`, `ctx.secrets`, `ctx.config`). This creates planning and security blind spots:

- Agents cannot reliably negotiate dependencies and secrets with users without code inspection.
- Visual topology lacks dependency/secret context for review.
- Network policy generation cannot be driven from a declarative source of truth.
- Drift between code behavior and declared intent is possible.
- Dependency connection info (host, port, database) is scattered across config, secrets, and node code.

### Design philosophy

**Workflows are disposable.** Small, self-contained, tightly spec'd units. Validate the new version, discard the old. No migration, no rolling updates, no backward compatibility.

**Dependencies are the single primitive.** A Tentacular workflow is a sealed pod (no local filesystem writes, no configmap access, hardened by design) that can only reach declared network dependencies. Everything the workflow needs from the outside world is a dependency. Therefore: secrets, network policy, connection config, and validation are all derivable from the dependency list.

## Goals / Non-Goals

**Goals:**

- Make `workflow.yaml` the authoritative contract via a typed dependency list.
- Derive secrets inventory, NetworkPolicy, and connection config from dependency declarations.
- Provide `ctx.dependency()` API so nodes read connection metadata from contract, not scattered config/secrets.
- Support runtime-tracing drift detection during mock tests.
- Provide agent-usable rich visualization artifacts from contract declarations.
- Strict enforcement by default; environment-level override to `audit` for dev.

**Non-Goals:**

- Storing secret values in `workflow.yaml`.
- Runtime mutation of workflow contracts after deployment.
- Full dynamic traffic introspection/eBPF policy generation.
- Vault implementation details (future integration out of scope).
- Migration tooling or backward compatibility.

## Decisions

### D1: Contract is a typed dependency list

Add `contract:` to `workflow.yaml` as a top-level peer of `nodes`, `edges`, `config`:

```yaml
contract:
  version: "1"
  dependencies:
    github-api:
      protocol: https
      host: api.github.com
      port: 443
      auth:
        type: bearer-token
        secret: github.token
    postgres:
      protocol: postgresql
      host: postgres-postgresql.postgres.svc.cluster.local
      port: 5432
      database: appdb
      user: postgres
      auth:
        type: password
        secret: postgres.password
  # Optional: override derived policy for edge cases
  # networkPolicy:
  #   additionalEgress:
  #     - cidr: 10.0.0.0/8
  #       port: 8080
  #       protocol: TCP
  #       reason: "internal service mesh"
```

What gets derived:

| Artifact | Source | Derivation |
|---|---|---|
| Required secrets | `dep.auth.secret` | Collect all auth secret refs |
| Egress NetworkPolicy | `dep.host` + `dep.port` | One allow rule per dep + DNS |
| Ingress NetworkPolicy | `triggers[].type` | Webhook → allow ingress; else deny all |
| Connection config | `dep.*` metadata | Injected via `ctx.dependency()` |

The `config` section remains for business-logic-only parameters (`target_repo`, `sep_label`).

Rationale: single source of truth eliminates redundancy. No separate secrets section, no separate networkPolicy section (unless overriding). Authoring burden is minimal — declare what you connect to, everything else follows.

### D2: `ctx.dependency()` node API

The engine injects contract dependency metadata into the node context. Nodes access it via:

```typescript
const pg = ctx.dependency("postgres");
// pg.host, pg.port, pg.database, pg.user available
// pg.secret resolved eagerly at call time from mounted secrets
// pg.authType is "password" (from contract auth.type)
```

For HTTPS dependencies, the returned `DependencyConnection` includes a convenience `fetch(path, init?)` method that auto-injects auth headers based on `authType`:

```typescript
const gh = ctx.dependency("github-api");
// gh.authType is "bearer-token" -> fetch auto-sets Authorization: Bearer <secret>
const resp = await gh.fetch("/repos/org/repo/issues");

const blob = ctx.dependency("azure-blob");
// blob.authType is "sas-token" -> fetch auto-appends SAS token to URL query
const resp2 = await blob.fetch("/container/report.html", { method: "PUT", body: html });
```

`DependencyConnection` fields:
- `protocol`: string — the dependency protocol (https, postgresql, nats, blob)
- `host`: string — the dependency host
- `port`: number — the dependency port
- `secret`: string — the resolved secret value (eagerly resolved at call time)
- `authType`: string — the auth type from contract (bearer-token, api-key, sas-token, password, webhook-url)
- Protocol-specific fields: `database`, `user` (postgresql); `subject` (nats); `container` (blob)
- `fetch(path, init?)`: async method (HTTPS deps only) — makes HTTP request with auto-injected auth

This replaces the current pattern where nodes manually assemble connection strings from `ctx.config` + `ctx.secrets`. Nodes become simpler and contract-aware by default.

The mock context returns dependency metadata with mock values during `tntc test`, consistent with the existing mock pattern. Mock `fetch()` returns mock responses and records calls for drift detection.

### D3: Runtime-tracing drift detection during mock test

`tntc test` already runs every node with a mock context. Drift detection piggybacks:

- Mock context records all `ctx.dependency()` accesses, `ctx.secrets` accesses, and `ctx.fetch` target hosts.
- After test, recorded usage is compared against contract declarations.
- Missing declarations (code uses something not in contract) and dead declarations (contract declares something code never uses) are both errors in strict mode.
- `ctx.secrets` and `ctx.fetch` direct usage (bypassing `ctx.dependency()`) is flagged as a contract violation — nodes should use the dependency API.

Rationale: no fragile TypeScript static analysis. Runtime tracing is accurate and zero-maintenance.

### D4: NetworkPolicy derived from dependencies + triggers

`deploy` auto-generates NetworkPolicy:

- **Default-deny egress**: implicit for hardened pod. All egress denied except declared dependencies.
- **Egress allow rules**: one per dependency (host/port/protocol from contract).
- **DNS egress**: mandatory UDP/TCP 53 to kube-dns whenever default-deny is active.
- **Ingress**: derived from trigger type. Webhook trigger → allow ingress on trigger port. Cron/manual/queue → deny all ingress.
- **Optional overrides**: `contract.networkPolicy.additionalEgress` for edge cases (e.g., internal service mesh CIDR blocks).

Rationale: policy is fully derivable. No hand-authoring required for the common case.

#### D4a: Tiered egress rules by host type

K8s NetworkPolicy operates at L3/L4 (IP + port), but contracts declare L7 identities (hostnames). Egress rules are tiered by host type to maximize isolation within NetworkPolicy's capabilities:

**1. Cluster-internal dependencies** (host ends with `.svc.cluster.local` or `.svc`):

Use `namespaceSelector` + port restriction to scope egress to the specific service namespace. The namespace is parsed from the host FQDN (e.g., `postgres-postgresql.postgres.svc.cluster.local` → namespace `postgres`).

```yaml
# Cluster-internal egress rule (e.g., postgres)
- to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: postgres
  ports:
    - protocol: TCP
      port: 5432
```

**2. External dependencies** (everything else, e.g., `api.github.com`):

Use `ipBlock` with `0.0.0.0/0` CIDR and port restriction. The `except` clause blocks RFC 1918 private ranges to prevent a compromised workflow from reaching cluster-internal services by IP (defense in depth).

```yaml
# External egress rule (e.g., api.github.com:443)
- to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
          - 10.0.0.0/8
          - 172.16.0.0/12
          - 192.168.0.0/16
  ports:
    - protocol: TCP
      port: 443
```

The intended hostname is recorded as an annotation on the NetworkPolicy resource (`tentacular.dev/intended-host: api.github.com`) for audit trail and human review.

**3. DNS egress** (always present):

Scoped to kube-system namespace to prevent DNS exfiltration to arbitrary resolvers.

```yaml
# DNS egress rule (always present)
- to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
  ports:
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 53
```

**Security posture summary:**
- Cluster-internal deps: pod-level isolation (tight)
- External deps: port-level isolation with RFC 1918 exclusion (pragmatic)
- DNS: namespace-scoped (prevents arbitrary resolver access)

**Future enhancement:** For true hostname-based egress filtering of external services, CNIs with FQDN policy support (Cilium `CiliumNetworkPolicy` with `toFQDNs`, Calico Enterprise `NetworkPolicy` with `domains`) can replace the `0.0.0.0/0` rules. This is out of scope for v1 but the contract already captures the hostname, so the upgrade path is mechanical.

### D5: Rich visualization as contract-aware artifact

`tntc visualize --rich` produces:

- DAG graph with dependency nodes (protocol/host labels)
- Required secret key inventory (derived from dependency auth refs)
- Network intent summary (derived egress/ingress rules)
- Co-resident artifacts (Mermaid + contract summary) for PR review

Rationale: agents and humans review the same derived artifacts before build/deploy.

### D6: Extensibility model

Core contract fields are versioned (`contract.version`) and strictly validated. Extension fields allowed via `x-*` namespaced keys, preserved through parsing.

### D7: SKILL pre-build contract review gate

SKILL requires before any `build`, `test --live`, or `deploy`:

- Generate rich visualization from contract
- Review diagram + derived artifacts with user
- Confirm dependency targets, secret refs, network intent
- Run validation and resolve mismatches

### D8: Strict enforcement by default

Contract enforcement is `strict` by default. Environment config can override to `audit` (warnings instead of errors). No per-workflow setting — all workflows are held to the same standard.

## Risks / Trade-offs

- **[Risk] Schema complexity for simple workflows** → Mitigation: word-counter has no dependencies, so `contract: {version: "1", dependencies: {}}` is the minimal form.
- **[Risk] Overly restrictive generated network policy** → Mitigation: `additionalEgress` overrides, dry-run report before apply, audit mode in dev.
- **[Risk] Ambiguity across dependency types** → Mitigation: typed protocol field with protocol-specific validation.
- **[Risk] DNS egress gaps** → Mitigation: mandatory DNS rule auto-generation.
- **[Risk] `ctx.dependency()` API changes node authoring pattern** → Mitigation: straightforward mechanical update, mock context provides same ergonomics.
- **[Risk] Dead declaration false positives from conditional code paths** → A dependency may be declared but only accessed in certain branches (e.g., error recovery, feature flags). Mock tests may not exercise all paths, causing false "dead declaration" reports. Mitigation: tests should exercise all dependency paths; audit mode available for dev environments where this is acceptable.

## Open Questions

- ~~What is the minimal supported dependency protocol set in v1?~~ **Resolved:** v1 supports `https`, `postgresql`, `nats`, `blob` with protocol-specific required fields and default ports.
- ~~How should wildcard domains be represented for policy generation safety?~~ **Resolved:** Wildcard domains are rejected in v1. Each dependency must declare an explicit host. This matches the "sealed pod" security model and avoids ambiguity in NetworkPolicy generation.
- ~~Should `ctx.dependency()` resolve the secret value at call time, or return a reference the node resolves explicitly?~~ **Resolved:** `ctx.dependency()` resolves the secret value at call time. The resolved value is available as `dep.secret`. This keeps node code simple and consistent with mock context returning mock values.
