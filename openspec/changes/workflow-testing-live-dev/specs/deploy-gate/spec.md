## ADDED Requirements

### Requirement: Deploy gate runs live test before deploy
When a `dev` environment is configured and `--force` is not set, `tntc deploy` SHALL run a live test (`tntc test --live --env dev`) before applying manifests. If the live test fails, deploy SHALL abort without applying any manifests.

#### Scenario: Deploy with passing live test
- **WHEN** `tntc deploy` is executed, a `dev` environment is configured, and the live test passes
- **THEN** the deploy SHALL proceed with manifest generation and application

#### Scenario: Deploy with failing live test
- **WHEN** `tntc deploy` is executed, a `dev` environment is configured, and the live test fails
- **THEN** the deploy SHALL abort with a structured error including the test failure details
- **AND** no manifests SHALL be applied to the cluster

#### Scenario: Deploy without dev environment configured
- **WHEN** `tntc deploy` is executed and no `dev` environment exists in the config cascade
- **THEN** the deploy SHALL proceed without running a live test

### Requirement: Force flag skips live test gate
`tntc deploy` SHALL accept a `--force` flag (alias `--skip-live-test`). When set, the live test gate SHALL be skipped entirely.

#### Scenario: Deploy with --force
- **WHEN** `tntc deploy --force` is executed with a `dev` environment configured
- **THEN** the deploy SHALL proceed without running a live test

#### Scenario: Deploy with --skip-live-test alias
- **WHEN** `tntc deploy --skip-live-test` is executed
- **THEN** it SHALL behave identically to `--force`

### Requirement: Post-deploy verification
`tntc deploy` SHALL accept a `--verify` flag (default true when `-o json` is set). When enabled, after successful manifest application, the CLI SHALL trigger the deployed workflow once and validate the execution result.

#### Scenario: Verification succeeds
- **WHEN** `tntc deploy -o json` completes and the post-deploy workflow execution succeeds
- **THEN** the verify phase SHALL report status "pass"

#### Scenario: Verification fails
- **WHEN** `tntc deploy -o json` completes and the post-deploy workflow execution fails
- **THEN** the verify phase SHALL report status "fail" with the execution error
- **AND** the overall deploy status SHALL be "fail"

### Requirement: Deploy structured output with phases
With `-o json`, deploy SHALL emit a `CommandResult` with a `phases` array containing entries for each phase: preflight, live-test (if applicable), deploy, verify (if applicable). Each phase SHALL have `name`, `status`, and `durationMs`. Skipped phases SHALL be omitted from the array.

#### Scenario: Full deploy JSON output
- **WHEN** `tntc deploy -o json` completes with live test and verification
- **THEN** the JSON SHALL include phases for preflight, live-test, deploy, and verify

#### Scenario: Deploy with --force JSON output
- **WHEN** `tntc deploy --force -o json` completes
- **THEN** the JSON SHALL include phases for preflight and deploy only (no live-test phase)

#### Scenario: Failed deploy JSON output with hints
- **WHEN** the live test gate fails during `tntc deploy -o json`
- **THEN** the JSON SHALL have `status: "fail"` and `hints` SHALL include "Use --force to skip the live test gate"
