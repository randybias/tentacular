# Testing

## Go Tests

```bash
go test ./pkg/...
```

Covers spec parsing, K8s manifest generation, secret provisioning, and preflight checks.

## Deno Engine Tests

```bash
cd engine && deno test --allow-read --allow-write=/tmp --allow-net --allow-env
```

Covers DAG compilation, context/fetch/auth injection, secrets cascade, and executor behavior (retry, timeout, parallel stages).

## Workflow Tests

```bash
tntc test                      # all node fixtures
tntc test my-workflow/fetch    # single node
tntc test --pipeline           # full DAG end-to-end
```

## Fixture Config and Secrets

Fixtures can include optional `config` and `secrets` fields to test nodes that read `ctx.config` or `ctx.secrets`:

```json
{
  "input": { "alert": true },
  "config": { "endpoints": ["https://example.com"] },
  "secrets": { "slack": { "webhook_url": "https://hooks.slack.com/test" } },
  "expected": { "delivered": false }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `input` | any | Yes | Value passed as `input` to the node function |
| `config` | `Record<string, unknown>` | No | Passed to `createMockContext()` as `ctx.config` |
| `secrets` | `Record<string, Record<string, string>>` | No | Passed to `createMockContext()` as `ctx.secrets` |
| `expected` | any | No | Expected return value (JSON deep equality) |

When `config` or `secrets` are omitted, the mock context uses empty objects (same as before).

## Test Counts

| Suite | Tests | Coverage Areas |
|-------|-------|---------------|
| Go (`pkg/spec`) | 17 | Parser: valid spec, naming, cycles, edges, triggers, config, deployment namespace |
| Go (`pkg/builder`) | 38 | K8s manifests: security, probes, RuntimeClass, CronJobs, imagePullPolicy, version labels, ConfigMap |
| Go (`pkg/cli`) | 42 | Secret provisioning, nested YAML, shared secrets, config loading/merging, secrets check/init |
| Go (`pkg/k8s`) | 3 | Preflight checks: JSON round-trip, warning omitempty |
| Deno (`compiler`) | 9 | DAG compilation: chains, fan-out, fan-in, cycles |
| Deno (`context`) | 12 | Context: fetch, auth injection, logging, config |
| Deno (`secrets`) | 6 | Secret loading: YAML, directory, cascade |
| Deno (`cascade`) | 7 | Cascade: precedence, merging, fallback |
| Deno (`executor`) | 7 | Execution: chains, parallel, retry, timeout |
| Deno (`nats`) | 7 | NATS: options validation, triggers |

See [architecture.md](architecture.md) for the full testing architecture including mock utilities and fixture patterns.
