# Testing Guide

How to write and run tests for Tentacular workflows.

## Fixture Format

Test fixtures are JSON files stored at `tests/fixtures/<nodename>.json` within the workflow directory.

```json
{
  "input": <value>,
  "config": { ... },
  "secrets": { ... },
  "expected": <value>
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `input` | any | Yes | The value passed as the `input` parameter to the node function |
| `config` | `Record<string, unknown>` | No | Passed to `createMockContext()` as `ctx.config` (default: `{}`) |
| `secrets` | `Record<string, Record<string, string>>` | No | Passed to `createMockContext()` as `ctx.secrets` (default: `{}`) |
| `contract` | `{ dependencies: {...} }` | No | Contract spec for `ctx.dependency()` resolution. When omitted, the test runner auto-loads from `workflow.yaml`. |
| `expected` | any | No | If present, the node's return value is compared against this (JSON deep equality) |

When `expected` is omitted, the test passes as long as the node executes without throwing an error.

Multiple fixtures per node are supported. The runner finds all `.json` files in `tests/fixtures/` whose filename starts with the node name:

```
tests/fixtures/
  fetch-data.json           # matches node "fetch-data"
  fetch-data-empty.json     # also matches node "fetch-data"
  transform.json            # matches node "transform"
```

## Node-Level Testing

Run tests for all nodes:

```bash
tntc test [dir]
```

Run tests for a specific node:

```bash
tntc test [dir]/<node>
```

Examples:

```bash
tntc test                    # test all nodes in current directory
tntc test ./my-workflow      # test all nodes in my-workflow/
tntc test ./my-workflow/fetch-data  # test only the fetch-data node
```

The test runner for each node:
1. Finds fixture files matching the node name in `tests/fixtures/`.
2. Loads the node module from the path specified in workflow.yaml.
3. Creates a mock context via `createMockContext()` with fixture `config` and `secrets` (if provided).
4. Calls the node function with the mock context and fixture input.
5. If `expected` is defined, compares the output via JSON serialization equality.
6. Reports pass/fail with execution time.

### Test Output

```
--- Test Results ---
  ✓ fetch-data: fetch-data.json (12ms)
  ✓ transform: transform.json (3ms)
  ✗ notify: notify.json (5ms)
    Expected: {"sent":true}
    Got: {"sent":false}

2/3 tests passed
```

The CLI exits with code 1 if any test fails.

### Drift Detection

After running node tests, `tntc test` performs contract
drift detection. The mock context records all access
patterns during test execution and compares them against
the contract declaration.

**Violation types:**

| Type | Meaning |
|------|---------|
| `direct-fetch` | Code uses `ctx.fetch()` instead of `ctx.dependency().fetch()` |
| `direct-secrets` | Code reads `ctx.secrets` directly instead of `ctx.dependency().secret` |
| `undeclared-dependency` | Code calls `ctx.dependency(name)` for a dep not in the contract |
| `dead-declaration` | Contract declares a dep that code never accesses |

In strict mode (default), any violation fails the test.
Use `--warn` to downgrade violations to warnings:

```bash
tntc test                     # strict: violations fail
tntc test --warn              # audit: violations warn only
```

The drift report is appended after test results:

```
=== Contract Drift Report ===

VIOLATIONS:
  [direct-fetch] Direct ctx.fetch("github", "/repos/test") bypasses contract
     Suggestion: Use ctx.dependency("github").fetch("/repos/test") instead

SUMMARY:
  Dependencies accessed: 2
  Direct fetch() calls: 1
  Direct secrets access: 0
  Dead declarations: 0
  Undeclared dependencies: 0
  Has violations: YES
```

With `-o json`, the drift report is included in the
test output envelope.

## Pipeline Testing

Run the full DAG end-to-end:

```bash
tntc test --pipeline
tntc test ./my-workflow --pipeline
```

Pipeline testing:
1. Parses workflow.yaml and compiles the DAG.
2. Loads all node modules.
3. Executes nodes in topological order (stages run in parallel via `Promise.all`).
4. Reports overall success/failure with any node errors.

Pipeline tests use mock contexts for individual nodes but execute the full data flow through all edges and stages.

## Mock Context

The testing framework provides `createMockContext()` for isolated node testing.

### createMockContext()

```typescript
import { createMockContext } from "tentacular/testing/mocks";

