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

## CLI Quick Reference

| Command | Usage | Key Flags | Description |
|---------|-------|-----------|-------------|
| `init` | `tntc init <name>` | | Scaffold a new workflow directory with workflow.yaml, example node, test fixture, .secrets.yaml.example |
| `validate` | `tntc validate [dir]` | | Parse and validate workflow.yaml (name, version, triggers, nodes, edges, DAG acyclicity, contract structure, derived artifacts) |
| `dev` | `tntc dev [dir]` | `-p` port (default 8080) | Start Deno engine locally with hot-reload (`--watch`). POST /run triggers execution |
| `test` | `tntc test [dir][/<node>]` | `--pipeline`, `--live`, `--env`, `--keep`, `--timeout`, `--warn` | Run node-level tests from fixtures, full pipeline test with `--pipeline`, or live cluster test with `--live`. `--warn` downgrades contract violations to warnings. |
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
| `visualize` | `tntc visualize [dir]` | `--rich`, `--write` | Generate Mermaid diagram of the workflow DAG. `--rich` adds dependency graph, derived secrets, and network intent. `--write` writes `workflow-diagram.md` and `contract-summary.md` to the workflow directory. |

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
| `ctx.dependency(name)` | `(string) => DependencyConnection` | **Primary API** for external services. Returns connection metadata (host, port, protocol, authType, protocol-specific fields) and resolved secret value. HTTPS deps also get a `fetch(path, init?)` convenience method that builds the URL (no auth injection -- nodes handle auth explicitly). Throws if the dependency is not declared in the contract. |
| `ctx.log` | `Logger` | Structured logging with `info`, `warn`, `error`, `debug` methods. All output prefixed with `[nodeId]`. |
| `ctx.config` | `Record<string, unknown>` | Workflow-level config from `config:` in workflow.yaml. Use for business-logic parameters only (e.g., `target_repo`). |
| `ctx.fetch(service, path, init?)` | `(string, string, RequestInit?) => Promise<Response>` | **Legacy.** HTTP request with auto URL construction. Flagged as contract violation when a contract is present. Use `ctx.dependency()` instead. |
| `ctx.secrets` | `Record<string, Record<string, string>>` | **Legacy.** Direct secret access. Flagged as contract violation when a contract is present. Use `ctx.dependency()` instead. |

### Using ctx.dependency()

Nodes access external service connection info through
the contract dependency API:

```typescript
import type { Context } from "tentacular";

export default async function run(
  ctx: Context,
  input: unknown
): Promise<unknown> {
  const pg = ctx.dependency("postgres");
  // pg.protocol -- "postgresql"
  // pg.host, pg.port, pg.database, pg.user
  // pg.authType -- "password" (any string)
  // pg.secret -- resolved password value

  const gh = ctx.dependency("github-api");
  // gh.protocol -- "https"
  // gh.host, gh.port
  // gh.authType -- "bearer-token" (any string)
  // gh.secret -- resolved bearer token
  // gh.fetch(path, init?) -- URL builder (no auth injection)

  // HTTPS deps have a fetch() shortcut that builds the URL.
  // Auth must be handled explicitly by the node:
  const resp = await gh.fetch!("/repos/org/repo", {
    headers: { "Authorization": `Bearer ${gh.secret}` },
  });

  ctx.log.info("connecting to dependencies");
  return { result: "done" };
}
```

**Auth is explicit.** `dep.fetch()` builds the URL
(`https://<host>:<port><path>`) but does not inject
auth headers. Nodes must set auth headers themselves
using `dep.secret` and `dep.authType`. This keeps the
engine simple and supports any auth mechanism:

```typescript
// Bearer token (e.g., GitHub API)
const gh = ctx.dependency("github-api");
await gh.fetch!("/repos/org/repo", {
  headers: { "Authorization": `Bearer ${gh.secret}` },
});

// API key header
const svc = ctx.dependency("my-service");
await svc.fetch!("/endpoint", {
  headers: { "X-API-Key": svc.secret! },
});

// Custom auth (e.g., HMAC signature)
const api = ctx.dependency("webhook-target");
// api.authType -- "hmac-sha256"
const sig = computeHmac(api.secret!, body);
await api.fetch!("/hook", {
  method: "POST",
  headers: { "X-Signature": sig },
  body,
});
```

In mock context (during `tntc test`), `ctx.dependency()`
returns mock values and records the access for
runtime-tracing drift detection.

