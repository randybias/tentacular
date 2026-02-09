## MODIFIED Requirements

### Requirement: Dockerfile copies engine, workflow, and nodes
The generated Dockerfile SHALL copy only the engine directory and import map into the container. Workflow code (workflow.yaml, nodes/) is no longer copied.

#### Scenario: Engine copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/ /app/engine/`

#### Scenario: Import map copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/deno.json /app/deno.json` so Deno import map resolution works inside the container

#### Scenario: Workflow spec NOT copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain `COPY workflow.yaml`

#### Scenario: Nodes NOT copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain `COPY nodes/`

### Requirement: Dockerfile sets secure Deno entrypoint
The generated Dockerfile SHALL use a Deno entrypoint with minimal permissions and a default workflow path.

#### Scenario: Permission flags
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--allow-net` (for HTTP trigger server)
- **AND** the ENTRYPOINT SHALL include `--allow-read=/app,/var/run/secrets` (read engine, workflow, and secret files)
- **AND** the ENTRYPOINT SHALL include `--allow-write=/tmp` (temporary file writes only)
- **AND** the ENTRYPOINT SHALL include `--allow-env` (runtime configuration)
- **AND** the ENTRYPOINT SHALL NOT include `--allow-all`

#### Scenario: Workflow path argument
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL pass `--workflow /app/workflow/workflow.yaml` to `engine/main.ts`

#### Scenario: Port exposed
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL `EXPOSE 8080`
- **AND** the ENTRYPOINT SHALL pass `--port 8080` to `engine/main.ts`

### Requirement: Image tag derivation
The build command SHALL use `pipedreamer-engine:latest` as the default image tag when `--tag` is not specified.

#### Scenario: Default tag
- **WHEN** `pipedreamer build` is executed without `--tag`
- **THEN** the image tag SHALL be `pipedreamer-engine:latest`

#### Scenario: Custom tag
- **WHEN** `pipedreamer build --tag my-image:latest` is executed
- **THEN** the image tag SHALL be `my-image:latest`

#### Scenario: Registry prefix
- **WHEN** `pipedreamer build --registry gcr.io/myproject` is executed without `--tag`
- **THEN** the image tag SHALL be `gcr.io/myproject/pipedreamer-engine:latest`
