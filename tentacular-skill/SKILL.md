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
specified in the workflow definition, which allows runtime
configuration of network policies to lockdown the
workflow's production access (WORK IN PROGRESS).

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
  modules, runs them with a Context providing fetch,
  logging, config, and secrets. Exposes HTTP triggers
  (`POST /run`, `GET /health`).

Workflows live in a directory containing a `workflow.yaml`
and a `nodes/` directory of TypeScript files. Each node is
a default-exported async function.

## CLI Quick Reference

| Command | Usage | Key Flags | Description |
|---------|-------|-----------|-------------|
| `init` | `tntc init <name>` | | Scaffold a new workflow directory with workflow.yaml, example node, test fixture, .secrets.yaml.example |
| `validate` | `tntc validate [dir]` | | Parse and validate workflow.yaml (name, version, triggers, nodes, edges, DAG acyclicity) |
| `dev` | `tntc dev [dir]` | `-p` port (default 8080) | Start Deno engine locally with hot-reload (`--watch`). POST /run triggers execution |
| `test` | `tntc test [dir][/<node>]` | `--pipeline`, `--live`, `--env`, `--keep`, `--timeout` | Run node-level tests from fixtures, full pipeline test with `--pipeline`, or live cluster test with `--live` |
| `build` | `tntc build [dir]` | `-t` tag, `-r` registry, `--push`, `--platform` | Generate Dockerfile (distroless Deno base), build container image via `docker build` |
| `deploy` | `tntc deploy [dir]` | `-n` namespace, `--env`, `--image`, `--runtime-class`, `--force`, `--verify` | Generate K8s manifests and apply to cluster. `--env` targets a named environment (context, namespace, image). Auto-gates on live test if dev env configured; `--force` skips. Namespace resolves: CLI > env > workflow.yaml > config > default |
| `configure` | `tntc configure` | `--registry`, `--namespace`, `--runtime-class`, `--project` | Set default config (user-level or project-level) |
| `secrets check` | `tntc secrets check [dir]` | | Check secrets provisioning against node requirements |
| `secrets init` | `tntc secrets init [dir]` | `--force` | Initialize .secrets.yaml from .secrets.yaml.example |
| `status` | `tntc status <name>` | `-n` namespace, `-o` json, `--detail` | Check deployment status in K8s; `--detail` shows pods, events, resources |
| `run` | `tntc run <name>` | `-n` namespace, `--timeout` | Trigger a deployed workflow and return JSON result |
| `logs` | `tntc logs <name>` | `-n` namespace, `-f`/`--follow`, `--tail` | View workflow pod logs; `-f` streams in real time |
| `list` | `tntc list` | `-n` namespace, `-o` json | List all deployed workflows with version, status, and age |
| `undeploy` | `tntc undeploy <name>` | `-n` namespace, `--yes`/`-y` | Remove a deployed workflow (Service, Deployment, Secret, CronJobs). Use `-y` to skip confirmation in scripts. Note: ConfigMap `<name>-code` is not deleted. |
| `cluster check` | `tntc cluster check` | `--fix`, `-n` namespace | Preflight validation of cluster readiness; `--fix` auto-remediates |
| `visualize` | `tntc visualize [dir]` | | Generate Mermaid diagram of the workflow DAG |

Global flags: `-n`/`--namespace` (default "default"),
`-r`/`--registry`, `-o`/`--output` (text\|json).

Namespace resolution order: CLI `-n` flag >
`workflow.yaml deployment.namespace` > config file
default > `default`. When `--env` is used (deploy,
test --live), environment namespace is inserted after
CLI `-n`.

Config files: `~/.tentacular/config.yaml` (user-level),
`.tentacular/config.yaml` (project-level). Project
overrides user.

### Create Config From Scratch

For this repository, the canonical public engine image is:
`ghcr.io/randybias/tentacular-engine:latest`.

Use this bootstrap flow:

```bash
mkdir -p .tentacular
cp .tentacular/config.yaml.example .tentacular/config.yaml
```

Then set at least a registry and environment image:

```yaml
registry: ghcr.io/randybias
namespace: default
runtime_class: gvisor

environments:
  prod:
    image: ghcr.io/randybias/tentacular-engine:latest
    runtime_class: gvisor
```

Without an explicit environment `image`, deploy/test-live may
fall back to `<workflow-dir>/.tentacular/base-image.txt` or
the internal default `tentacular-engine:latest`.

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
| `ctx.fetch(service, path, init?)` | `(string, string, RequestInit?) => Promise<Response>` | HTTP request with auto URL construction (`https://api.<service>.com<path>`) and auth injection from secrets (Bearer token or X-API-Key). Use full URL in `path` to bypass service resolution. |
| `ctx.log` | `Logger` | Structured logging with `info`, `warn`, `error`, `debug` methods. All output prefixed with `[nodeId]`. |
| `ctx.config` | `Record<string, unknown>` | Workflow-level config from `config:` in workflow.yaml. |
| `ctx.secrets` | `Record<string, Record<string, string>>` | Secrets loaded from `.secrets.yaml` (local) or K8s Secret volume at `/app/secrets` (production). Keyed by service name. |

