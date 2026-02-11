## ADDED Requirements

### Requirement: Deployment guide describes engine-only build
The Build section in `tentacular-skill/references/deployment-guide.md` SHALL describe building an engine-only base image.

#### Scenario: What It Does steps updated
- **WHEN** the "What It Does" steps are read
- **THEN** they SHALL describe: generate engine-only Dockerfile, copy engine to build context, `docker build`, save tag to `.tentacular/base-image.txt`
- **AND** they SHALL NOT mention copying workflow.yaml or nodes/

#### Scenario: Generated Dockerfile example updated
- **WHEN** the "Generated Dockerfile" section is read
- **THEN** the Dockerfile example SHALL show engine-only content: `COPY .engine/`, `COPY .engine/deno.json`, `RUN deno cache engine/main.ts`, `ENV DENO_DIR=/tmp/deno-cache`
- **AND** it SHALL NOT contain `COPY workflow.yaml` or `COPY nodes/`
- **AND** the ENTRYPOINT SHALL include `--workflow /app/workflow/workflow.yaml`

#### Scenario: Image tag format updated
- **WHEN** the "Image Tag" section is read
- **THEN** it SHALL state the default tag is `tentacular-engine:latest`
- **AND** it SHALL describe override with `--tag`

### Requirement: Deployment guide describes ConfigMap deploy
The Deploy section SHALL describe ConfigMap generation and the new image resolution cascade.

#### Scenario: Generated Manifests includes ConfigMap
- **WHEN** the "Generated Manifests" section is read
- **THEN** it SHALL include a ConfigMap manifest example with name `{workflow-name}-code` and data keys for workflow.yaml and nodes
- **AND** the Deployment manifest example SHALL include a `code` volume (configMap) and volumeMount at `/app/workflow`

#### Scenario: Flags table updated
- **WHEN** the deploy Flags table is read
- **THEN** it SHALL include `--image` flag for specifying the base engine image
- **AND** it SHALL NOT include `--cluster-registry`

### Requirement: Deployment guide includes Fast Iteration section
A new "Fast Iteration" section SHALL explain the edit-deploy workflow that bypasses Docker builds.

#### Scenario: Fast Iteration workflow
- **WHEN** the "Fast Iteration" section is read
- **THEN** it SHALL explain: edit workflow code → `tntc deploy` → ConfigMap updated + rollout restart → done
- **AND** it SHALL state that no Docker build is needed for code-only changes

### Requirement: Full Lifecycle example updated
The "Full Lifecycle" example SHALL show the new two-phase workflow.

#### Scenario: Lifecycle commands updated
- **WHEN** the "Full Lifecycle" example is read
- **THEN** it SHALL show `tntc build` (once, for engine image) followed by `tntc deploy` (for each code change)
- **AND** it SHALL use `--image` instead of `--cluster-registry`
