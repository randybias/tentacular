## ADDED Requirements

### Requirement: Dockerfile generation uses distroless Deno base
The `pkg/builder/dockerfile.go` `GenerateDockerfile()` function SHALL produce a Dockerfile that uses `denoland/deno:distroless` as the base image.

#### Scenario: Base image is distroless
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the returned Dockerfile string SHALL contain `FROM denoland/deno:distroless` as the first instruction

#### Scenario: Workdir is /app
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the returned Dockerfile string SHALL set `WORKDIR /app`

### Requirement: Dockerfile copies engine, workflow, and nodes
The generated Dockerfile SHALL copy the engine directory, workflow.yaml, and nodes/ directory into the container.

#### Scenario: Engine copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/ /app/engine/`

#### Scenario: Workflow spec copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY workflow.yaml /app/`

#### Scenario: Nodes copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY nodes/ /app/nodes/`

#### Scenario: Import map copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/deno.json /app/deno.json` so Deno import map resolution works inside the container

### Requirement: Dockerfile caches Deno dependencies
The generated Dockerfile SHALL run `deno cache` to pre-cache dependencies at build time.

#### Scenario: Dependency caching
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain a `RUN` instruction that caches `engine/main.ts` dependencies

### Requirement: Dockerfile sets secure Deno entrypoint
The generated Dockerfile SHALL use a Deno entrypoint with minimal permissions.

#### Scenario: Permission flags
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--allow-net` (for HTTP trigger server)
- **AND** the ENTRYPOINT SHALL include `--allow-read=/app` (read workflow and engine files)
- **AND** the ENTRYPOINT SHALL include `--allow-write=/tmp` (temporary file writes only)
- **AND** the ENTRYPOINT SHALL NOT include `--allow-all` or `--allow-env`

#### Scenario: Workflow path argument
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL pass `--workflow /app/workflow.yaml` to `engine/main.ts`

#### Scenario: Port exposed
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL `EXPOSE 8080`
- **AND** the ENTRYPOINT SHALL pass `--port 8080` to `engine/main.ts`

### Requirement: CLI excludes itself from container
The generated Dockerfile SHALL NOT copy the Go CLI binary into the container.

#### Scenario: No CLI in container
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain any `COPY` instruction referencing `pipedreamer` binary, `cmd/`, or `pkg/`

### Requirement: Build command generates Dockerfile and invokes docker build
The `pipedreamer build` command SHALL generate a temporary Dockerfile, copy the engine into the build context, build the image, and clean up.

#### Scenario: Successful build
- **WHEN** `pipedreamer build` is executed in a directory containing a valid `workflow.yaml`
- **THEN** it SHALL generate `Dockerfile.pipedreamer` in the workflow directory
- **AND** it SHALL copy the engine into `.engine/` in the workflow directory
- **AND** it SHALL invoke `docker build -f Dockerfile.pipedreamer -t <tag> .`
- **AND** upon success, it SHALL remove `Dockerfile.pipedreamer` and `.engine/`

#### Scenario: Build cleanup on failure
- **WHEN** `pipedreamer build` is executed and `docker build` fails
- **THEN** it SHALL still remove `Dockerfile.pipedreamer` and `.engine/` (deferred cleanup)
- **AND** it SHALL return an error indicating the docker build failed

### Requirement: Image tag derivation
The build command SHALL derive the image tag from the workflow name and version.

#### Scenario: Default tag
- **WHEN** `pipedreamer build` is executed without `--tag`
- **THEN** the image tag SHALL be `<workflow-name>:<version-with-dots-as-dashes>`
- **EXAMPLE** workflow name `data-pipeline` version `1.0` produces tag `data-pipeline:1-0`

#### Scenario: Custom tag
- **WHEN** `pipedreamer build --tag my-image:latest` is executed
- **THEN** the image tag SHALL be `my-image:latest`

#### Scenario: Registry prefix
- **WHEN** `pipedreamer build --registry gcr.io/myproject` is executed
- **THEN** the image tag SHALL be prefixed with the registry: `gcr.io/myproject/data-pipeline:1-0`

### Requirement: Build validates workflow spec
The build command SHALL validate the workflow spec before building.

#### Scenario: Invalid spec rejected
- **WHEN** `pipedreamer build` is executed in a directory with an invalid `workflow.yaml`
- **THEN** it SHALL return an error indicating validation failures
- **AND** it SHALL NOT invoke `docker build`

#### Scenario: Missing spec rejected
- **WHEN** `pipedreamer build` is executed in a directory without `workflow.yaml`
- **THEN** it SHALL return an error indicating the spec file was not found

### Requirement: Build accepts directory argument
The build command SHALL accept an optional directory argument.

#### Scenario: Explicit directory
- **WHEN** `pipedreamer build ./my-workflow` is executed
- **THEN** it SHALL look for `workflow.yaml` in the `./my-workflow/` directory

#### Scenario: Default directory
- **WHEN** `pipedreamer build` is executed without arguments
- **THEN** it SHALL look for `workflow.yaml` in the current directory

### Requirement: Engine directory discovery
The build command SHALL locate the engine directory from the pipedreamer installation.

#### Scenario: Engine found
- **WHEN** the build command runs
- **THEN** it SHALL locate the `engine/` directory relative to the pipedreamer binary or a known installation path

#### Scenario: Engine not found
- **WHEN** the engine directory cannot be located
- **THEN** the build command SHALL return a clear error: "cannot find engine directory"
