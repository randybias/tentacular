## ADDED Requirements

### Requirement: Live workflow testing
The `tntc test --live` command SHALL deploy a workflow to a configured dev environment, execute it, validate the result, and clean up.

#### Scenario: Successful live test
- **GIVEN** a valid environment config for "dev" with context, namespace, and image
- **WHEN** `tntc test --live` is executed in a workflow directory
- **THEN** it SHALL deploy the workflow to the dev environment namespace
- **AND** wait for deployment Ready (ReadyReplicas == Replicas)
- **AND** trigger workflow execution
- **AND** validate the execution result
- **AND** clean up deployed resources
- **AND** emit a structured pass result

#### Scenario: Live test with custom environment
- **WHEN** `tntc test --live --env staging` is executed
- **THEN** it SHALL use the "staging" environment config instead of "dev"

#### Scenario: Live test with --keep flag
- **WHEN** `tntc test --live --keep` is executed
- **THEN** it SHALL NOT clean up deployed resources after the test completes

#### Scenario: Live test timeout
- **GIVEN** `--timeout 60s` is specified
- **WHEN** the deployment does not reach Ready within 60 seconds
- **THEN** the test SHALL fail with a timeout error and cleanup

#### Scenario: Live test without configured environment
- **WHEN** `tntc test --live` is executed and no "dev" environment is configured
- **THEN** it SHALL fail with an error indicating the environment is not configured
- **AND** the error SHALL include a hint to run `tntc configure` or add environment config

#### Scenario: Live test with kind cluster
- **GIVEN** the dev environment targets a kind cluster
- **WHEN** `tntc test --live` is executed
- **THEN** it SHALL auto-detect the kind cluster
- **AND** adjust deployment parameters (no gVisor, IfNotPresent pull policy)
- **AND** load the image into the kind cluster

### Requirement: JSON output for test results
The test command SHALL support `--output json` / `-o json` for structured output.

#### Scenario: JSON output for mock tests
- **WHEN** `tntc test . -o json` is executed
- **THEN** the output SHALL be a JSON `CommandResult` envelope with command "test", status "pass" or "fail", results array, and timing

#### Scenario: JSON output for live tests
- **WHEN** `tntc test --live -o json` is executed
- **THEN** the output SHALL be a JSON `CommandResult` envelope with live test phases and execution result
