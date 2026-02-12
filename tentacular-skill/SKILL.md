# Tentacular

Tentacular is a workflow execution system with two components:

- **Go CLI** (`cmd/tntc/`, `pkg/`) -- manages the workflow lifecycle: scaffold, validate, dev, test, build, deploy.
- **Deno/TypeScript Engine** (`engine/`) -- executes workflows as DAGs. Compiles workflow.yaml into topologically sorted stages, loads TypeScript node modules, runs them with a Context providing fetch, logging, config, and secrets. Exposes HTTP triggers (`POST /run`, `GET /health`).

Workflows live in a directory containing a `workflow.yaml` and a `nodes/` directory of TypeScript files. Each node is a default-exported async function.

## CLI Quick Reference

| Command | Usage | Key Flags | Description |
|---------|-------|-----------|-------------|
| `init` | `tntc init <name>` | | Scaffold a new workflow directory with workflow.yaml, example node, test fixture, .secrets.yaml.example |
| `validate` | `tntc validate [dir]` | | Parse and validate workflow.yaml (name, version, triggers, nodes, edges, DAG acyclicity) |
| `dev` | `tntc dev [dir]` | `-p` port (default 8080) | Start Deno engine locally with hot-reload (`--watch`). POST /run triggers execution |
| `test` | `tntc test [dir][/<node>]` | `--pipeline` | Run node-level tests from fixtures, or full pipeline test with `--pipeline` |
| `build` | `tntc build [dir]` | `-t` tag, `-r` registry, `--push`, `--platform` | Generate Dockerfile (distroless Deno base), build container image via `docker build` |
| `deploy` | `tntc deploy [dir]` | `-n` namespace, `--image`, `--runtime-class` | Generate K8s manifests and apply to cluster. Namespace resolves: CLI > workflow.yaml > config > default |
| `configure` | `tntc configure` | `--registry`, `--namespace`, `--runtime-class`, `--project` | Set default config (user-level or project-level) |
| `secrets check` | `tntc secrets check [dir]` | | Check secrets provisioning against node requirements |
| `secrets init` | `tntc secrets init [dir]` | `--force` | Initialize .secrets.yaml from .secrets.yaml.example |
| `status` | `tntc status <name>` | `-n` namespace, `-o` json, `--detail` | Check deployment status in K8s; `--detail` shows pods, events, resources |
| `run` | `tntc run <name>` | `-n` namespace, `--timeout` | Trigger a deployed workflow and return JSON result |
| `logs` | `tntc logs <name>` | `-n` namespace, `-f`/`--follow`, `--tail` | View workflow pod logs; `-f` streams in real time |
| `list` | `tntc list` | `-n` namespace, `-o` json | List all deployed workflows with version, status, and age |
| `undeploy` | `tntc undeploy <name>` | `-n` namespace, `--yes` | Remove a deployed workflow (Service, Deployment, Secret, CronJobs). Note: ConfigMap `<name>-code` is not deleted. |
| `cluster check` | `tntc cluster check` | `--fix`, `-n` namespace | Preflight validation of cluster readiness; `--fix` auto-remediates |
| `visualize` | `tntc visualize [dir]` | | Generate Mermaid diagram of the workflow DAG |

Global flags: `-n`/`--namespace` (default "default"), `-r`/`--registry`, `-o`/`--output` (text\|json).

Namespace resolution order: CLI `-n` flag > `workflow.yaml deployment.namespace` > config file default > `default`.

Config files: `~/.tentacular/config.yaml` (user-level), `.tentacular/config.yaml` (project-level). Project overrides user.

## Node Contract

Every node is a TypeScript file with a single default export:

```typescript
import type { Context } from "tentacular";

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

## Trigger Types

| Type | Mechanism | Required Fields | K8s Resources | Status |
|------|-----------|----------------|---------------|--------|
| `manual` | HTTP POST `/run` | none | — | Implemented |
| `cron` | K8s CronJob → curl POST `/run` | `schedule`, optional `name` | CronJob | Implemented |
| `queue` | NATS subscription → execute | `subject` | — | Implemented |
| `webhook` | Future: gateway → NATS bridge | `path` | — | Roadmap |

### Trigger Name Field

Triggers can have an optional `name` field (must match `[a-z][a-z0-9_-]*`, unique within workflow). Named cron triggers POST `{"trigger": "<name>"}` to `/run`. Root nodes receive this as `input.trigger` to branch behavior.

### Cron Trigger Lifecycle

1. Define in workflow.yaml: `type: cron`, `schedule: "0 9 * * *"`, optional `name`
2. `tntc deploy` generates CronJob manifest(s) alongside Deployment and Service
3. CronJob naming: `{wf}-cron` (single) or `{wf}-cron-0`, `{wf}-cron-1` (multiple)
4. CronJob curls `http://{wf}.{ns}.svc.cluster.local:8080/run` with trigger payload
5. `tntc undeploy` deletes CronJobs by label selector (automatic cleanup)

### Queue Trigger (NATS)

Queue triggers subscribe to NATS subjects. The engine connects using config and secrets:

- **URL**: `config.nats_url` in workflow.yaml (e.g., `nats.ospo-dev.miralabs.dev:18453`)
- **Auth**: `secrets.nats.token` (token authentication)
- **TLS**: Uses system CA trust store (Let's Encrypt certs work automatically)

If either `nats_url` or `nats.token` is missing, the engine warns and skips NATS setup (graceful degradation).

Messages are parsed as JSON and passed as input to root nodes. If the NATS message has a reply subject, the workflow result is sent back (request-reply pattern).

## Config Block

The `config:` block is **open** — it accepts arbitrary keys alongside `timeout` and `retries`. Custom keys flow through to `ctx.config` in nodes. This is the standard mechanism for non-secret workflow configuration.

```yaml
config:
  timeout: 30s
  retries: 2
  nats_url: "nats.ospo-dev.miralabs.dev:18453"
  custom_setting: "value"
```

In Go, extra keys are stored in `WorkflowConfig.Extras` (via `yaml:",inline"`). Use `ToMap()` to get a flat merged map.

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
tntc configure --registry reg.io   # one-time setup (user or --project)
tntc init my-workflow              # scaffold directory
cd my-workflow
# edit nodes/*.ts and workflow.yaml
tntc validate                      # check spec validity
tntc secrets check                 # verify secrets provisioned
tntc secrets init                  # create .secrets.yaml from template
tntc dev                           # local dev server with hot-reload
tntc test                          # run node tests from fixtures
tntc test --pipeline               # run full DAG end-to-end
tntc build                         # build container image
tntc cluster check --fix           # verify K8s cluster readiness
tntc deploy                        # deploy (namespace from workflow.yaml)
tntc status my-workflow            # check deployment status
tntc list                          # list all deployed workflows + versions
tntc run my-workflow               # trigger workflow, get JSON result
tntc logs my-workflow              # view pod logs
tntc undeploy my-workflow          # remove from cluster
```

## References

For detailed documentation on specific topics:

- [Workflow Specification](references/workflow-spec.md) -- complete workflow.yaml format, all fields, trigger types, validation rules
- [Node Development](references/node-development.md) -- Context API details, data passing between nodes, patterns
- [Testing Guide](references/testing-guide.md) -- fixture format, mock context, node and pipeline testing
- [Deployment Guide](references/deployment-guide.md) -- build, deploy, cluster check, security model, secrets
