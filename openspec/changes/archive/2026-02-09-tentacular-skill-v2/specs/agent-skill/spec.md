## ADDED Requirements

### Requirement: SKILL.md entry point exists
The `tentacular-skill/SKILL.md` file SHALL exist as the primary entry point for AI agents learning the Tentacular system.

#### Scenario: SKILL.md provides project overview
- **WHEN** an agent reads `tentacular-skill/SKILL.md`
- **THEN** the document SHALL contain a project overview describing the two-component architecture (Go CLI for management, Deno/TypeScript engine for execution)

#### Scenario: SKILL.md provides CLI quick-reference
- **WHEN** an agent reads `tentacular-skill/SKILL.md`
- **THEN** the document SHALL contain a quick-reference covering all CLI commands: `init`, `validate`, `dev`, `test`, `build`, `deploy`, `status`, `cluster check`, and `visualize`, each with usage syntax and key flags

#### Scenario: SKILL.md provides node contract summary
- **WHEN** an agent reads `tentacular-skill/SKILL.md`
- **THEN** the document SHALL include the node function contract showing `export default async function run(ctx: Context, input: T): Promise<U>` with a brief description of each Context member (`fetch`, `log`, `config`, `secrets`)

#### Scenario: SKILL.md provides workflow.yaml skeleton
- **WHEN** an agent reads `tentacular-skill/SKILL.md`
- **THEN** the document SHALL include a minimal valid `workflow.yaml` example with `name`, `version`, `triggers`, `nodes`, and `edges` fields

#### Scenario: SKILL.md links to references
- **WHEN** an agent needs detailed information on a specific topic
- **THEN** the SKILL.md SHALL link to the corresponding reference document in `references/` for each of: workflow specification, node development, testing, and deployment

### Requirement: Workflow specification reference exists
The `tentacular-skill/references/workflow-spec.md` file SHALL document the complete v2 workflow.yaml format.

#### Scenario: All top-level fields documented
- **WHEN** an agent reads `references/workflow-spec.md`
- **THEN** the document SHALL describe every top-level field: `name` (string, required, kebab-case), `version` (string, required, semver), `description` (string, optional), `triggers` (array, required), `nodes` (map, required), `edges` (array, required), and `config` (object, optional)

#### Scenario: Trigger types documented
- **WHEN** an agent reads `references/workflow-spec.md`
- **THEN** the document SHALL describe all trigger types: `manual`, `cron` (with `schedule` field), and `webhook` (with `path` field)

#### Scenario: Node spec format documented
- **WHEN** an agent reads `references/workflow-spec.md`
- **THEN** the document SHALL describe the node spec format: `path` (string, required, relative path to .ts file) and `capabilities` (map, optional)

#### Scenario: Edge format and validation documented
- **WHEN** an agent reads `references/workflow-spec.md`
- **THEN** the document SHALL describe edge format (`from`/`to` fields referencing node names), validation rules (references must point to defined nodes, no self-loops), and DAG acyclicity requirement

#### Scenario: Complete annotated example included
- **WHEN** an agent reads `references/workflow-spec.md`
- **THEN** the document SHALL include at least one complete, valid workflow.yaml example with multiple nodes, edges forming a DAG, and inline comments explaining each section

### Requirement: Node development reference exists
The `tentacular-skill/references/node-development.md` file SHALL document TypeScript patterns for writing Tentacular nodes.

#### Scenario: Node function signature documented
- **WHEN** an agent reads `references/node-development.md`
- **THEN** the document SHALL describe the required node signature: a default-exported async function receiving `(ctx: Context, input: unknown)` and returning `Promise<unknown>`

#### Scenario: Context.fetch documented with auth injection
- **WHEN** an agent reads `references/node-development.md`
- **THEN** the document SHALL describe `ctx.fetch(service, path, init?)` including automatic URL construction (`https://api.<service>.com<path>`), auth header injection from secrets (`Authorization: Bearer` for `token`, `X-API-Key` for `api_key`), and how to use full URLs

