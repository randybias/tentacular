# Testing Guide

How to write and run tests for Tentacular workflows.

## Fixture Format

Test fixtures are JSON files stored at `tests/fixtures/<nodename>.json` within the workflow directory.

```json
{
  "input": <value>,
  "expected": <value>
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `input` | any | Yes | The value passed as the `input` parameter to the node function |
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
3. Creates a mock context via `createMockContext()`.
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
| `ctx.fetch(service, path)` | Returns `{ mock: true, service, path }` as JSON by default |
| `ctx.log.*` | Captures all log calls to `ctx._logs` array |
| `ctx.config` | Empty object `{}` |
| `ctx.secrets` | Empty object `{}` |

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
