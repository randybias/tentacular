## ADDED Requirements

### Requirement: Contract-aware validation command output
`tntc validate` SHALL include contract validation findings in both text and JSON output modes.

#### Scenario: Contract validation pass
- **WHEN** a workflow's declared contract is structurally valid
- **THEN** `tntc validate` SHALL report successful contract validation

#### Scenario: Contract structural error reported
- **WHEN** a contract has missing required fields or invalid protocol types
- **THEN** `tntc validate` SHALL report errors with remediation guidance

### Requirement: Runtime-tracing drift detection in test
`tntc test` SHALL capture dependency usage from mock context during test execution and compare against contract declarations.

#### Scenario: Undeclared dependency detected
- **WHEN** mock test records a `ctx.fetch` call to a host not derivable from any declared dependency
- **THEN** `tntc test` SHALL report the undeclared dependency and fail in strict mode

#### Scenario: Dead declaration detected
- **WHEN** a contract declares a dependency that no node accesses during test
- **THEN** `tntc test` SHALL report the unused declaration and fail in strict mode

**Note:** Dead declaration detection depends on test coverage of all dependency code paths. Dependencies used only in conditional branches (e.g., error handling, feature flags) may produce false positives if tests do not exercise those paths. Tests should aim to cover all dependency usage paths.

#### Scenario: Direct ctx.secrets bypass detected
- **WHEN** a node uses `ctx.secrets` directly instead of `ctx.dependency()`
- **THEN** `tntc test` SHALL report the bypass as a contract violation

### Requirement: Derived artifact display
CLI SHALL display derived artifacts (secret inventory, network policy summary) from contract declarations.

#### Scenario: Show derived secrets
- **WHEN** `tntc validate` runs on a workflow with contract
- **THEN** output SHALL include the derived secret key inventory

#### Scenario: Show derived network policy
- **WHEN** `tntc validate` runs on a workflow with contract
- **THEN** output SHALL include the derived egress/ingress policy summary

### Requirement: Rich visualization output
`tntc visualize --rich` SHALL produce DAG topology plus contract-derived metadata.

#### Scenario: Rich visualization content
- **WHEN** `tntc visualize --rich <workflow-dir>` is run
- **THEN** output SHALL include DAG edges, dependency nodes with protocol/host labels, derived secret inventory, and derived network intent

### Requirement: Co-resident visualization artifacts
Visualization SHALL write deterministic artifacts into the workflow directory.

#### Scenario: Write visualization artifacts
- **WHEN** visualization is run with write mode
- **THEN** Mermaid diagram and contract-summary artifacts SHALL be written co-resident with the workflow

### Requirement: Pre-build review artifact generation
CLI SHALL produce review artifacts suitable for agent/user planning loops before build/deploy.

#### Scenario: Pre-build contract review
- **WHEN** a workflow is prepared for build or deployment
- **THEN** CLI SHALL produce rich visualization + derived artifact summary for review
- **AND** artifacts SHALL be deterministic for PR diff review