const ctx = createMockContext();
```

The mock context provides:

| Feature | Behavior |
|---------|----------|
| `ctx.dependency(name)` | Returns mock `DependencyConnection` with contract metadata and mock secret values. Records access for drift detection. HTTPS deps get a mock `fetch()` that returns `{ mock: true, dependency, path }`. |
| `ctx.fetch(service, path)` | **Legacy.** Returns `{ mock: true, service, path }` as JSON by default. Flagged as contract violation when contract is present. |
| `ctx.log.*` | Captures all log calls to `ctx._logs` array |
| `ctx.config` | Empty object `{}` |
| `ctx.secrets` | **Legacy.** Empty object `{}`. Direct access flagged as contract violation when contract is present. |

### Overrides

Pass partial overrides to customize the mock:

```typescript
const ctx = createMockContext({
  config: { repo: "owner/repo" },
  secrets: { github: { token: "test-token" } },
});
```

### Capturing Logs

All log calls are recorded in the `_logs` array:

```typescript
const ctx = createMockContext();
await myNode(ctx, {});

// Inspect captured logs
console.log(ctx._logs);
// [
//   { level: "info", msg: "processing", args: [] },
//   { level: "warn", msg: "slow response", args: [{ ms: 500 }] }
// ]
```

### Custom Fetch Responses

Use `_setFetchResponse()` to register mock responses for specific service:path combinations:

```typescript
const ctx = createMockContext();

// Register a mock response for github:/repos/owner/repo/issues
ctx._setFetchResponse(
  "github",
  "/repos/owner/repo/issues",
  new Response(JSON.stringify([{ number: 1, title: "Bug" }]), {
    headers: { "content-type": "application/json" },
  })
);

// Now ctx.fetch("github", "/repos/owner/repo/issues") returns the mock response
```

### mockFetchResponse Helper

Convenience function for creating mock Response objects:

```typescript
import { mockFetchResponse } from "tentacular/testing/mocks";

const response = mockFetchResponse({ items: [1, 2, 3] });       // 200 OK
const errorResponse = mockFetchResponse({ error: "not found" }, 404);  // 404

ctx._setFetchResponse("github", "/user", response);
```

## Fixture with Config and Secrets

For nodes that read `ctx.config` or `ctx.secrets`, include these in the fixture:

`tests/fixtures/notify-slack.json`:

```json
{
  "input": { "alert": true, "summary": "Service down" },
  "secrets": { "slack": { "webhook_url": "https://hooks.slack.com/test" } },
  "expected": { "delivered": false, "status": 0 }
}
```

The test runner passes `config` and `secrets` to `createMockContext()`, so the node can access `ctx.secrets.slack.webhook_url` and take the appropriate code path instead of the "no secrets" early-exit path.

## Complete Fixture Example

Workflow directory structure:

```
my-workflow/
  workflow.yaml
  nodes/
    fetch-data.ts
    transform.ts
  tests/
    fixtures/
      fetch-data.json
      fetch-data-empty.json
      transform.json
```

`tests/fixtures/fetch-data.json`:

```json
{
  "input": {},
  "expected": {
    "items": [1, 2, 3],
    "count": 3
  }
}
```

`tests/fixtures/fetch-data-empty.json`:

```json
{
  "input": { "filter": "none" },
  "expected": {
    "items": [],
    "count": 0
  }
}
```

`tests/fixtures/transform.json` (no expected -- just validates no error):

```json
{
  "input": {
    "items": [1, 2, 3],
    "count": 3
  }
}
```

Run all tests:

```bash
$ tntc test
--- Test Results ---
  ✓ fetch-data: fetch-data.json (8ms)
  ✓ fetch-data: fetch-data-empty.json (2ms)
  ✓ transform: transform.json (1ms)

3/3 tests passed
```

## Graceful Degradation Pattern

Nodes that depend on external services (APIs, databases, etc.) should handle missing credentials in the mock context without throwing errors. This enables meaningful testing even when real credentials are not available.

### Pattern

Check for required credentials before making external calls. When credentials are absent, return a safe default or skip the external call:

```typescript
export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const gh = ctx.dependency("github-api");
  if (!gh.secret) {
    ctx.log.warn("no github credentials, returning empty result");
    return { issues: [], count: 0, source: "mock" };
  }

  // Real logic with external calls
  const res = await gh.fetch!("/repos/owner/repo/issues", {
    headers: { "Authorization": `Bearer ${gh.secret}` },
  });
  const issues = await res.json();
  return { issues, count: issues.length, source: "live" };
}
```

### Testing with and without credentials

Use separate fixtures to test both paths. When using `ctx.dependency()`, the fixture must include a `contract` block so the mock context can resolve dependencies:

`tests/fixtures/fetch-issues.json` (no secrets -- tests graceful degradation):

```json
{
  "input": {},
  "contract": {
    "dependencies": {
      "github-api": {
        "protocol": "https",
        "host": "api.github.com",
        "auth": { "type": "bearer-token", "secret": "github.token" }
      }
    }
  },
  "expected": { "issues": [], "count": 0, "source": "mock" }
}
```

`tests/fixtures/fetch-issues-live.json` (with secrets -- tests real logic):

```json
{
  "input": {},
  "contract": {
    "dependencies": {
      "github-api": {
        "protocol": "https",
        "host": "api.github.com",
        "auth": { "type": "bearer-token", "secret": "github.token" }
      }
    }
  },
  "secrets": { "github": { "token": "test-token" } },
  "expected": null
}
```

The first fixture validates the no-credentials path returns a safe default. The second fixture provides secrets so the node takes the real code path (with `expected: null` or omitted, the test passes as long as no error is thrown).

This pattern is especially important for nodes using Postgres, external APIs, or cloud storage, since the default mock context has empty secrets.

## Live Testing

Live testing deploys a workflow to a real Kubernetes cluster, triggers it, validates the result, and cleans up. This validates the full deployment pipeline including container builds, K8s manifests, secrets mounting, and real network calls.

### Prerequisites

A named environment must be configured. See the [Deployment Guide](deployment-guide.md) for environment configuration details.

Minimal config (`~/.tentacular/config.yaml` or `.tentacular/config.yaml`):

```yaml
environments:
  dev:
    context: kind-dev
    namespace: dev-workflows