**Migration note:** Replace `ctx.config` + `ctx.secrets`
assembly with `ctx.dependency()`. The `config` section
in workflow.yaml should contain only business-logic
parameters (e.g., `target_repo`, `sep_label`).
Connection metadata (host, port, database, user) and
secret references belong in `contract.dependencies`.

**Migration from auto-injection:** If your nodes
previously relied on `dep.fetch()` auto-injecting auth
headers, update them to set headers explicitly:

```typescript
// Before (auto-injection -- no longer supported):
const gh = ctx.dependency("github-api");
const res = await gh.fetch!("/repos/test");

// After (explicit auth):
const gh = ctx.dependency("github-api");
const res = await gh.fetch!("/repos/test", {
  headers: { "Authorization": `Bearer ${gh.secret}` },
});
```

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

## Contract Model

The `contract` section in `workflow.yaml` is the
authoritative declaration of every external dependency
a workflow needs. It is a top-level peer of `nodes`,
`edges`, and `config`.

**Core principle: dependencies are the single primitive.**
A Tentacular workflow is a sealed pod that can only reach
declared network dependencies. Secrets, NetworkPolicy,
connection config, and validation are ALL derived from
the dependency list. There is no separate `secrets` or
`networkPolicy` section to author -- both are derived
automatically.

### Contract Structure

```yaml
contract:
  version: "1"
  dependencies:
    github-api:
      protocol: https
      host: api.github.com
      # port defaults to 443 for https
      auth:
        type: bearer-token
        secret: github.token
    postgres:
      protocol: postgresql
      host: db.svc.cluster.local
      # port defaults to 5432 for postgresql
      database: appdb
      user: postgres
      auth:
        type: password
        secret: postgres.password
    azure-blob:
      protocol: https
      host: myaccount.blob.core.windows.net
      auth:
        type: sas-token
        secret: azure-blob.sas_token
    slack-webhook:
      protocol: https
      host: hooks.slack.com
      auth:
        type: bearer-token
        secret: slack.webhook_url
```

### What Gets Derived

| Artifact | Source | Derivation |
|----------|--------|------------|
| Required secrets | `dep.auth.secret` | Collect all auth secret refs |
| Egress NetworkPolicy | `dep.host` + `dep.port` | One allow rule per dep + DNS |
| Ingress NetworkPolicy | `triggers[].type` | Webhook: allow; else deny all |
| Connection config | `dep.*` metadata | Injected via `ctx.dependency()` |

### Dependency Protocols

The `protocol` field accepts any string. Known
protocols (`https`, `postgresql`, `nats`, `blob`)
get field validation and default ports. Unknown
protocols are accepted with a warning, enabling
custom protocol types without parser changes.

Known protocols and their metadata fields:

- **https**: host, port (default 443), auth
- **postgresql**: host, port (default 5432),
  database, user, auth
- **nats**: host, port (default 4222), auth,
  subject
- **blob**: host, port (default 443), auth,
  container

Auth types are declared explicitly via `auth.type`
in the contract. The value is any string identifying
the auth mechanism (e.g., `bearer-token`, `api-key`,
`sas-token`, `password`, `hmac-sha256`). There is no
closed vocabulary -- use whatever describes your auth
scheme. The `authType` field on `DependencyConnection`
reflects this declared value. Nodes use `dep.authType`
and `dep.secret` to handle auth explicitly.

### Minimal Contract (No Dependencies)

Workflows with no external dependencies use an empty
dependency map:

```yaml
contract:
  version: "1"
  dependencies: {}
```

### NetworkPolicy Overrides

For edge cases not derivable from dependencies, use
optional overrides:

```yaml
contract:
  version: "1"
  dependencies: { ... }
  networkPolicy:
    additionalEgress:
      - cidr: 10.0.0.0/8
        port: 8080
        protocol: TCP
        reason: "internal service mesh"
```

### Contract Enforcement

Contract enforcement is **strict** by default. All
workflows are held to the same standard.

In strict mode, `tntc test` fails on any contract
drift. Use `--warn` to downgrade violations to
warnings without failing the test:

```bash
tntc test                     # strict: violations fail
tntc test --warn              # audit: violations warn
```

Environment config can also set audit mode globally
for a development environment:

```yaml
# In .tentacular/config.yaml
environments:
  dev:
    enforcement: audit
```

### Drift Detection

`tntc test` runs runtime-tracing drift detection by
comparing actual code behavior against contract
declarations. The mock context records all access
patterns during test execution.

**Violation types:**

