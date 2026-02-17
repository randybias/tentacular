## ADDED Requirements

### Requirement: Parser support for contract schema
`spec.Parse()` SHALL support parsing a top-level `contract` section in `workflow.yaml` with typed dependency structures.

#### Scenario: Valid contract parsed
- **WHEN** `workflow.yaml` contains a valid `contract` block with dependencies
- **THEN** `spec.Parse()` SHALL return a non-nil workflow object with parsed dependency metadata and protocol-specific fields

#### Scenario: Unknown protocol rejected
- **WHEN** a dependency declares an unsupported protocol
- **THEN** validation SHALL fail with an error listing supported protocols

### Requirement: Contract referential integrity validation
Validation SHALL enforce structural consistency within the contract section.

#### Scenario: Auth secret reference format validated
- **WHEN** a dependency declares `auth.secret` with a value containing a dot separator (e.g., `postgres.password`)
- **THEN** validation SHALL accept the reference as a valid secret key path

#### Scenario: Duplicate dependency name rejected
- **WHEN** two dependencies share the same name key in `contract.dependencies`
- **THEN** validation SHALL fail with a duplicate dependency error

### Requirement: Strict enforcement by default
Validation SHALL enforce strict contract compliance by default. Environment-level override to `audit` mode is supported.

#### Scenario: Strict mode drift failure (default)
- **WHEN** runtime tracing finds `ctx.fetch` or `ctx.secrets` usage not covered by a declared dependency
- **THEN** test SHALL fail with actionable mismatch errors

#### Scenario: Direct ctx.secrets/ctx.fetch flagged as violation
- **WHEN** node code uses `ctx.secrets` or `ctx.fetch` directly instead of `ctx.dependency()`
- **THEN** drift detection SHALL flag this as a contract violation with guidance to use the dependency API

#### Scenario: Audit mode via environment override
- **WHEN** an environment configuration sets `enforcement: audit`
- **THEN** validation SHALL report drift findings as warnings without failing

### Requirement: Contract absence handling
Workflows without a `contract` section SHALL be treated as legacy and contract-related features SHALL not apply.

#### Scenario: No contract block present
- **WHEN** `workflow.yaml` does not contain a `contract` section
- **THEN** `spec.Parse()` SHALL return a nil contract field, and contract validation, drift detection, and NetworkPolicy derivation SHALL be skipped

#### Scenario: Contract required in strict mode
- **WHEN** strict enforcement is active and a workflow lacks a `contract` section
- **THEN** validation SHALL warn that the workflow has no contract (but SHALL NOT fail, to allow incremental adoption)

### Requirement: Contract extension preservation
Parser/serializer SHALL preserve `x-*` extension fields in contract metadata through round-trip.

#### Scenario: Extension round-trip
- **WHEN** a workflow with `x-*` keys is parsed and re-emitted
- **THEN** extension metadata SHALL be preserved

**Implementation note:** The Go `Contract` struct SHALL use `Extensions map[string]interface{} \`yaml:",inline"\`` to capture `x-*` fields, following the same pattern as `WorkflowConfig.Extras` in `pkg/spec/types.go`.
