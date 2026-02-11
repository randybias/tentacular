## 1. Workflow Spec

- [x] 1.1 Create `examples/github-digest/workflow.yaml` with name "github-digest", semver version, manual + cron triggers, 3 nodes (fetch-repos, summarize, notify) with paths and capability declarations, 2 edges forming a linear DAG, and workflow config (timeout, retries)
- [x] 1.2 Verify the workflow.yaml is valid by running `tntc validate examples/github-digest`

## 2. Node Implementations

- [x] 2.1 Create `examples/github-digest/nodes/fetch-repos.ts` — source node with JSDoc comments, typed interfaces (RepoData), default async function that calls `ctx.fetch("github", ...)` and returns an array of repo records with name, description, and stars fields
- [x] 2.2 Create `examples/github-digest/nodes/summarize.ts` — pure transform node with JSDoc comments, typed interfaces (DigestSummary), default async function that takes repo data input and returns an object with title, summary text, and repoCount
- [x] 2.3 Create `examples/github-digest/nodes/notify.ts` — sink node with JSDoc comments, default async function that calls `ctx.fetch` with a webhook service to POST the digest payload

## 3. Test Fixtures

- [x] 3.1 Create `examples/github-digest/tests/fixtures/fetch-repos.json` with `{ "input": ..., "expected": ... }` structure where expected output contains an array of repo records matching the fetch-repos node's return shape
- [x] 3.2 Create `examples/github-digest/tests/fixtures/summarize.json` with `{ "input": ..., "expected": ... }` structure where input matches the fetch-repos expected output shape and expected matches the summarize node's return shape (title, summary, repoCount)
- [x] 3.3 Verify fixture data flows are compatible: fetch-repos expected output shape matches summarize input shape

## 4. Verification

- [x] 4.1 Run `tntc validate examples/github-digest` and confirm exit code 0
- [x] 4.2 Run `tntc test examples/github-digest` and confirm all fixture-based node tests pass with timing output
- [x] 4.3 Run `tntc dev examples/github-digest` and confirm the dev server starts successfully with the workflow loaded
- [x] 4.4 Review all three node files to confirm JSDoc documentation comments are present covering purpose, inputs, and outputs