| Type | Meaning | Suggestion |
|------|---------|------------|
| `direct-fetch` | Code uses `ctx.fetch()` instead of `ctx.dependency().fetch()` | Migrate to `ctx.dependency()` |
| `direct-secrets` | Code reads `ctx.secrets` directly | Use `ctx.dependency().secret` |
| `undeclared-dependency` | Code calls `ctx.dependency(name)` for a dep not in the contract | Add to `contract.dependencies` |
| `dead-declaration` | Contract declares a dep that code never accesses | Remove from contract or add usage |

**Drift report output:**

```
=== Contract Drift Report ===

VIOLATIONS:
  [direct-fetch] Direct ctx.fetch("github", "/repos/test") bypasses contract
     Suggestion: Use ctx.dependency("github").fetch("/repos/test") instead
  [dead-declaration] Dependency "unused-api" declared in contract but never accessed
     Suggestion: Remove "unused-api" from contract.dependencies or ensure the node uses it

SUMMARY:
  Dependencies accessed: 2
  Direct fetch() calls: 1
  Direct secrets access: 0
  Dead declarations: 1
  Undeclared dependencies: 0
  Has violations: YES
```

With `-o json`, the drift report is included in the
test output envelope as a structured `drift` field.

### Extensibility

Extension fields are supported via `x-*` namespaced
keys. They are preserved through parsing and do not
break core schema validation.

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
workflows through six steps. Each step produces
structured JSON output (with `-o json`) for automation.

```
tntc validate -o json               # 1. Validate spec + contract
tntc visualize --rich --write       # 2. Persist contract artifacts
tntc test -o json                   # 3. Mock tests + drift detection
tntc test --live --env dev -o json  # 4. Live test against dev
tntc deploy -o json                 # 5. Deploy (auto-gates on live)
tntc run <name> -o json             # 6. Post-deploy verification
```

### Step Details

1. **Validate** -- parses workflow.yaml, checks
   name/version format, trigger definitions, node paths,
   edge references, DAG acyclicity, and contract
   structure. Also displays derived artifacts: secret
   inventory and network policy summary. Catches spec
   and contract errors before any execution.