## Trigger Types

| Type | Mechanism | Required Fields | K8s Resources | Status |
|------|-----------|----------------|---------------|--------|
| `manual` | HTTP POST `/run` | none | — | Implemented |
| `cron` | K8s CronJob -> curl POST `/run` | `schedule`, optional `name` | CronJob | Implemented |
| `queue` | NATS subscription -> execute | `subject` | — | Implemented |
| `webhook` | Future: gateway -> NATS bridge | `path` | — | Roadmap |

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

Queue triggers subscribe to NATS subjects. The engine
connects using config and secrets:

- **URL**: `config.nats_url` in workflow.yaml
  (e.g., `nats.ospo-dev.miralabs.dev:18453`)
- **Auth**: `secrets.nats.token` (token authentication)
- **TLS**: Uses system CA trust store (Let's Encrypt
  certs work automatically)

If either `nats_url` or `nats.token` is missing, the
engine warns and skips NATS setup (graceful degradation).

Messages are parsed as JSON and passed as input to root
nodes. If the NATS message has a reply subject, the
workflow result is sent back (request-reply pattern).

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

The recommended agentic deployment flow validates workflows through five steps. Each step produces structured JSON output (with `-o json`) for automation.

```
tntc validate -o json           # 1. Parse and validate workflow.yaml
tntc test -o json               # 2. Run mock tests (node-level + pipeline)
tntc test --live --env dev -o json  # 3. Live test against dev environment
tntc deploy -o json             # 4. Deploy (auto-gates on live test)
tntc run <name> -o json         # 5. Post-deploy verification
```

### Step Details

1. **Validate** -- parses workflow.yaml, checks name/version format, trigger definitions, node paths, edge references, and DAG acyclicity. Catches spec errors before any execution.

2. **Mock Test** -- runs node-level tests from fixtures using the mock context. No cluster or credentials required. Validates node logic and data flow in isolation.

