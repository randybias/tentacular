# Pipedreamer v2

Pipedreamer is a workflow execution system with two components:

- **Go CLI** (`cmd/pipedreamer/`, `pkg/`) -- manages the workflow lifecycle: scaffold, validate, dev, test, build, deploy.
- **Deno/TypeScript Engine** (`engine/`) -- executes workflows as DAGs. Compiles workflow.yaml into topologically sorted stages, loads TypeScript node modules, runs them with a Context providing fetch, logging, config, and secrets. Exposes HTTP triggers (`POST /run`, `GET /health`).

Workflows live in a directory containing a `workflow.yaml` and a `nodes/` directory of TypeScript files. Each node is a default-exported async function.

## CLI Quick Reference

| Command | Usage | Key Flags | Description |
|---------|-------|-----------|-------------|
| `init` | `pipedreamer init <name>` | | Scaffold a new workflow directory with workflow.yaml, example node, test fixture, .secrets.yaml.example |
| `validate` | `pipedreamer validate [dir]` | `-v` verbose | Parse and validate workflow.yaml (name, version, triggers, nodes, edges, DAG acyclicity) |
| `dev` | `pipedreamer dev [dir]` | `-p` port (default 8080) | Start Deno engine locally with hot-reload (`--watch`). POST /run triggers execution |
| `test` | `pipedreamer test [dir][/<node>]` | `--pipeline` | Run node-level tests from fixtures, or full pipeline test with `--pipeline` |
| `build` | `pipedreamer build [dir]` | `-t` tag | Generate Dockerfile (distroless Deno base), build container image via `docker build` |
| `deploy` | `pipedreamer deploy [dir]` | `-n` namespace, `-r` registry | Generate K8s manifests (Deployment with gVisor, Service) and apply to cluster |
| `status` | `pipedreamer status <name>` | `-n` namespace, `-o` json | Check deployment status in K8s |
| `cluster check` | `pipedreamer cluster check` | `--fix`, `-n` namespace | Preflight validation of cluster readiness; `--fix` auto-remediates |
| `visualize` | `pipedreamer visualize [dir]` | | Generate Mermaid diagram of the workflow DAG |

Global flags: `-n`/`--namespace` (default "default"), `-r`/`--registry`, `-o`/`--output` (text\|json), `-v`/`--verbose`.

## Node Contract

Every node is a TypeScript file with a single default export:

```typescript
import type { Context } from "pipedreamer";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  // input: output from upstream node(s) via edges
  // return value: passed to downstream node(s)
  ctx.log.info("processing");
  return { result: "done" };
}
```

### Context API

| Member | Type | Description |
|--------|------|-------------|
| `ctx.fetch(service, path, init?)` | `(string, string, RequestInit?) => Promise<Response>` | HTTP request with auto URL construction (`https://api.<service>.com<path>`) and auth injection from secrets (Bearer token or X-API-Key). Use full URL in `path` to bypass service resolution. |
| `ctx.log` | `Logger` | Structured logging with `info`, `warn`, `error`, `debug` methods. All output prefixed with `[nodeId]`. |
| `ctx.config` | `Record<string, unknown>` | Workflow-level config from `config:` in workflow.yaml. |
| `ctx.secrets` | `Record<string, Record<string, string>>` | Secrets loaded from `.secrets.yaml` (local) or K8s Secret volume at `/app/secrets` (production). Keyed by service name. |

## Minimal workflow.yaml

```yaml
name: my-workflow
version: "1.0"
description: "A minimal workflow"

triggers:
  - type: manual

nodes:
  hello:
    path: ./nodes/hello.ts

edges: []

config:
  timeout: 30s
  retries: 0
```

## Common Workflow

```
pipedreamer init my-workflow     # scaffold directory
cd my-workflow
# edit nodes/*.ts and workflow.yaml
pipedreamer validate             # check spec validity
pipedreamer dev                  # local dev server with hot-reload
pipedreamer test                 # run node tests from fixtures
pipedreamer test --pipeline      # run full DAG end-to-end
pipedreamer build                # build container image
pipedreamer cluster check --fix  # verify K8s cluster readiness
pipedreamer deploy               # deploy to Kubernetes
pipedreamer status my-workflow   # check deployment status
```

## References

For detailed documentation on specific topics:

- [Workflow Specification](references/workflow-spec.md) -- complete workflow.yaml format, all fields, trigger types, validation rules
- [Node Development](references/node-development.md) -- Context API details, data passing between nodes, patterns
- [Testing Guide](references/testing-guide.md) -- fixture format, mock context, node and pipeline testing
- [Deployment Guide](references/deployment-guide.md) -- build, deploy, cluster check, security model, secrets
