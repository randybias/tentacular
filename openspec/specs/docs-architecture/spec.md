# docs-architecture Specification

## Purpose
Requirements for the architecture documentation to reflect the engine-only build phase and ConfigMap deploy phase.

## Requirements
### Requirement: Architecture doc describes engine-only build phase
The "Build Phase" section in `docs/architecture.md` SHALL describe the engine-only base image build flow.

#### Scenario: Build phase steps
- **WHEN** the "Build Phase" section is read
- **THEN** it SHALL describe: generate engine-only Dockerfile, copy engine to build context, `docker build` produces base image, default tag `pipedreamer-engine:latest`, image tag saved to `.pipedreamer/base-image.txt`
- **AND** it SHALL NOT mention copying workflow.yaml or nodes/ into the build context

### Requirement: Architecture doc describes ConfigMap deploy phase
The "Deploy Phase" section SHALL describe ConfigMap generation, image resolution, and rollout restart.

#### Scenario: Deploy phase steps
- **WHEN** the "Deploy Phase" section is read
- **THEN** it SHALL describe: parse workflow.yaml, resolve base image via cascade (`--image` > `.pipedreamer/base-image.txt` > `pipedreamer-engine:latest`), generate ConfigMap from workflow code, generate K8s manifests, apply all, rollout restart
- **AND** it SHALL NOT reference `--cluster-registry`

### Requirement: Generated K8s Resources table includes ConfigMap
The "Generated K8s Resources" table SHALL include the ConfigMap resource.

#### Scenario: ConfigMap row in table
- **WHEN** the "Generated K8s Resources" table is read
- **THEN** it SHALL include a row for ConfigMap with name `{workflow-name}-code` and key fields describing workflow code (workflow.yaml + nodes/*.ts)
- **AND** the Deployment row SHALL mention the code volume mount at `/app/workflow`

### Requirement: Package table reflects engine-only Dockerfile
The package layout table SHALL describe `GenerateDockerfile()` as producing an engine-only Dockerfile.

#### Scenario: dockerfile.go description
- **WHEN** the package table entry for `dockerfile.go` is read
- **THEN** it SHALL describe engine-only Dockerfile generation (not monolithic)
