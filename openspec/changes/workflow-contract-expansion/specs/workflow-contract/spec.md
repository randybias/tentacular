## ADDED Requirements

### Requirement: Workflow contract as typed dependency list
A workflow SHALL support a top-level `contract` section in `workflow.yaml` (peer of `nodes`, `edges`, `config`) containing a versioned, typed dependency list.

#### Scenario: Contract section present
- **WHEN** a workflow includes `contract` in `workflow.yaml`
- **THEN** the parser SHALL load dependencies into typed structures with protocol-specific metadata

#### Scenario: Minimal contract for dependency-free workflow
- **WHEN** a workflow has no external dependencies
- **THEN** the contract SHALL be valid with `version` and an empty `dependencies` map

### Requirement: Dependency declarations with connection metadata and auth
Each dependency SHALL declare protocol, host, port, protocol-specific connection metadata, and auth with a secret key reference.

#### Scenario: HTTPS dependency declared
- **WHEN** a workflow depends on GitHub API
- **THEN** the dependency SHALL declare `protocol: https`, `host: api.github.com`, `port: 443`, and `auth` with secret key reference

#### Scenario: PostgreSQL dependency declared
- **WHEN** a workflow depends on Postgres
- **THEN** the dependency SHALL declare `protocol: postgresql`, `host`, `port`, `database`, `user`, and `auth` with secret key reference

#### Scenario: Secret value rejection
- **WHEN** a dependency auth declaration includes a raw secret value instead of a key reference
- **THEN** validation SHALL fail with an error indicating secret values are not allowed in `workflow.yaml`

### Requirement: Secrets derived from dependency auth
The set of required secrets SHALL be derived from `dep.auth.secret` across all dependencies. No separate secrets section.

#### Scenario: Secret inventory derivation
- **WHEN** a workflow declares dependencies with `auth.secret` values `github.token` and `postgres.password`
- **THEN** the derived secret inventory SHALL contain exactly `["github.token", "postgres.password"]`

### Requirement: NetworkPolicy derived from dependencies and triggers
NetworkPolicy SHALL be derived from dependency hosts/ports (egress) and trigger types (ingress). Default-deny is implicit.

#### Scenario: Egress derived from dependencies
- **WHEN** a workflow declares dependencies on `api.github.com:443` and `postgres.svc:5432`
- **THEN** derived egress policy SHALL allow TCP 443 to `api.github.com` and TCP 5432 to `postgres.svc`, deny all other egress

#### Scenario: DNS egress auto-included
- **WHEN** egress policy is derived
- **THEN** derived policy SHALL include UDP/TCP 53 to kube-dns regardless of dependency declarations

#### Scenario: Ingress derived from webhook trigger
- **WHEN** a workflow has a `webhook` trigger
- **THEN** derived ingress policy SHALL allow traffic on the webhook trigger port

#### Scenario: Label-scoped ingress for non-webhook triggers
- **WHEN** a workflow has only `cron`, `manual`, or `queue` triggers
- **THEN** derived ingress policy SHALL allow only pods with label `tentacular.dev/role: trigger` on port 8080

### Requirement: Optional network policy overrides
The contract SHALL support optional `networkPolicy.additionalEgress` for edge cases not derivable from dependencies.

#### Scenario: Additional egress override
- **WHEN** a contract includes `networkPolicy.additionalEgress` entries
- **THEN** those entries SHALL be merged into the derived egress policy alongside dependency-derived rules

### Requirement: Supported protocol set
Version 1 of the contract schema SHALL support a defined set of protocols with protocol-specific validation.

#### Scenario: Supported protocols in v1
- **GIVEN** contract version "1"
- **THEN** the supported protocols SHALL be: `https`, `postgresql`, `nats`, `blob`
- **AND** each protocol SHALL have protocol-specific required fields:
  - `https`: host, port (default 443)
  - `postgresql`: host, port (default 5432), database, user
  - `nats`: host, port (default 4222), subject
  - `blob`: host, container

#### Scenario: Protocol default ports
- **WHEN** a dependency omits `port`
- **THEN** parser SHALL apply the protocol's default port value

### Requirement: Dynamic-target dependency type
Dependencies with runtime-determined targets SHALL declare `type: "dynamic-target"` with explicit CIDR and port constraints.

#### Scenario: Dynamic-target dependency declared
- **WHEN** a dependency declares `type: "dynamic-target"`
- **THEN** `cidr` and `dynPorts` fields SHALL be required
- **AND** host/port validation SHALL be skipped

### Requirement: Explicit host declarations (no wildcards)
Dependency host values SHALL be explicit hostnames. Wildcard patterns are rejected in v1.

#### Scenario: Wildcard host rejected
- **WHEN** a dependency declares `host: "*.github.com"` or any glob/wildcard pattern
- **THEN** validation SHALL fail with an error indicating wildcards are not supported

### Requirement: Auth type declarations
Each dependency auth block SHALL declare a `type` field indicating how auth is applied. The `authType` is exposed to nodes via `ctx.dependency()`.

#### Scenario: Auth type field is open-ended
- **GIVEN** contract version "1"
- **THEN** common auth type examples include: `bearer-token`, `api-key`, `sas-token`, `password`, `webhook-url`. The set is open; any string is accepted.

### Requirement: Extensibility namespace
The contract SHALL support `x-*` extension fields for provider-specific metadata without breaking core schema validation.

#### Scenario: Extension field preserved
- **WHEN** a contract includes a `x-*` extension key
- **THEN** validation SHALL preserve the extension field and enforce core schema requirements
