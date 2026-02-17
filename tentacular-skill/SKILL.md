# Tentacular

Tentacular is a *secure* workflow build and execution
system designed for AI agents. It's purpose is to allow
an AI agent (you) to easily build repeatable and durable
workflows (pipelines). Arbitrary workflows can be built
dynamically by an agent, reused or replaced as necessary,
and deployed onto Kubernetes clusters to be triggered
manually or by various hooks (cron, webhook, etc).
Tentacular differs from statically built, systems such as
n8n, which has specific nodes in a graph structure, where
each node has significant functionality, much of which you
may not use. It also differs from monolithic agentic
systems and frameworks like langgraph/langchain and
Pydantic AI, in that all of the code in a workflow is
meant to be disposable and workflows are custom built to
need every time (or reused and iterated upon).

Every tentacular agentic workflow is deployed in a tightly
constrained sandbox, without a tool chain or other
ancillary systems. Only what is needed is deployed into a
production runtime ([gVisor](https://gvisor.dev/)). In addition, each
workflow's networking and other access requirements are
declared in the workflow contract, which drives automatic
generation of Kubernetes NetworkPolicy to lockdown the
workflow's production network access.

In tentacular, every node in the DAG is created at build
time by (you) the coding agent, tested end to end in a
development environment with real credentials, and then
shipped to production after it has been validated. The
nodes and graph are shipped as a set of Typescript code
that executes on a single pod in a Kubernetes cluster.

The sky is the limit on what you can build. From a simple
word counter workflow, to a workflow that makes multiple
API, LLM, and MCP server calls. There are no constraints
other than what you can imagine.

The key benefits of Tentacular are:

- Hardened execution environment for production workloads
  - Attack surface is absolute minimum as there is no
    non-essential code
  - Runs on [gVisor](https://gvisor.dev/) by default
- Agent-friendly
  - Build workflows using plain english,
    but in a repeatable and manageable manner
  - Full tentacular SKILL to help manage the workflow
    end-to-end
  - Testing is baked into the workflow development process
    prior to pushing to production

It is composed of two key components;

- **Go CLI** (`cmd/tntc/`, `pkg/`) -- manages the
  workflow lifecycle: scaffold, validate, dev, test,
  build, deploy.
- **Deno/TypeScript Engine** (`engine/`) -- executes
  workflows as DAGs. Compiles workflow.yaml into
  topologically sorted stages, loads TypeScript node
  modules, runs them with a Context providing dependency
  resolution, fetch, logging, config, and secrets.
  Exposes HTTP triggers (`POST /run`, `GET /health`).

Workflows live in a directory containing a `workflow.yaml`
and a `nodes/` directory of TypeScript files. Each node is
a default-exported async function.

For full CLI reference, read [references/cli.md](references/cli.md).

## Node Contract

Every node is a TypeScript file with a single default
export:

```typescript
import type { Context } from "tentacular";

export default async function run(
  ctx: Context,
  input: unknown
): Promise<unknown> {
  // input: output from upstream node(s) via edges
  // return value: passed to downstream node(s)
  ctx.log.info("processing");
  return { result: "done" };
}
```

### Context API

| Member | Type | Description |
|--------|------|-------------|
| `ctx.dependency(name)` | `(string) => DependencyConnection` | **Primary API** for external services. Returns connection metadata (host, port, protocol, authType, protocol-specific fields) and resolved secret value. HTTPS deps also get a `fetch(path, init?)` convenience method that builds the URL (no auth injection -- nodes handle auth explicitly). Throws if the dependency is not declared in the contract. |
| `ctx.log` | `Logger` | Structured logging with `info`, `warn`, `error`, `debug` methods. All output prefixed with `[nodeId]`. |
| `ctx.config` | `Record<string, unknown>` | Workflow-level config from `config:` in workflow.yaml. Use for business-logic parameters only (e.g., `target_repo`). |
| `ctx.fetch(service, path, init?)` | `(string, string, RequestInit?) => Promise<Response>` | **Legacy.** HTTP request with auto URL construction. Flagged as contract violation when a contract is present. Use `ctx.dependency()` instead. |
| `ctx.secrets` | `Record<string, Record<string, string>>` | **Legacy.** Direct secret access. Flagged as contract violation when a contract is present. Use `ctx.dependency()` instead. |

### Using ctx.dependency()

Nodes access external service connection info through
the contract dependency API:

```typescript
const pg = ctx.dependency("postgres");
// pg.protocol, pg.host, pg.port, pg.database, pg.user
// pg.authType -- "password", pg.secret -- resolved value

const gh = ctx.dependency("github-api");
// gh.fetch(path, init?) -- URL builder (no auth injection)
const resp = await gh.fetch!("/repos/org/repo", {
  headers: { "Authorization": `Bearer ${gh.secret}` },
});
```

**Auth is explicit.** `dep.fetch()` builds the URL
(`https://<host>:<port><path>`) but does not inject
auth headers. Nodes set auth headers themselves using
`dep.secret` and `dep.authType`.

In mock context (during `tntc test`), `ctx.dependency()`
returns mock values and records access for drift
detection.

**Migration note:** Replace `ctx.config` + `ctx.secrets`
assembly with `ctx.dependency()`. Connection metadata
and secret references belong in `contract.dependencies`.
The `config` section should contain only business-logic
parameters (e.g., `target_repo`, `sep_label`).

## Trigger Types

| Type | Mechanism | Required Fields | K8s Resources | Status |
|------|-----------|----------------|---------------|--------|
| `manual` | HTTP POST `/run` | none | -- | Implemented |
| `cron` | K8s CronJob -> curl POST `/run` | `schedule`, optional `name` | CronJob | Implemented |
| `queue` | NATS subscription -> execute | `subject` | -- | Implemented |
| `webhook` | Future: gateway -> NATS bridge | `path` | -- | Roadmap |

### Trigger Name Field

Triggers can have an optional `name` field (must match
`[a-z][a-z0-9_-]*`, unique within workflow). Named cron
triggers POST `{"trigger": "<name>"}` to `/run`. Root
nodes receive this as `input.trigger` to branch behavior.

### Cron Trigger Lifecycle

1. Define in workflow.yaml: `type: cron`,
   `schedule: "0 9 * * *"`, optional `name`
2. `tntc deploy` generates CronJob manifest(s) alongside
   Deployment and Service
3. CronJob naming: `{wf}-cron` (single) or
   `{wf}-cron-0`, `{wf}-cron-1` (multiple)
4. CronJob curls
   `http://{wf}.{ns}.svc.cluster.local:8080/run` with
   trigger payload
5. `tntc undeploy` deletes CronJobs by label selector
   (automatic cleanup)

### Queue Trigger (NATS)

Queue triggers subscribe to NATS subjects. Config:
`config.nats_url`, auth: `secrets.nats.token`. If
either is missing, the engine warns and skips NATS
(graceful degradation). Messages are parsed as JSON
and passed as input to root nodes.

## Contract Model

The `contract` section declares every external dependency
a workflow needs. Dependencies are the single primitive:
secrets, NetworkPolicy, connection config, and validation
are all derived from the dependency list. There is no
separate `secrets` or `networkPolicy` section to author.

For the complete contract specification, including
structure, dependency protocols, auth types, enforcement
modes, drift detection, dynamic-target dependencies,
label-scoped ingress, and NetworkPolicy overrides, read
[references/contract.md](references/contract.md).

## Config Block

The `config:` block is **open** -- it accepts arbitrary
keys alongside `timeout` and `retries`. Custom keys flow
through to `ctx.config` in nodes. This is the standard
mechanism for non-secret workflow configuration.

```yaml
config:
  timeout: 30s
  retries: 2
  nats_url: "nats.ospo-dev.miralabs.dev:18453"
  custom_setting: "value"
```

In Go, extra keys are stored in
`WorkflowConfig.Extras` (via `yaml:",inline"`). Use
`ToMap()` to get a flat merged map.

## Minimal workflow.yaml

```yaml
name: my-workflow
version: "1.0"
description: "A minimal workflow"

triggers:
  - type: manual

contract:
  version: "1"
  dependencies: {}

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
tntc configure --registry reg.io   # one-time setup
tntc init my-workflow              # scaffold directory
cd my-workflow
# edit nodes/*.ts and workflow.yaml
tntc validate                      # check spec validity
tntc secrets check                 # verify secrets
tntc secrets init                  # create .secrets.yaml
tntc dev                           # local dev server
tntc test                          # run node tests
tntc test --pipeline               # run full DAG e2e
tntc build                         # build container image
tntc cluster check --fix           # verify K8s cluster
tntc deploy                        # deploy to cluster
tntc status my-workflow            # check deploy status
tntc list                          # list all workflows
tntc run my-workflow               # trigger workflow
tntc logs my-workflow              # view pod logs
tntc undeploy my-workflow          # remove from cluster
```

## Deployment Flow

The recommended agentic deployment flow validates
workflows through six steps:

```
tntc validate -o json               # 1. Validate spec + contract
tntc visualize --rich --write       # 2. Persist contract artifacts
tntc test -o json                   # 3. Mock tests + drift detection
tntc test --live --env dev -o json  # 4. Live test against dev
tntc deploy -o json                 # 5. Deploy (auto-gates on live)
tntc run <name> -o json             # 6. Post-deploy verification
```

**Required checklist before build/deploy:**

- [ ] Contract section present in workflow.yaml
- [ ] `tntc validate` passes (contract + spec)
- [ ] `tntc visualize --rich --write` reviewed with user
- [ ] Dependency hosts and ports confirmed
- [ ] Secret refs match `.secrets.yaml` keys
- [ ] Derived NetworkPolicy matches expected access
- [ ] `tntc test` passes with zero drift
- [ ] `workflow-diagram.md` and `contract-summary.md` committed to repo

For step details, deploy gate behavior, structured
output format, and the pre-build review gate, read
[references/deployment-guide.md](references/deployment-guide.md).

## Environment Configuration

Named environments (`dev`, `staging`, `production`)
extend the config cascade with cluster-specific
settings. Define them in `~/.tentacular/config.yaml`
or `.tentacular/config.yaml`. Each environment can
specify `context`, `namespace`, `image`,
`runtime_class`, `config_overrides`, and
`secrets_source`. Resolution order: CLI flags >
environment config > project config > user config >
defaults. See
[references/deployment-guide.md](references/deployment-guide.md)
for full environment configuration details.

## Agent Workflow Guide

Agents must plan before coding. The planning loop
confirms user intent, authors the contract first,
derives secrets and network intent, pre-validates
the environment, and defines dev-to-prod promotion
gates. Do not start implementation until the plan
is explicitly confirmed by the user.

For the full agent workflow guide including planning
steps, E2E cycle, image tag cascade, deploy flags,
and common gotchas, read
[references/agent-workflow.md](references/agent-workflow.md).

## References

For detailed documentation on specific topics:

- [CLI Reference](references/cli.md)
  -- full command table, global flags, namespace
  resolution, config files, visualization reference
- [Contract Model](references/contract.md)
  -- contract structure, dependency protocols, auth
  types, enforcement, drift detection, dynamic targets,
  label-scoped ingress, NetworkPolicy overrides
- [Agent Workflow Guide](references/agent-workflow.md)
  -- planning steps, E2E cycle, image tag cascade,
  deploy flags, common gotchas
- [Workflow Specification](references/workflow-spec.md)
  -- complete workflow.yaml format, all fields, trigger
  types, validation rules
- [Node Development](references/node-development.md)
  -- Context API details, data passing between nodes,
  patterns
- [Testing Guide](references/testing-guide.md)
  -- fixture format, mock context, node and pipeline
  testing, live testing
- [Deployment Guide](references/deployment-guide.md)
  -- build, deploy, cluster check, security model,
  secrets, environment config, deploy gate
