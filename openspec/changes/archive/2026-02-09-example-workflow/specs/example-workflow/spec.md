## ADDED Requirements

### Requirement: Workflow YAML is valid v2 spec
The `examples/github-digest/workflow.yaml` SHALL be a valid Pipedreamer v2 workflow specification that passes `pipedreamer validate`.

#### Scenario: Validate passes
- **WHEN** `pipedreamer validate examples/github-digest` is executed
- **THEN** the command SHALL exit with code 0 and report no validation errors

#### Scenario: Workflow name and version
- **WHEN** the workflow.yaml is parsed
- **THEN** the `name` field SHALL be "github-digest" and the `version` field SHALL be a valid semver string

#### Scenario: Triggers defined
- **WHEN** the workflow.yaml is parsed
- **THEN** the `triggers` array SHALL contain at least a manual trigger and a cron trigger

#### Scenario: Three nodes declared
- **WHEN** the workflow.yaml is parsed
- **THEN** the `nodes` record SHALL contain exactly three entries: "fetch-repos", "summarize", and "notify"

#### Scenario: Edges form a linear DAG
- **WHEN** the workflow.yaml is parsed
- **THEN** the `edges` array SHALL contain exactly two edges: `{ from: "fetch-repos", to: "summarize" }` and `{ from: "summarize", to: "notify" }`

#### Scenario: Node paths reference existing files
- **WHEN** each node's `path` value is resolved relative to the workflow directory
- **THEN** the file SHALL exist at `nodes/fetch-repos.ts`, `nodes/summarize.ts`, and `nodes/notify.ts`

### Requirement: fetch-repos node fetches GitHub data
The `fetch-repos` node SHALL be a source node that uses `ctx.fetch` to retrieve repository data from the GitHub API and returns a structured list of repositories.

#### Scenario: Node exports default async function
- **WHEN** the `nodes/fetch-repos.ts` module is imported
- **THEN** it SHALL export a default async function with the signature `(ctx: Context, input: unknown) => Promise<unknown>`

#### Scenario: Node calls ctx.fetch for GitHub API
- **WHEN** the fetch-repos node executes
- **THEN** it SHALL call `ctx.fetch` with "github" as the service name and a path targeting the repos endpoint

#### Scenario: Node returns repository data
- **WHEN** the fetch-repos node executes successfully
- **THEN** it SHALL return an object containing an array of repository records with at minimum `name`, `description`, and `stars` fields

#### Scenario: Node declares network capability
- **WHEN** the fetch-repos node entry in workflow.yaml is inspected
- **THEN** it SHALL declare `capabilities: { net: "github.com" }`

#### Scenario: Node includes documentation comments
- **WHEN** the `nodes/fetch-repos.ts` source file is inspected
- **THEN** it SHALL contain JSDoc comments explaining the node's purpose, inputs, and outputs

### Requirement: summarize node transforms repo data into digest
The `summarize` node SHALL be a pure transform node that takes repository data as input and produces a formatted digest summary.

#### Scenario: Node exports default async function
- **WHEN** the `nodes/summarize.ts` module is imported
- **THEN** it SHALL export a default async function with the signature `(ctx: Context, input: unknown) => Promise<unknown>`

#### Scenario: Node produces digest summary
- **WHEN** the summarize node receives repo data as input
- **THEN** it SHALL return an object containing a `title`, `summary` text, and `repoCount` number

#### Scenario: Node does not call ctx.fetch
- **WHEN** the summarize node executes
- **THEN** it SHALL NOT call `ctx.fetch` because it is a pure transform with no external dependencies

#### Scenario: Node includes documentation comments
- **WHEN** the `nodes/summarize.ts` source file is inspected
- **THEN** it SHALL contain JSDoc comments explaining the node's purpose, inputs, and outputs

### Requirement: notify node sends digest to webhook
The `notify` node SHALL be a sink node that uses `ctx.fetch` to send the digest summary to a webhook endpoint.

#### Scenario: Node exports default async function
- **WHEN** the `nodes/notify.ts` module is imported
- **THEN** it SHALL export a default async function with the signature `(ctx: Context, input: unknown) => Promise<unknown>`

#### Scenario: Node calls ctx.fetch for webhook delivery
- **WHEN** the notify node executes
- **THEN** it SHALL call `ctx.fetch` with a webhook service name and POST the digest payload

#### Scenario: Node declares network capability
- **WHEN** the notify node entry in workflow.yaml is inspected
- **THEN** it SHALL declare `capabilities: { net: "slack.com" }` or equivalent webhook destination

#### Scenario: Node includes documentation comments
- **WHEN** the `nodes/notify.ts` source file is inspected
- **THEN** it SHALL contain JSDoc comments explaining the node's purpose, inputs, and outputs

### Requirement: Test fixtures enable node-level testing
The `examples/github-digest/tests/fixtures/` directory SHALL contain JSON fixture files for testing individual nodes with `pipedreamer test`.

#### Scenario: fetch-repos fixture exists with correct structure
- **WHEN** `tests/fixtures/fetch-repos.json` is loaded
- **THEN** it SHALL contain a JSON object with an `input` field and an `expected` field matching the TestFixture format

#### Scenario: summarize fixture exists with correct structure
- **WHEN** `tests/fixtures/summarize.json` is loaded
- **THEN** it SHALL contain a JSON object with an `input` field providing repository data and an `expected` field with the expected digest summary output

#### Scenario: Fixtures match node input/output contracts
- **WHEN** the fetch-repos fixture's `expected` output is compared to the summarize fixture's `input`
- **THEN** the data shapes SHALL be compatible, demonstrating that data flows correctly through the DAG

### Requirement: pipedreamer test passes with fixtures
The `pipedreamer test` command SHALL successfully execute node-level tests against the github-digest example using the provided fixtures and mock context.

#### Scenario: Node tests pass
- **WHEN** `pipedreamer test examples/github-digest` is executed
- **THEN** the command SHALL exit with code 0 and report passing tests for nodes that have fixtures

#### Scenario: Test output shows timing
- **WHEN** `pipedreamer test examples/github-digest` is executed
- **THEN** the output SHALL include per-node test timing in milliseconds and a pass/fail summary

### Requirement: pipedreamer dev starts dev server
The `pipedreamer dev` command SHALL successfully start the development server for the github-digest example workflow.

#### Scenario: Dev server starts
- **WHEN** `pipedreamer dev examples/github-digest` is executed
- **THEN** the command SHALL start the Deno engine with `--watch` flag and the workflow loaded, listening on the default port