#### Scenario: Context.log documented
- **WHEN** an agent reads `references/node-development.md`
- **THEN** the document SHALL describe the Logger interface with `info`, `warn`, `error`, and `debug` methods, noting that logs are prefixed with the node ID

#### Scenario: Context.config and Context.secrets documented
- **WHEN** an agent reads `references/node-development.md`
- **THEN** the document SHALL describe `ctx.config` as a `Record<string, unknown>` from workflow config and `ctx.secrets` as a `Record<string, Record<string, string>>` loaded from `.secrets.yaml` or K8s volume mount

#### Scenario: Data passing between nodes documented
- **WHEN** an agent reads `references/node-development.md`
- **THEN** the document SHALL describe how node outputs flow to downstream nodes via edges: single dependency passes output directly, multiple dependencies merge into a keyed object

#### Scenario: Complete node example included
- **WHEN** an agent reads `references/node-development.md`
- **THEN** the document SHALL include at least one complete node file example demonstrating Context API usage (fetch, log, config access)

### Requirement: Testing guide reference exists
The `tentacular-skill/references/testing-guide.md` file SHALL document how to write and run tests for Tentacular workflows.

#### Scenario: Fixture format documented
- **WHEN** an agent reads `references/testing-guide.md`
- **THEN** the document SHALL describe the JSON fixture format: `{ "input": <value>, "expected": <value> }` where `expected` is optional, stored at `tests/fixtures/<nodename>.json`

#### Scenario: Node-level testing documented
- **WHEN** an agent reads `references/testing-guide.md`
- **THEN** the document SHALL describe how to run node tests using `tntc test [dir][/<node>]` and how the test runner loads fixtures, imports the node, creates a mock context, and compares output to expected

#### Scenario: Pipeline testing documented
- **WHEN** an agent reads `references/testing-guide.md`
- **THEN** the document SHALL describe pipeline testing with `tntc test --pipeline`, explaining that it compiles the full DAG and executes all nodes in topological order

#### Scenario: Mock context documented
- **WHEN** an agent reads `references/testing-guide.md`
- **THEN** the document SHALL describe `createMockContext()` including mock fetch (returns `{ mock: true }` by default), log capture (`_logs` array), and `_setFetchResponse()` for custom mock responses

#### Scenario: Test fixture example included
- **WHEN** an agent reads `references/testing-guide.md`
- **THEN** the document SHALL include at least one complete test fixture JSON file example

### Requirement: Deployment guide reference exists
The `tentacular-skill/references/deployment-guide.md` file SHALL document the build, deploy, and cluster management workflow.

#### Scenario: Build command documented
- **WHEN** an agent reads `references/deployment-guide.md`
- **THEN** the document SHALL describe `tntc build [dir]` including image tag format (`<name>:<version>`), Dockerfile generation (distroless Deno base, engine copy, dependency caching), and `--tag` flag

#### Scenario: Deploy command documented
- **WHEN** an agent reads `references/deployment-guide.md`
- **THEN** the document SHALL describe `tntc deploy [dir]` including K8s manifest generation (Deployment with gVisor RuntimeClass, Service), `--namespace` flag, and `--registry` flag

#### Scenario: Cluster check documented
- **WHEN** an agent reads `references/deployment-guide.md`
- **THEN** the document SHALL describe `tntc cluster check` including preflight checks, `--fix` flag for auto-remediation, and `--namespace` flag

#### Scenario: Security model documented
- **WHEN** an agent reads `references/deployment-guide.md`
- **THEN** the document SHALL describe the Fortress security model: Deno permission flags (`--allow-net`, `--allow-read=/app`, `--allow-write=/tmp`), distroless container base image, gVisor RuntimeClass, and K8s Secret volume mounts for secrets

#### Scenario: Secrets management documented
- **WHEN** an agent reads `references/deployment-guide.md`
- **THEN** the document SHALL describe both local secrets (`.secrets.yaml` file) and production secrets (K8s Secret mounted at `/app/secrets`), including the `.secrets.yaml.example` convention
