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

## Test Counts

| Suite | Tests | Coverage Areas |
|-------|-------|---------------|
| Go (`pkg/spec`) | 16 | Parser: valid spec, naming, cycles, edges, triggers, config |
| Go (`pkg/builder`) | 25 | K8s manifests: security, probes, RuntimeClass, CronJobs |
| Go (`pkg/cli`) | 12 | Secret provisioning: dir/YAML cascade, error handling |
| Go (`pkg/k8s`) | 3 | Preflight checks: JSON round-trip, warning omitempty |
| Deno (`compiler`) | 9 | DAG compilation: chains, fan-out, fan-in, cycles |
| Deno (`context`) | 12 | Context: fetch, auth injection, logging, config |
| Deno (`secrets`) | 6 | Secret loading: YAML, directory, cascade |
| Deno (`cascade`) | 7 | Cascade: precedence, merging, fallback |
| Deno (`executor`) | 7 | Execution: chains, parallel, retry, timeout |
| Deno (`nats`) | 6 | NATS: options validation, triggers |

See [architecture.md](architecture.md) for the full testing architecture including mock utilities and fixture patterns.