3. **Live Test** -- deploys the workflow to a configured dev environment, triggers it, validates the result, and cleans up. Requires a `dev` environment in config (see [Environment Configuration](#environment-configuration)). Use `--env <name>` to target a different environment. Add `--keep` to skip cleanup for debugging. Default timeout is 120 seconds (`--timeout` to override).

4. **Deploy** -- generates K8s manifests and applies to the target cluster. When a dev environment is configured, deploy automatically runs a live test first and aborts if it fails. Use `--force` (alias `--skip-live-test`) to skip the live test gate. Add `--verify` to run post-deploy verification (trigger workflow once after deploy).

5. **Post-Deploy Verification** -- triggers the deployed workflow once and validates the result. Confirms the workflow runs successfully in its target environment.

### Deploy Gate

When a dev environment is configured, `tntc deploy` runs a live test before applying manifests. This prevents deploying broken workflows.

```bash
tntc deploy                     # auto-runs live test first (if dev env configured)
tntc deploy --force             # skip live test, deploy directly
tntc deploy --skip-live-test    # alias for --force
```

If the live test fails, deploy aborts with a structured error including the test failure details and hints for remediation.

### Structured Output

All commands support `-o json` for agent-consumable output. Every JSON response uses a common envelope:

```json
{
  "version": "1",
  "command": "validate|test|deploy|run",
  "status": "pass|fail",
  "summary": "human-readable one-liner",
  "hints": ["actionable suggestion if failed"],
  "timing": {
    "startedAt": "2026-02-16T09:00:00Z",
    "durationMs": 1234
  }
}
```

Commands add their own fields to the envelope:

- **validate**: validation errors array
- **test**: per-node test results with pass/fail, expected vs. actual, execution time
- **test --live**: `execution` field with the workflow run result (success, outputs, errors, timing)
- **deploy**: `execution` field on verification failure; `hints` with remediation steps on failure
- **run**: execution result with outputs, errors, and workflow timing

Example deploy output with `-o json`:

```json
{
  "version": "1",
  "command": "deploy",
  "status": "pass",
  "summary": "deployed my-workflow to production",
  "hints": [],
  "timing": { "startedAt": "2026-02-16T09:00:00Z", "durationMs": 17000 }
}
```

## Environment Configuration

Named environments extend the existing config cascade. Define environments in `~/.tentacular/config.yaml` (user-level) or `.tentacular/config.yaml` (project-level).

```yaml
registry: ghcr.io/randybias
namespace: default

environments:
  dev:
    context: kind-dev              # kubeconfig context name
    namespace: dev-workflows
    image: ghcr.io/randybias/tentacular-engine:latest
    runtime_class: ""              # no gVisor in dev
    config_overrides:
      timeout: 60s
      debug: true
    secrets_source: .secrets.yaml
  staging:
    context: staging-cluster
    namespace: staging
    image: ghcr.io/randybias/tentacular-engine:latest
    runtime_class: gvisor
```

### Environment Fields

| Field | Type | Description |
|-------|------|-------------|
| `context` | string | Kubeconfig context to use for this environment |
| `namespace` | string | Target K8s namespace |
| `image` | string | Engine image tag |
| `runtime_class` | string | RuntimeClass name (empty to disable gVisor) |
| `config_overrides` | map | Merged into workflow config for this environment |
| `secrets_source` | string | Path to secrets file for this environment |

Environment values override the base config. CLI flags override environment values. Resolution order: CLI flags > environment config > project config > user config > defaults.

### Kind Cluster Detection

When deploying to a kind cluster, the CLI auto-detects it (kubeconfig context with `kind-` prefix and localhost server) and adjusts:

- **RuntimeClass**: Set to empty (kind does not support gVisor)
- **ImagePullPolicy**: Set to `IfNotPresent` (images are loaded locally)
- **Image loading**: After `tntc build`, images are loaded into kind via `kind load docker-image`

Detection is automatic -- no manual configuration needed. A diagnostic message is printed:

```
Detected kind cluster 'dev', adjusted: no gVisor, imagePullPolicy=IfNotPresent
```

## Agent Workflow Guide

This section documents the non-interactive workflow for coding
agents deploying and testing tentacular workflows. All commands
support `-o json` for structured output; when active, progress
messages go to stderr and only the JSON envelope goes to stdout.

### Full E2E Cycle (Non-Interactive)

```bash
# 1. Validate spec
tntc validate example-workflows/my-wf -o json

# 2. Mock tests (no cluster needed)
tntc test example-workflows/my-wf -o json

# 3. Build container image
tntc build example-workflows/my-wf -t tentacular-engine:my-wf

# 4. Live test on dev (deploy -> run -> validate -> cleanup)
tntc test --live --env dev example-workflows/my-wf -o json

# 5. Deploy to target environment
tntc deploy --env prod example-workflows/my-wf -o json

# 6. Post-deploy run
tntc run my-wf -n <namespace> -o json

# 7. Cleanup when done
tntc undeploy my-wf -n <namespace> -y
```

### Image Tag Cascade

The CLI resolves the engine image in this order:

1. `--image` flag (highest priority)
2. Environment config `image` field (`--env`)
3. `<workflow-dir>/.tentacular/base-image.txt` (written
   by `tntc build`)
4. `tentacular-engine:latest` (fallback)

Repository default: configure `environments.<name>.image`
to `ghcr.io/randybias/tentacular-engine:latest` so deploys
do not rely on the unqualified fallback tag.

`tntc build` writes the built tag to the workflow directory,
not the current working directory. Both `deploy` and
`test --live` read from the workflow directory automatically.

### Build + Registry Interaction

The config `registry` value is prepended to the `-t` tag only
when the tag does not already contain a registry prefix (a `/`
before the `:`). This prevents double-prefixing:

```bash
# Config has registry: reg.io
tntc build -t my-image:v1          # builds reg.io/my-image:v1
tntc build -t reg.io/my-image:v1   # builds reg.io/my-image:v1 (no double prefix)
```

When pushing to a remote registry for ARM64 clusters:

```bash
tntc build -t tentacular-engine:my-wf \
  -r reg.io --push --platform linux/arm64
```

### Deploy with --env

The `--env` flag resolves context, namespace, runtime-class,
and image from the named environment in config. CLI flags
override environment values:

```bash
# Use everything from prod environment config
tntc deploy --env prod example-workflows/my-wf

# Override namespace from environment
tntc deploy --env prod -n custom-ns example-workflows/my-wf

# Force deploy (skip live test gate)
tntc deploy --env prod --force example-workflows/my-wf
```

### Fresh Deploy vs. Update

On fresh deployments (all resources created, none updated),
the CLI skips the rollout restart since Kubernetes already
starts pods for new Deployments. On updates to existing
resources, a rollout restart is triggered automatically.

### JSON Output Behavior

When `-o json` is active:

- **stdout**: Only the structured JSON envelope
- **stderr**: All progress messages (preflight checks,
  manifest application, rollout status, cleanup)

This lets agents parse stdout as JSON while still seeing
progress in stderr. Parse the `status` field ("pass" or
"fail") to determine success. On failure, check `hints`
for remediation steps.

### Common Gotchas for Agents

1. **undeploy needs confirmation**: Always pass `-y`
   in non-interactive scripts. Without it, the command
   blocks waiting for stdin.

2. **Namespace must exist**: `deploy` does not create
   namespaces. Create them first with `kubectl create
   namespace <name>`.

3. **kubeconfig context**: When targeting multiple
   clusters, use `--env` to switch contexts automatically.
   Without `--env`, the CLI uses the current kubeconfig
   context.

4. **kind clusters**: Auto-detected by context name
   prefix `kind-`. gVisor and imagePullPolicy are
   adjusted automatically. After `tntc build`, images
   are loaded into kind via `kind load docker-image`.

5. **Secrets**: Local secrets come from
   `<workflow-dir>/.secrets.yaml`. The CLI generates a
   K8s Secret manifest from this file during deploy.
   If `.secrets/` directory exists, it takes precedence
   over `.secrets.yaml`.

6. **Deploy gate**: When a `dev` environment is
   configured, `deploy` auto-runs a live test first.
   Use `--force` to skip. This only triggers when the
   CLI detects a dev environment in config.

## References

For detailed documentation on specific topics:

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
