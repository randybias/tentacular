## ADDED Requirements

### Requirement: GenerateDockerfile takes no parameters
The `pkg/builder/dockerfile.go` `GenerateDockerfile()` function SHALL take no parameters and return a Dockerfile string for an engine-only image.

#### Scenario: No workflowDir parameter
- **WHEN** `GenerateDockerfile()` is called
- **THEN** it SHALL require no arguments
- **AND** it SHALL return a valid Dockerfile string without any filesystem interaction

### Requirement: Dockerfile excludes workflow code
The generated Dockerfile SHALL NOT contain any instructions that copy workflow code into the image.

#### Scenario: No workflow.yaml copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain `COPY workflow.yaml`

#### Scenario: No nodes directory copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain `COPY nodes/`

#### Scenario: No per-node cache lines
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain any `deno cache` instructions for files in `nodes/`

### Requirement: Dockerfile copies engine code only
The generated Dockerfile SHALL copy only the engine directory and import map into the image.

#### Scenario: Engine directory copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/ /app/engine/`

#### Scenario: Import map copied
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/deno.json /app/deno.json`

### Requirement: Dockerfile caches engine dependencies at build time
The generated Dockerfile SHALL pre-cache engine dependencies at build time via `deno cache`.

#### Scenario: Engine deps cached
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain a `RUN` instruction that caches `engine/main.ts` dependencies

### Requirement: Dockerfile sets DENO_DIR for runtime caching
The generated Dockerfile SHALL set `DENO_DIR` environment variable to enable runtime caching of third-party dependencies imported by workflow nodes.

#### Scenario: DENO_DIR environment variable
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `ENV DENO_DIR=/tmp/deno-cache`

### Requirement: Dockerfile ENTRYPOINT uses default workflow path
The generated Dockerfile ENTRYPOINT SHALL include `--workflow /app/workflow/workflow.yaml` as the default workflow path, matching the ConfigMap mount point used in Phase 2.

#### Scenario: Default workflow path in ENTRYPOINT
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--workflow` followed by `/app/workflow/workflow.yaml`

#### Scenario: Port argument in ENTRYPOINT
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--port 8080`

### Requirement: Dockerfile preserves existing security permissions
The generated Dockerfile ENTRYPOINT SHALL preserve the existing Deno permission flags without adding new ones.

#### Scenario: Existing permissions preserved
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--allow-net`
- **AND** the ENTRYPOINT SHALL include `--allow-read=/app,/var/run/secrets`
- **AND** the ENTRYPOINT SHALL include `--allow-write=/tmp`
- **AND** the ENTRYPOINT SHALL include `--allow-env`
- **AND** the ENTRYPOINT SHALL NOT include `--allow-all`

### Requirement: Build command saves image tag to file
After a successful build (and push if requested), the build command SHALL save the image tag to `.tentacular/base-image.txt` in the project root.

#### Scenario: Tag file written after build
- **WHEN** `tntc build` completes successfully
- **THEN** it SHALL write the full image tag to `.tentacular/base-image.txt`
- **AND** it SHALL create the `.tentacular/` directory if it does not exist

#### Scenario: Tag file written after push
- **WHEN** `tntc build --push --registry gcr.io/proj` completes successfully
- **THEN** `.tentacular/base-image.txt` SHALL contain the full pushed tag including registry prefix

### Requirement: Default image tag is tentacular-engine:latest
When `--tag` is not specified, the build command SHALL use `tentacular-engine:latest` as the default image tag.

#### Scenario: Default tag without --tag flag
- **WHEN** `tntc build` is executed without `--tag`
- **THEN** the image tag SHALL be `tentacular-engine:latest`

#### Scenario: Registry prefix with default tag
- **WHEN** `tntc build --registry gcr.io/myproject` is executed without `--tag`
- **THEN** the image tag SHALL be `gcr.io/myproject/tentacular-engine:latest`

#### Scenario: Custom tag overrides default
- **WHEN** `tntc build --tag my-engine:v2` is executed
- **THEN** the image tag SHALL be `my-engine:v2`