2. **Review Contract Artifacts** -- generates rich
   visualization showing DAG topology, dependency graph,
   derived secrets, and network intent. Agent and user
   review these artifacts before proceeding to test or
   build. See [Pre-Build Review Gate](#pre-build-review-gate).

3. **Mock Test** -- runs node-level tests from fixtures
   using the mock context. No cluster or credentials
   required. Validates node logic, data flow, and
   contract drift (undeclared deps, dead deps, API
   bypass). Fails in strict mode on any drift.

4. **Live Test** -- deploys the workflow to a configured
   dev environment, triggers it, validates the result,
   and cleans up. Requires a `dev` environment in config
   (see [Environment Configuration](#environment-configuration)).
   Use `--env <name>` to target a different environment.
   Add `--keep` to skip cleanup for debugging. Default
   timeout is 120 seconds (`--timeout` to override).

5. **Deploy** -- generates K8s manifests including
   auto-derived NetworkPolicy from contract dependencies
   and trigger types. When a dev environment is
   configured, deploy automatically runs a live test
   first and aborts if it fails. Use `--force` (alias
   `--skip-live-test`) to skip the live test gate. Add
   `--verify` for post-deploy verification. Deploy
   aborts before manifest apply if contract validation
   fails in strict mode.

6. **Post-Deploy Verification** -- triggers the deployed
   workflow once and validates the result. Confirms the
   workflow runs successfully in its target environment.

### Deploy Gate

When a dev environment is configured, `tntc deploy` runs a live test before applying manifests. This prevents deploying broken workflows.

```bash
tntc deploy                     # auto-runs live test first (if dev env configured)
tntc deploy --force             # skip live test, deploy directly
tntc deploy --skip-live-test    # alias for --force
```

If the live test fails, deploy aborts with a structured
error including the test failure details and hints for
remediation.

### Pre-Build Review Gate

Before any `build`, `test --live`, or `deploy`, the
agent MUST run a contract review loop:

1. `tntc validate` -- confirm contract parses cleanly
   and derived artifacts are correct.
2. `tntc visualize --rich --write` -- generate and
   persist rich diagram and contract summary to the
   workflow directory.
3. Review with user -- present the diagram and derived
   artifacts. Confirm:
   - Dependency targets (hosts, ports, protocols)
   - Secret key references match provisioned secrets
   - Derived network policy matches expected access
4. `tntc test` -- run mock tests with drift detection.
   Resolve any contract violations before proceeding.

**Required checklist before build/deploy:**

- [ ] Contract section present in workflow.yaml
- [ ] `tntc validate` passes (contract + spec)
- [ ] `tntc visualize --rich --write` reviewed with user
- [ ] Dependency hosts and ports confirmed
- [ ] Secret refs match `.secrets.yaml` keys
- [ ] Derived NetworkPolicy matches expected access
- [ ] `tntc test` passes with zero drift
- [ ] `workflow-diagram.md` and `contract-summary.md` committed to repo

Agents MUST fail closed when contract or diagram
artifacts are missing or stale. Do not proceed to
build or deploy without completing this gate.

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

### Develop a Plan in Advance for New or Updated Workflows

Before writing or changing workflow code, the agent MUST run a
planning loop with the user. The goal is to eliminate hidden
dependencies (especially secrets and runtime config) before
implementation starts.

#### Planning Objectives

1. Confirm the user's intent and expected outcome.
2. Author the `contract.dependencies` block first.
3. Derive secrets and network intent from the contract.
4. Pre-validate the environment, credentials, and
   connectivity.
5. Define explicit dev-to-prod promotion gates.

#### Step 1: Confirm User Intent (Do Not Assume)

Ask targeted questions and restate the intent before coding:

1. What business outcome should this workflow produce?
2. What triggers are required (`manual`, `cron`, `queue`)?
3. What are the expected inputs and outputs?
4. What systems are read-only vs. write targets?
5. What constitutes success vs. acceptable degradation?

The agent should summarize the intent back to the user and get
confirmation before proceeding.

#### Step 2: Author the Contract

The planning loop starts with contract authoring.
Enumerate all external dependencies and write the
`contract.dependencies` block in `workflow.yaml`:

1. APIs/services (GitHub, Slack, NATS, cloud storage)
2. Data stores (Postgres, Redis, object stores)
3. For each: protocol, host, port, auth type, secret ref
4. Environment-specific behavior (`dev` vs. `prod`)
5. Required trigger scheduling and runtime behavior

If any dependency is uncertain, stop and ask. Do not
proceed with guessed endpoints, guessed credentials,
or placeholder resources unless explicitly requested
for mock-only testing.

Once the contract is authored, run `tntc validate` and
`tntc visualize --rich` to verify derived artifacts:

- Derived secrets: confirm all `auth.secret` refs match
  provisioned keys in `.secrets.yaml`
- Derived NetworkPolicy: confirm egress rules match
  expected network access
- Review the rich visualization with the user

#### Step 3: Define Config and Secrets Sources

The `config` section now holds only business-logic
parameters (e.g., `target_repo`, `sep_label`).
Connection metadata belongs in contract dependencies.

Secrets handling policy:

1. Secret keys are derived from contract `auth.secret`
   refs -- do not define them separately
2. Current source: local workflow secrets
   (`<workflow>/.secrets.yaml` or `<workflow>/.secrets/`)
3. Future source: external secrets vault (**FUTURE**)
4. Never use environment variables for workflow secrets
5. Never print secret values in terminal output
6. Validate key presence and format before live runs

Minimum to confirm with user:

1. Secret key names match contract `auth.secret` refs
2. Real target endpoints match contract host/port values
3. Environment mapping (`dev` namespace/context/image
   and `prod` namespace/context/image)
4. Expected side effects per environment

#### Step 4: Planning Loop With User (Round-Trip Until Stable)

The agent should propose a concrete plan and iterate with the
user until there are no open questions.

Plan must include:

1. Workflow changes to make
2. Secrets/config values required per environment
3. Pre-validation checks to run first
4. Test sequence (unit/mock, pipeline, live dev)
5. Promotion gate for prod deploy
6. Rollback/cleanup plan (`tntc undeploy ... -y`)

The plan should be repeated back in detailed form after each
user clarification. Do not start implementation until the plan
is explicitly confirmed.

#### Step 5: Pre-Validate Before Implementation Work

Run lightweight checks before coding/deploying:

```bash
# Validate cluster targets
tntc cluster check --fix -n <dev-namespace>
tntc cluster check --fix -n <prod-namespace>

# Validate workflow spec and contract
tntc validate <workflow-dir>

# Review derived artifacts with user
tntc visualize --rich <workflow-dir>

# Check secrets provisioning vs contract
tntc secrets check <workflow-dir>

# Confirm config points to expected image and envs
cat .tentacular/config.yaml
```

Also pre-validate critical credentials/connectivity where
possible (without exposing secret values), for example:

1. GitHub token format and API reachability
2. Database credentials vs. expected DB target
3. Slack webhook format and safe test delivery target
4. Storage URL/container existence and write permissions
5. Queue URL/auth/subject validity for queue triggers

If validation fails, fix inputs first; do not continue blindly.

#### Step 6: Enforce Dev E2E Gate Before Prod

Every new or updated workflow must pass the full
contract and testing pipeline before production:

1. `tntc validate` (spec + contract)
2. `tntc visualize --rich` (review with user)
3. `tntc test` (mock tests + drift detection)
4. `tntc test --pipeline`
5. `tntc test --live --env dev`
6. Review outputs/logs and confirm side effects
7. Only then `tntc deploy --env prod` (with `--verify`)

Using separate dev creds is fine and encouraged. The key
requirement is that dev is a real environment with real
integration behavior, not mock-only.

#### Non-Negotiable Agent Rules

1. Do not deploy to prod without a successful live dev run
   unless the user explicitly overrides this decision.
2. Do not use placeholder credentials for live testing.
3. Do not proceed when secret keys/config targets are ambiguous.
4. Always state resolved image, context, and namespace before
   running live tests or deploy.
5. Always clean up temporary deployments after live testing
   unless `--keep` is intentionally requested.

### Full E2E Cycle (Non-Interactive)

```bash
# 1. Validate spec + contract
tntc validate example-workflows/my-wf -o json

# 2. Persist and review contract artifacts with user
tntc visualize --rich --write example-workflows/my-wf

# 3. Mock tests + drift detection (no cluster needed)
tntc test example-workflows/my-wf -o json

# 4. Build container image
tntc build example-workflows/my-wf \
  -t tentacular-engine:my-wf

# 5. Live test on dev (deploy -> run -> validate -> cleanup)
tntc test --live --env dev \
  example-workflows/my-wf -o json

# 6. Deploy to target environment
tntc deploy --env prod \
  example-workflows/my-wf -o json

# 7. Post-deploy run
tntc run my-wf -n <namespace> -o json

# 8. Cleanup when done
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

7. **Contract required**: All workflows need a
   `contract` section (even if `dependencies: {}`).
   Deploy fails in strict mode without a valid
   contract. See [Contract Model](#contract-model).

8. **NetworkPolicy auto-generated**: `deploy`
   generates NetworkPolicy from contract dependencies.
   Verify with `kubectl get networkpolicy` after
   deploy. Use `contract.networkPolicy.additionalEgress`
   for edge cases not derivable from dependencies.

9. **Drift detection**: `tntc test` compares runtime
   behavior against contract declarations. Direct
   `ctx.fetch()` or `ctx.secrets` usage is flagged
   as a contract violation. Use `ctx.dependency()`.

## Visualization Reference

`tntc visualize` generates workflow diagrams. The
`--rich` flag adds contract-derived metadata.

### Basic Mode

```bash
tntc visualize example-workflows/sep-tracker
```

Produces a Mermaid diagram of the DAG topology
(nodes and edges only).

### Rich Mode

```bash
tntc visualize --rich example-workflows/sep-tracker
```

Rich output includes:

- **DAG topology**: nodes and edges as in basic mode
- **Dependency nodes**: external services shown with
  protocol and host labels
- **Derived secret inventory**: all secret key refs
  collected from `dep.auth.secret` across dependencies
- **Network intent summary**: derived egress and
  ingress rules from contract + triggers

Rich visualization output is deterministic and stable
for diffs.

### Write Mode

```bash
tntc visualize --rich --write example-workflows/sep-tracker
```

The `--write` flag writes artifacts to the workflow
directory instead of printing to stdout:

- **`workflow-diagram.md`** -- Mermaid diagram content
  (with code fence markers), ready for rendering
- **`contract-summary.md`** -- Derived secrets
  inventory, egress rules, and ingress rules as
  markdown tables

These files are co-resident with the workflow for
PR review. Output is deterministic and stable for
diffs. Agents MUST use `--write` during the pre-build
review gate to persist artifacts for commit.

### Example: sep-tracker Workflow Diagram

![sep-tracker workflow diagram](../example-workflows/sep-tracker/workflow-diagram.png)

The sep-tracker workflow demonstrates a full contract
with four external dependencies (GitHub API, Postgres,
Azure Blob Storage, Slack webhook). The rich
visualization shows how each dependency connects to
the nodes that use it, along with derived secrets and
network policy rules.

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