```

### Running Live Tests

```bash
tntc test --live                       # live test using "dev" environment (default)
tntc test --live --env staging         # live test using "staging" environment
tntc test --live --keep                # skip cleanup (leave deployed for debugging)
tntc test --live --timeout 180s        # override default 120s timeout
tntc test --live -o json              # structured JSON output
```

### Live Test Flow

The live test executes these steps in order:

1. **Load environment config** -- reads the named environment from the config cascade, resolving context, namespace, image, and runtime class.
2. **Deploy** -- calls the shared `deployWorkflow()` function which handles kind cluster detection (adjusting RuntimeClass and ImagePullPolicy), manifest generation, preflight checks, and applying manifests to the environment namespace. If the environment specifies a kubeconfig context, it connects to that cluster.
3. **Wait for Ready** -- polls the Deployment until `ReadyReplicas == Replicas` or the timeout expires.
4. **Trigger workflow** -- sends a `POST /run` to the deployed workflow via a temporary curl pod and captures the execution result.
5. **Parse and validate** -- parses the JSON execution result and checks the `success` field.
6. **Cleanup** -- removes the deployed workflow from the environment namespace (skipped with `--keep`).
7. **Emit result** -- outputs structured test result (text or JSON based on `-o` flag).

### Live Test Output

Text output:

```
Live test: environment=dev, namespace=dev-workflows
Deploying my-workflow to namespace dev-workflows...
  Preflight checks passed
  Triggered rollout restart
Deployed my-workflow to dev-workflows
Waiting for my-workflow to become ready (timeout: 120s)...
  Deployment ready
Running workflow my-workflow...
[PASS] workflow completed successfully
  (8510ms)
Cleaning up my-workflow from dev-workflows...
  deleted Deployment/my-workflow
  deleted Service/my-workflow
  deleted ConfigMap/my-workflow-code
```

JSON output (with `-o json`):

```json
{
  "version": "1",
  "command": "test",
  "status": "pass",
  "summary": "workflow completed successfully",
  "hints": [],
  "execution": {
    "success": true,
    "outputs": { "hello": { "result": "done" } },
    "timing": { "totalMs": 1800 }
  },
  "timing": {
    "startedAt": "2026-02-16T09:00:00Z",
    "durationMs": 8510
  }
}
```

When the live test fails:

```json
{
  "version": "1",
  "command": "test",
  "status": "fail",
  "summary": "workflow returned success=false",
  "hints": ["check deployment logs with: tntc logs <workflow-name>"],
  "execution": {
    "success": false,
    "errors": ["node fetch-data threw: connection refused"]
  },
  "timing": {
    "startedAt": "2026-02-16T09:00:00Z",
    "durationMs": 12200
  }
}
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--live` | false | Run a live cluster test instead of mock tests |
| `--env` | `dev` | Named environment to test against (must be defined in config) |
| `--keep` | false | Do not clean up the deployed workflow after testing |
| `--timeout` | `120s` | Maximum time to wait for deployment readiness |
| `--warn` | false | Downgrade contract violations to warnings (audit mode) |
| `-o json` | text | Emit structured JSON output |

### Combining Mock and Live Tests

Mock tests and live tests serve different purposes and should both be part of the workflow development cycle:

| Aspect | Mock (`tntc test`) | Live (`tntc test --live`) |
|--------|-------------------|--------------------------|
| Speed | Milliseconds | Seconds to minutes |
| Dependencies | None | Kubernetes cluster |
| Credentials | Not needed | Real credentials required |
| Scope | Node logic | Full deployment pipeline |
| When to use | During development | Before deploying to production |

A typical workflow:

```bash
tntc test                           # fast feedback during development
tntc test --pipeline                # validate full DAG data flow
tntc test --live --env dev          # validate real deployment before promoting
tntc deploy -n production           # deploy auto-gates on live test
```
