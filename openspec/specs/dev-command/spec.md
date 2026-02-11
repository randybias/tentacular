# dev-command Specification

## Purpose
TBD - created by archiving change dev-command. Update Purpose after archive.
## Requirements
### Requirement: Dev server starts and listens on configured port
The `tntc dev [dir]` command SHALL start a local development server that listens for HTTP requests on the configured port.

#### Scenario: Default port startup
- **WHEN** `tntc dev ./my-workflow` is executed without `--port`
- **THEN** the engine SHALL start an HTTP server listening on port 8080
- **THEN** the console SHALL print the listening address and available endpoints

#### Scenario: Custom port startup
- **WHEN** `tntc dev ./my-workflow --port 9090` is executed
- **THEN** the engine SHALL start an HTTP server listening on port 9090

#### Scenario: Missing workflow.yaml
- **WHEN** `tntc dev ./empty-dir` is executed and `./empty-dir/workflow.yaml` does not exist
- **THEN** the command SHALL fail with an error message indicating no workflow.yaml was found

#### Scenario: DAG compilation output on startup
- **WHEN** the dev server starts successfully
- **THEN** the console SHALL print the workflow name, version, number of stages, number of nodes, and the node composition of each stage

### Requirement: File changes trigger hot-reload
The engine SHALL watch the workflow directory for file changes and hot-reload node modules when relevant files are modified.

#### Scenario: Node file modification triggers reload
- **WHEN** the dev server is running with `--watch` enabled
- **WHEN** a `.ts` file in the workflow directory is modified
- **THEN** the engine SHALL detect the change within 1 second
- **THEN** the engine SHALL clear the module cache and re-import all node modules
- **THEN** the console SHALL print which files changed and confirm reload completion

#### Scenario: Non-relevant file changes are ignored
- **WHEN** a file with an extension other than `.ts`, `.js`, `.yaml`, or `.json` is modified
- **THEN** the engine SHALL NOT trigger a reload

#### Scenario: Rapid successive changes are debounced
- **WHEN** multiple files are modified within 200ms of each other
- **THEN** the engine SHALL trigger only one reload after the debounce period

#### Scenario: Reload failure does not crash the server
- **WHEN** a node file is modified with a syntax error
- **THEN** the engine SHALL log the reload error to console
- **THEN** the server SHALL continue running with the previously loaded modules

### Requirement: HTTP /run endpoint executes workflow
The HTTP server SHALL expose a `/run` endpoint that triggers workflow execution and returns structured results.

#### Scenario: Successful workflow execution via POST
- **WHEN** a POST request is sent to `http://localhost:8080/run`
- **THEN** the engine SHALL execute the complete workflow through all DAG stages
- **THEN** the response SHALL be a JSON object with `success: true`, `outputs`, and `timing` fields
- **THEN** the HTTP status code SHALL be 200

#### Scenario: Successful workflow execution via GET
- **WHEN** a GET request is sent to `http://localhost:8080/run`
- **THEN** the engine SHALL execute the workflow identically to a POST request

#### Scenario: Workflow execution failure
- **WHEN** a `/run` request is made and a node throws an error during execution
- **THEN** the response SHALL be a JSON object with `success: false` and `errors` field containing the node error
- **THEN** the HTTP status code SHALL be 500

#### Scenario: Execution after hot-reload
- **WHEN** a node file is modified and hot-reload completes
- **WHEN** a `/run` request is sent
- **THEN** the engine SHALL execute the updated node code, not the previously cached version

### Requirement: Health check endpoint responds
The HTTP server SHALL expose a `/health` endpoint for readiness checks.

#### Scenario: Health check response
- **WHEN** a GET request is sent to `http://localhost:8080/health`
- **THEN** the response SHALL be `{"status":"ok"}` with content-type `application/json`
- **THEN** the HTTP status code SHALL be 200

#### Scenario: Unknown route returns 404
- **WHEN** a request is sent to a path other than `/run` or `/health`
- **THEN** the HTTP status code SHALL be 404

### Requirement: Graceful shutdown on signal
The dev server SHALL shut down cleanly when interrupted.

#### Scenario: SIGINT triggers shutdown
- **WHEN** the dev server is running
- **WHEN** a SIGINT signal is received (e.g., Ctrl+C)
- **THEN** the Go CLI SHALL forward SIGTERM to the Deno child process
- **THEN** the console SHALL print a shutdown message
- **THEN** the process SHALL exit with code 0

#### Scenario: SIGTERM triggers shutdown
- **WHEN** a SIGTERM signal is received
- **THEN** the behavior SHALL be identical to SIGINT handling

### Requirement: Local secrets file support
The engine SHALL load secrets from a local `.secrets.yaml` file for development use.

#### Scenario: Secrets loaded from .secrets.yaml
- **WHEN** the dev server starts and `.secrets.yaml` exists in the workflow directory
- **THEN** the engine SHALL parse the YAML file and populate `ctx.secrets` with its contents
- **THEN** node functions SHALL be able to access secrets via `ctx.secrets`

#### Scenario: Missing .secrets.yaml does not prevent startup
- **WHEN** the dev server starts and `.secrets.yaml` does not exist in the workflow directory
- **THEN** the engine SHALL start normally with an empty secrets object

### Requirement: Execution trace output
The engine SHALL print execution trace information to the console during workflow runs.

#### Scenario: Node timing output
- **WHEN** a workflow execution completes via `/run`
- **THEN** the console SHALL print the duration of each node execution in milliseconds

#### Scenario: Overall execution summary
- **WHEN** a workflow execution completes via `/run`
- **THEN** the console SHALL print the total execution duration and success/failure status

