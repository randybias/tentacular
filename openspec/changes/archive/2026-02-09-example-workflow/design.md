## Context

Pipedreamer v2 has a working engine (DAG compiler, SimpleExecutor, context system, dynamic node loader), a Go CLI with validate/dev/test commands, and a testing framework with fixture loading and mock context. However, the `examples/` directory is empty. There is no reference workflow that ties these components together, and no integration-level proof that the full stack works end-to-end.

The `pipedreamer init` command scaffolds a minimal single-node workflow, but a real-world example with multiple nodes, edges, typed data passing, external API calls, and test fixtures is needed to demonstrate the platform's value and validate the toolchain.

## Goals / Non-Goals

**Goals:**
- Create a github-digest example that demonstrates all three node archetypes: source (external API), transform (pure data), and sink (outbound delivery)
- Provide complete test fixtures that work with `pipedreamer test`
- Ensure `pipedreamer validate`, `pipedreamer test`, and `pipedreamer dev` all succeed against this example
- Include documentation comments in every TypeScript file that teach node authoring patterns
- Establish a reference structure that `pipedreamer init` could eventually generate as a "full example" template

**Non-Goals:**
- Actually calling the GitHub API or Slack webhooks during tests (fixtures and mocks handle this)
- Adding new engine capabilities or CLI features
- Creating multiple examples (one comprehensive example is sufficient for this change)
- Writing a tutorial document (the code and comments are the documentation)

## Decisions

### Decision 1: Linear DAG topology (fetch-repos -> summarize -> notify)
**Choice:** Use a simple linear 3-node pipeline rather than a diamond or fan-out DAG.
**Rationale:** A linear DAG is the most common real-world pattern and the easiest to understand. It demonstrates data flowing through all three node archetypes (source -> transform -> sink) without introducing parallel execution complexity. The DAG engine supports parallel stages, but that is better demonstrated in a future advanced example. Alternative considered: a fan-out where summarize produces both a Slack notification and an email — rejected because it adds complexity without teaching new concepts for a first example.

### Decision 2: Typed input/output interfaces in each node
**Choice:** Define explicit TypeScript interfaces for each node's input and output shapes (e.g., `RepoData`, `DigestSummary`) in the same file as the node function.
**Rationale:** This teaches developers to think about the data contract between nodes. Co-locating the interfaces with the node code keeps the example self-contained. Alternative considered: a shared `types.ts` file — rejected because it creates coupling between nodes and is not the recommended pattern for simple workflows.

### Decision 3: ctx.fetch for external calls with capability declarations
**Choice:** The `fetch-repos` node uses `ctx.fetch("github", "/users/{user}/repos")` and declares `capabilities: { net: "github.com" }` in workflow.yaml. The `notify` node uses `ctx.fetch("slack", "/webhook")` with `capabilities: { net: "slack.com" }`.
**Rationale:** This demonstrates the Fortress security model where each node declares its network capabilities, and `ctx.fetch` routes through the gateway. It shows how capability declarations in workflow.yaml correspond to runtime fetch calls.

### Decision 4: Two fixture files (fetch-repos and summarize), no fixture for notify
**Choice:** Provide test fixtures for `fetch-repos` (mocked GitHub API response) and `summarize` (mocked repo data input with expected digest output). No fixture for `notify` since it is a sink with no meaningful output to assert.
**Rationale:** The testing framework loads fixtures from `tests/fixtures/<nodename>.json`. Source and transform nodes have clear input/output contracts worth testing. Sink nodes are best tested by verifying they call `ctx.fetch` with the right arguments, which requires the mock context approach rather than fixture-based testing. Alternative considered: a notify fixture with an empty expected output — rejected because it adds a file with no testing value.

### Decision 5: Manual and cron triggers
**Choice:** The workflow.yaml declares two triggers: `{ type: manual }` and `{ type: cron, schedule: "0 9 * * 1" }` (every Monday at 9am).
**Rationale:** Demonstrates that workflows can have multiple triggers. Manual is useful for development (`pipedreamer dev`), cron is the realistic production trigger for a weekly digest. Webhook trigger is omitted because it requires additional path configuration that is not relevant to this example's focus.

## Risks / Trade-offs

- **GitHub API structure may change** -> The fixture data uses a simplified subset of the real GitHub API response shape. This is fine because the nodes never actually call the real API; fixtures and mocks provide the data. If someone tries to run the example against real GitHub, the node code documents the expected response shape.
- **Example may drift from engine changes** -> If engine types or context APIs change in future changes, the example may break. Mitigated by the verification tasks: `pipedreamer validate` and `pipedreamer test` will catch drift immediately.
- **Single example may be insufficient** -> One example cannot demonstrate every feature (parallel stages, webhook triggers, secrets, error recovery). This is acceptable; the github-digest covers the most common patterns. Additional examples can be added in future changes.
