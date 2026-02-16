## ADDED Requirements

### Requirement: Kind cluster auto-detection
The deploy pipeline SHALL auto-detect kind clusters and adjust deployment parameters accordingly.

#### Scenario: Kind cluster detected
- **GIVEN** the current kubeconfig context has a "kind-" prefix AND the server address is localhost/127.0.0.1
- **WHEN** `tntc deploy` or `tntc test --live` is executed
- **THEN** the deploy SHALL set runtimeClass to empty string (no gVisor)
- **AND** set imagePullPolicy to "IfNotPresent"
- **AND** print a diagnostic message identifying the kind cluster

#### Scenario: Kind image loading
- **GIVEN** a kind cluster is detected
- **WHEN** `tntc build` completes
- **THEN** it SHALL call `kind load docker-image` to make the image available in the kind cluster

#### Scenario: Non-kind cluster
- **GIVEN** the current kubeconfig context does not match kind heuristics
- **WHEN** `tntc deploy` is executed
- **THEN** default deployment parameters SHALL be used (gVisor runtime class, Always pull policy)

### Requirement: Deploy gate with live test
Default `tntc deploy` SHALL run a live test before deploying when a dev environment is configured.

#### Scenario: Deploy with passing live test
- **GIVEN** a dev environment is configured
- **WHEN** `tntc deploy` is executed without `--force`
- **THEN** it SHALL run a live test first
- **AND** if the live test passes, proceed with the deploy

#### Scenario: Deploy with failing live test
- **GIVEN** a dev environment is configured
- **WHEN** `tntc deploy` is executed without `--force` and the live test fails
- **THEN** it SHALL abort the deploy with a structured error
- **AND** include hints about fixing the failure or using `--force`

#### Scenario: Deploy with --force flag
- **WHEN** `tntc deploy --force` is executed
- **THEN** it SHALL skip the live test and deploy directly

#### Scenario: Deploy without dev environment
- **GIVEN** no dev environment is configured
- **WHEN** `tntc deploy` is executed
- **THEN** it SHALL deploy directly without running a live test

### Requirement: Post-deploy verification
The deploy command SHALL support a `--verify` flag to validate the deployment after apply.

#### Scenario: Verify runs workflow
- **WHEN** `tntc deploy --verify` is executed after a successful deploy
- **THEN** it SHALL trigger the workflow once and validate the execution result

#### Scenario: Verify with JSON output
- **WHEN** `tntc deploy --verify -o json` is executed
- **THEN** the output SHALL include phases: [preflight, live-test, deploy, verify]

### Requirement: Config overrides in deployment
The deploy pipeline SHALL accept config overrides from environment configuration.

#### Scenario: Environment config overrides
- **GIVEN** an environment config with `config_overrides: {api_url: "http://dev-api"}`
- **WHEN** deploying to that environment
- **THEN** the config overrides SHALL be merged into the workflow ConfigMap

### Requirement: Image pull policy in manifest
The generated Deployment manifest SHALL support configurable imagePullPolicy.

#### Scenario: Default pull policy
- **WHEN** a Deployment manifest is generated without kind detection
- **THEN** imagePullPolicy SHALL default to "Always"

#### Scenario: Kind pull policy
- **WHEN** a Deployment manifest is generated with kind cluster detected
- **THEN** imagePullPolicy SHALL be "IfNotPresent"
