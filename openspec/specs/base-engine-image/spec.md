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

### Requirement: Dockerfile caches engine dependencies at build time with lockfile
The generated Dockerfile SHALL pre-cache engine dependencies at build time via `deno cache` using a lockfile for integrity verification.

#### Scenario: Lockfile copied into image
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain `COPY .engine/deno.lock /app/deno.lock`

#### Scenario: Engine deps cached with lockfile
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain a `RUN` instruction that caches `engine/main.ts` dependencies using `--lock=deno.lock`
- **AND** the cache instruction SHALL NOT use `--no-lock`

### Requirement: Dockerfile does NOT override DENO_DIR
The generated Dockerfile SHALL NOT set the `DENO_DIR` environment variable. Engine dependencies are cached at build time to the distroless default `/deno-dir/` and served from that read-only image layer at runtime.

#### Scenario: No DENO_DIR environment variable
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL NOT contain `ENV DENO_DIR`

### Requirement: Dockerfile ENTRYPOINT uses default workflow path
The generated Dockerfile ENTRYPOINT SHALL include `--workflow /app/workflow/workflow.yaml` as the default workflow path, matching the ConfigMap mount point used in Phase 2.

#### Scenario: Default workflow path in ENTRYPOINT
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--workflow` followed by `/app/workflow/workflow.yaml`

#### Scenario: Port argument in ENTRYPOINT
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--port 8080`

### Requirement: Dockerfile ENTRYPOINT uses broad permissions as fallback
The generated Dockerfile ENTRYPOINT SHALL use broad Deno permission flags as a fallback. When a workflow has contract dependencies, the K8s Deployment manifest overrides the ENTRYPOINT with scoped flags via `command` and `args`.

#### Scenario: Broad fallback permissions in ENTRYPOINT
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT SHALL include `--allow-net` (broad, scoped at deploy time via K8s args)
- **AND** the ENTRYPOINT SHALL include `--allow-read=/app,/var/run/secrets`
- **AND** the ENTRYPOINT SHALL include `--allow-write=/tmp`
- **AND** the ENTRYPOINT SHALL include `--allow-env`
- **AND** the ENTRYPOINT SHALL NOT include `--allow-all`

#### Scenario: K8s Deployment overrides ENTRYPOINT when contract exists
- **WHEN** `GenerateK8sManifests()` is called for a workflow with contract dependencies
- **THEN** the Deployment container spec SHALL include `command` and `args` fields that override the image ENTRYPOINT
- **AND** `--allow-net` SHALL be replaced with `--allow-net=<host1>:<port>,<host2>:<port>,0.0.0.0:8080` scoped to declared dependencies and the trigger listener
- **AND** if any dependency has `type: dynamic-target`, `--allow-net` SHALL remain broad (no host restriction)
- **AND** `--allow-env` SHALL be scoped to `--allow-env=DENO_DIR,HOME`
- **AND** numeric args (e.g., port `8080`) SHALL be YAML-quoted to prevent integer interpretation

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
