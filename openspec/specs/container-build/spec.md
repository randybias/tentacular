# container-build Specification

## Purpose
TBD - created by archiving change build-and-deploy. Update Purpose after archive.
## Requirements
### Requirement: Dockerfile generation uses distroless Deno base
The `pkg/builder/dockerfile.go` `GenerateDockerfile()` function SHALL produce a Dockerfile that uses `denoland/deno:distroless` as the base image.

#### Scenario: Base image is distroless
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the returned Dockerfile string SHALL contain `FROM denoland/deno:distroless` as the first instruction

#### Scenario: Workdir is /app
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the returned Dockerfile string SHALL set `WORKDIR /app`

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

### Requirement: Dockerfile caches Deno dependencies with lockfile integrity
The generated Dockerfile SHALL run `deno cache` with lockfile verification to pre-cache dependencies at build time.

#### Scenario: Lockfile copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/deno.lock /app/deno.lock`

#### Scenario: Dependency caching with lockfile
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain a `RUN` instruction that caches `engine/main.ts` dependencies using `--lock=deno.lock`
- **AND** the cache instruction SHALL NOT use `--no-lock`

### Requirement: Dockerfile sets secure Deno entrypoint
The generated Dockerfile SHALL use a Deno entrypoint with minimal permissions and a default workflow path.

#### Scenario: Permission flags (broad fallback)
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--allow-net` (broad fallback; scoped to specific hosts at deploy time via K8s args when contract exists)
- **AND** the ENTRYPOINT SHALL include `--allow-read=/app,/var/run/secrets` (read engine, workflow, and secret files)
- **AND** the ENTRYPOINT SHALL include `--allow-write=/tmp` (temporary file writes only)
- **AND** the ENTRYPOINT SHALL include `--allow-env` (runtime configuration)
- **AND** the ENTRYPOINT SHALL NOT include `--allow-all`
- **AND** the ENTRYPOINT SHALL include `--no-lock` (lockfile integrity is enforced at build time via `--lock=deno.lock` in `deno cache`; runtime uses `--no-lock` because the read-only root filesystem prevents lock file writes)

#### Scenario: Workflow path argument
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL pass `--workflow /app/workflow/workflow.yaml` to `engine/main.ts`

#### Scenario: Port exposed
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL `EXPOSE 8080`
- **AND** the ENTRYPOINT SHALL pass `--port 8080` to `engine/main.ts`

### Requirement: CLI excludes itself from container
The generated Dockerfile SHALL NOT copy the Go CLI binary into the container.

#### Scenario: No CLI in container
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain any `COPY` instruction referencing `tentacular` binary, `cmd/`, or `pkg/`

### Requirement: Build command generates Dockerfile and invokes docker build
The `tntc build` command SHALL generate a temporary Dockerfile, copy the engine into the build context, build the image, and clean up.

#### Scenario: Successful build
- **WHEN** `tntc build` is executed in a directory containing a valid `workflow.yaml`
- **THEN** it SHALL generate `Dockerfile.tentacular` in the workflow directory
- **AND** it SHALL copy the engine into `.engine/` in the workflow directory
- **AND** it SHALL invoke `docker build -f Dockerfile.tentacular -t <tag> .`
- **AND** upon success, it SHALL remove `Dockerfile.tentacular` and `.engine/`

#### Scenario: Build cleanup on failure
- **WHEN** `tntc build` is executed and `docker build` fails
- **THEN** it SHALL still remove `Dockerfile.tentacular` and `.engine/` (deferred cleanup)
- **AND** it SHALL return an error indicating the docker build failed

### Requirement: Image tag derivation
The build command SHALL use `tentacular-engine:latest` as the default image tag when `--tag` is not specified.

#### Scenario: Default tag
- **WHEN** `tntc build` is executed without `--tag`
- **THEN** the image tag SHALL be `tentacular-engine:latest`

#### Scenario: Custom tag
- **WHEN** `tntc build --tag my-image:latest` is executed
- **THEN** the image tag SHALL be `my-image:latest`

#### Scenario: Registry prefix
- **WHEN** `tntc build --registry gcr.io/myproject` is executed without `--tag`
- **THEN** the image tag SHALL be `gcr.io/myproject/tentacular-engine:latest`

### Requirement: Build validates workflow spec
The build command SHALL validate the workflow spec before building.

#### Scenario: Invalid spec rejected
- **WHEN** `tntc build` is executed in a directory with an invalid `workflow.yaml`
- **THEN** it SHALL return an error indicating validation failures
- **AND** it SHALL NOT invoke `docker build`

#### Scenario: Missing spec rejected
- **WHEN** `tntc build` is executed in a directory without `workflow.yaml`
- **THEN** it SHALL return an error indicating the spec file was not found

### Requirement: Build accepts directory argument
The build command SHALL accept an optional directory argument.

#### Scenario: Explicit directory
- **WHEN** `tntc build ./my-workflow` is executed
- **THEN** it SHALL look for `workflow.yaml` in the `./my-workflow/` directory

#### Scenario: Default directory
- **WHEN** `tntc build` is executed without arguments
- **THEN** it SHALL look for `workflow.yaml` in the current directory

### Requirement: Engine directory discovery
The build command SHALL locate the engine directory from the tentacular installation.

#### Scenario: Engine found
- **WHEN** the build command runs
- **THEN** it SHALL locate the `engine/` directory relative to the tentacular binary or a known installation path

#### Scenario: Engine not found
- **WHEN** the engine directory cannot be located
- **THEN** the build command SHALL return a clear error: "cannot find engine directory"

