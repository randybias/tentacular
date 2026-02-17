## ADDED Requirements

### Requirement: ctx.dependency() API for external service metadata
The Context interface SHALL provide a `dependency(name)` method that returns connection metadata for a declared contract dependency.

#### Scenario: Access dependency connection metadata
- **WHEN** a node calls `ctx.dependency("postgres")`
- **THEN** the returned object SHALL contain `host`, `port`, `protocol`, `authType`, and protocol-specific fields (`database`, `user`) as declared in the contract
- **AND** the `secret` field SHALL contain the resolved secret value (eagerly resolved from mounted K8s secrets at call time)

#### Scenario: Access HTTPS dependency
- **WHEN** a node calls `ctx.dependency("github-api")`
- **THEN** the returned object SHALL contain `host`, `port`, `protocol: "https"`, `authType: "bearer-token"`, `secret` (resolved auth token), and a `fetch(path, init?)` convenience method

#### Scenario: HTTPS dependency fetch builds URL without auth injection
- **WHEN** a node calls `dep.fetch("/path")` on an HTTPS dependency
- **THEN** the fetch SHALL build the URL as `https://<host>:<port>/path` without injecting any auth headers
- **AND** the node SHALL use `dep.secret` and `dep.authType` to set auth explicitly

#### Scenario: Undeclared dependency access
- **WHEN** a node calls `ctx.dependency("unknown-service")`
- **THEN** the call SHALL throw an error indicating the dependency is not declared in the contract

### Requirement: ctx.dependency() replaces manual config/secrets assembly
Nodes SHALL use `ctx.dependency()` to access external service connection info instead of reading from `ctx.config` and `ctx.secrets` directly.

#### Scenario: Current pattern replaced
- **GIVEN** a node currently reads `ctx.config.pg_host` and `ctx.secrets["postgres"]["password"]`
- **WHEN** migrated to contract model
- **THEN** the node SHALL use `const pg = ctx.dependency("postgres")` to access `pg.host`, `pg.port`, `pg.database`, `pg.user`, `pg.secret`

### Requirement: Mock context dependency support
The mock context SHALL support `ctx.dependency()` for testing with mock values.

#### Scenario: Mock dependency returns test values
- **WHEN** a node calls `ctx.dependency("postgres")` in mock context
- **THEN** the mock SHALL return dependency metadata with mock/empty values consistent with existing mock patterns
- **AND** the mock SHALL record the access for runtime-tracing drift detection

#### Scenario: Mock dependency access recording
- **WHEN** mock test completes
- **THEN** all `ctx.dependency()` calls SHALL be recorded with dependency names for contract drift comparison

#### Scenario: Mock dependency fetch recording
- **WHEN** a node calls `dep.fetch(path)` on a mock dependency
- **THEN** the mock SHALL return a mock response and record the fetch call (dependency name, path) for drift detection

### Requirement: ctx.fetch() and ctx.secrets direct usage flagged
Runtime tracing SHALL detect and flag direct `ctx.fetch()` and `ctx.secrets` usage as contract violations when a contract is present.

#### Scenario: Direct ctx.fetch flagged
- **WHEN** a node calls `ctx.fetch("github", "/repos/...")` instead of using `ctx.dependency("github-api")`
- **THEN** drift detection SHALL flag this as a violation with guidance to use `ctx.dependency()`

#### Scenario: Direct ctx.secrets flagged
- **WHEN** a node reads `ctx.secrets["postgres"]["password"]` instead of using `ctx.dependency("postgres").secret`
- **THEN** drift detection SHALL flag this as a violation

### Requirement: Inter-node data flow unchanged
The `input` parameter to node functions SHALL continue to carry upstream node outputs. `ctx.dependency()` is exclusively for external service metadata from the contract.

#### Scenario: Input parameter unaffected
- **WHEN** a fan-in node receives data from upstream nodes
- **THEN** the `input` parameter SHALL contain merged output keyed by upstream node name, unchanged from current behavior
