## ADDED Requirements

### Requirement: Live test command flags
`tntc test` SHALL accept flags: `--live` (boolean, run live test), `--env` (string, default "dev"), `--keep` (boolean, skip cleanup), `--timeout` (duration, default 120s). When `--live` is set, the command SHALL call `runLiveTest` instead of the Deno test runner.

#### Scenario: Default live test
- **WHEN** `tntc test --live` is invoked without other flags
- **THEN** the test SHALL run against the `dev` environment with 120s timeout and cleanup enabled

#### Scenario: Custom environment and timeout
- **WHEN** `tntc test --live --env staging --timeout 180s` is invoked
- **THEN** the test SHALL run against the `staging` environment with 180s timeout

### Requirement: Live test deployment flow
`runLiveTest` SHALL: load environment config, switch kubeconfig context, detect kind cluster, deploy workflow to environment namespace with config overrides, wait for deployment readiness, trigger workflow execution, parse the execution result, clean up (unless `--keep`), and emit a structured result.

#### Scenario: Successful live test
- **WHEN** a workflow is live-tested against a configured dev environment and execution succeeds
- **THEN** the test SHALL report pass, clean up the deployment, and exit with code 0

#### Scenario: Live test with --keep
- **WHEN** `tntc test --live --keep` completes
- **THEN** the deployed workflow SHALL remain in the environment namespace

#### Scenario: Live test timeout
- **WHEN** the deployment does not reach Ready status within the timeout
- **THEN** the test SHALL fail with a timeout error and still attempt cleanup

### Requirement: Wait for deployment readiness
`WaitForReady()` SHALL poll the Deployment until `ReadyReplicas == Replicas` or the timeout expires. Polling SHALL use the K8s API, not kubectl.

#### Scenario: Deployment becomes ready
- **WHEN** a Deployment's ReadyReplicas equals Replicas within the timeout
- **THEN** `WaitForReady()` SHALL return nil (no error)

#### Scenario: Deployment times out
- **WHEN** a Deployment does not reach readiness within the timeout
- **THEN** `WaitForReady()` SHALL return a timeout error

### Requirement: K8s client with explicit context
`NewClientWithContext(contextName string)` SHALL create a K8s client using the specified kubeconfig context, without modifying the global kubeconfig state.

#### Scenario: Create client with specific context
- **WHEN** `NewClientWithContext("kind-dev")` is called
- **THEN** the returned client SHALL operate against the cluster referenced by the `kind-dev` context

### Requirement: Deploy function extraction for reuse
The deploy pipeline SHALL extract `deployWorkflow(dir, namespace string, opts InternalDeployOptions) (*DeployResult, error)` from `runDeploy`. Both `runDeploy` and `runLiveTest` SHALL call this extracted function.

#### Scenario: Live test reuses deploy pipeline
- **WHEN** `runLiveTest` deploys a workflow
- **THEN** it SHALL use the same `deployWorkflow()` function as `tntc deploy`

### Requirement: Live test structured output
With `-o json`, live test SHALL emit a `CommandResult` with `command: "test"`, status, summary, and a `phases` array containing entries for deploy, wait, trigger, validate, and cleanup -- each with name, status, and durationMs.

#### Scenario: Live test JSON phases
- **WHEN** `tntc test --live -o json` completes successfully
- **THEN** the JSON SHALL include phases for deploy, wait, trigger, validate, and cleanup
