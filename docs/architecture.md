# Pipedreamer v2 Architecture

## 1. System Overview

Pipedreamer v2 is a workflow execution platform that runs TypeScript DAGs on Kubernetes with defense-in-depth sandboxing. A Go CLI manages the full lifecycle — scaffolding, validation, local dev, testing, container builds, and K8s deployments — while a Deno engine executes workflow DAGs inside hardened containers with gVisor kernel isolation.

```
                     Developer Machine                          Kubernetes Cluster
                ┌──────────────────────────┐          ┌────────────────────────────────┐
                │                          │          │                                │
                │  pipedreamer CLI (Go)    │          │   ┌────────────────────────┐   │
                │  ┌────────────────────┐  │  deploy  │   │  Pod (gVisor sandbox)  │   │
                │  │ init / validate    │  │ ───────> │   │  ┌──────────────────┐  │   │
                │  │ dev / test         │  │          │   │  │ Deno Engine (TS) │  │   │
                │  │ build / deploy     │  │  status  │   │  │ ┌──────────────┐ │  │   │
                │  │ status / cluster   │  │ <─────── │   │  │ │ Workflow DAG │ │  │   │
                │  │ visualize          │  │          │   │  │ └──────────────┘ │  │   │
                │  └────────────────────┘  │          │   │  └──────────────────┘  │   │
                │           │              │          │   │  /app/secrets (vol)     │   │
                │      ┌────┴────┐         │          │   └────────────────────────┘   │
                │      │ Docker  │         │  push    │   K8s Secret ──┘               │
                │      │ Build   │ ────────│────────> │   Zot Registry                 │
                │      └─────────┘         │          │                                │
                └──────────────────────────┘          └────────────────────────────────┘
```

| Directory | Purpose |
|-----------|---------|
| `cmd/pipedreamer/` | CLI entry point (Cobra command tree) |
| `pkg/` | Go packages: spec parser, builder, K8s client, CLI commands |
| `engine/` | Deno TypeScript engine: compiler, executor, context, server |
| `examples/` | Runnable example workflows (github-digest, hn-digest, pr-digest) |
| `deploy/` | Infrastructure scripts (gVisor installation, RuntimeClass) |
| `openspec/` | Change tracking and specifications |
| `docs/` | Project documentation |

## 2. Go CLI Architecture

### Command Tree

```
pipedreamer
├── init <name>         Scaffold new workflow project
├── validate [dir]      Validate workflow.yaml spec
├── dev [dir]           Local dev server with hot-reload
├── test [dir/node]     Run node or pipeline tests
├── build [dir]         Build container image
├── deploy [dir]        Deploy to Kubernetes
├── status <name>       Check deployment health (--detail for extended info)
├── run <name>          Trigger a deployed workflow
├── logs <name>         View workflow pod logs (--follow to stream)
├── list                List deployed workflows
├── undeploy <name>     Remove a deployed workflow
├── cluster check       Preflight cluster validation
└── visualize [dir]     Generate Mermaid DAG diagram
```

Global flags: `--namespace`, `--registry`, `--output` (text|json)

### Package Layout

```
pkg/
├── spec/           Workflow YAML parser + validator
│   ├── types.go        Workflow, NodeSpec, Edge, Trigger, WorkflowConfig
│   └── parse.go        Parse(), checkCycles() — kebab-case, semver, DAG validation
├── builder/        Artifact generation (no runtime deps)
│   ├── dockerfile.go   GenerateDockerfile() → distroless Deno container
│   └── k8s.go          GenerateK8sManifests() → Deployment + Service YAML
├── cli/            Cobra command implementations
│   ├── init.go         Scaffold: workflow.yaml, nodes/hello.ts, fixtures
│   ├── validate.go     Parse + report errors
│   ├── dev.go          Spawn Deno engine with --watch, graceful shutdown
│   ├── test.go         Run Deno test runner against fixtures
│   ├── build.go        Docker build with engine copy, tag derivation
│   ├── deploy.go       Generate manifests, provision secrets, k8s.Apply()
│   ├── status.go       Query deployment via k8s.GetStatus() (--detail for extended info)
│   ├── run.go          Trigger deployed workflow via temp curl pod
│   ├── logs.go         Stream/tail pod logs via k8s.GetPodLogs()
│   ├── list.go         List deployed workflows via k8s.ListWorkflows()
│   ├── undeploy.go     Remove deployed workflow via k8s.DeleteResources()
│   ├── cluster.go      Preflight checks with --fix auto-remediation
│   └── visualize.go    Mermaid graph output
└── k8s/            Kubernetes client operations
    ├── client.go       NewClient(), Apply(), GetStatus(), DeleteResources(),
    │                   ListWorkflows(), GetPodLogs(), RunWorkflow(), GetDetailedStatus()
    └── preflight.go    PreflightCheck(): API, gVisor, namespace, RBAC, secrets
```

### Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing for workflow specs and secrets
- `k8s.io/client-go` — K8s API client (apply, status, preflight checks)
- `k8s.io/apimachinery` — K8s types and API machinery

## 3. Deno Engine Architecture

### Startup Sequence

```
1.  Parse CLI flags: --workflow, --port, --secrets, --watch
2.  Resolve workflow path
3.  Load workflow.yaml → WorkflowSpec
4.  Compile DAG → CompiledDAG (Kahn's algorithm → stages)
5.  Resolve secrets (cascade: explicit → .secrets/ → .secrets.yaml → /app/secrets)
6.  Load all node modules via dynamic import
7.  Create base Context (fetch, log, config, secrets)
8.  Create NodeRunner (per-node context creation)
9.  Parse timeout/retry config
10. Start HTTP server on configured port
11. Start NATS triggers if queue triggers defined (dynamic import)
12. Register signal handlers (SIGTERM/SIGINT) for graceful shutdown
13. (Optional) Start file watcher for hot-reload
```

### Compilation Pipeline

The compiler transforms a `WorkflowSpec` into a `CompiledDAG` with execution stages:

1. **Spec validation** — verify required fields present (`name`, `nodes` with at least one entry, `edges` array)
2. **Edge validation** — verify all edge references point to defined nodes, detect self-loops
3. **Topological sort** — Kahn's algorithm with deterministic ordering (sorted queue)
4. **Stage grouping** — nodes grouped by dependency depth; same-stage nodes have all deps in earlier stages

```
workflow.yaml edges:        Compiled stages:
  fetch → transform         Stage 1: [fetch]      (parallel)
  fetch → enrich            Stage 2: [enrich, transform]  (parallel)
  transform → notify        Stage 3: [notify]
  enrich → notify
```

### Execution Model

- **Stages execute sequentially** — each stage waits for the previous to complete
- **Nodes within a stage execute in parallel** — via `Promise.all()`
- **Input resolution** — single dependency: pass output directly; multiple dependencies: merge into keyed object
- **Timeout** — per-node timeout with `Promise.race` pattern (default 30s)
- **Retry** — exponential backoff: 100ms, 200ms, 400ms... up to maxRetries
- **Fail-fast** — if any node in a stage fails, execution stops immediately

### Context System

Each node receives a `Context` object:

```typescript
interface Context {
  fetch(service: string, path: string, init?: RequestInit): Promise<Response>;
  log: Logger;          // info, warn, error, debug — prefixed with [nodeId]
  config: Record<string, unknown>;
  secrets: SecretsConfig;
}
```

- **`ctx.fetch(service, path)`** — resolves to `https://api.{service}.com{path}`, auto-injects auth headers from secrets (Bearer token or X-API-Key)
- **`ctx.log`** — structured logging with node ID prefix
- **`ctx.config`** — workflow-level config from `config:` block. The config block is **open**: arbitrary keys (e.g., `nats_url`) are preserved alongside typed fields (`timeout`, `retries`). In Go, extra keys flow into `WorkflowConfig.Extras` via `yaml:",inline"`. Use `ToMap()` to get a flat merged map.
- **`ctx.secrets`** — loaded from cascade (see Section 7)

### Module Loader

- Nodes are loaded via `dynamic import()` with absolute file:// URLs
- **Path traversal guard** — resolved paths are validated to stay within the workflow directory
- Cache busting for hot-reload: `?t={timestamp}` query parameter
- Type validation: verifies default export is an async function
- `clearModuleCache()` resets the loader for dev reloads
- Hot-reload uses atomic reference swap — the runner reads `nodeRef.current` at call time, so in-flight requests complete with the old module map while new requests get the reloaded one

### Import Map (`deno.json`)

```json
{
  "imports": {
    "pipedreamer": "./mod.ts",
    "std/": "https://deno.land/std@0.224.0/"
  }
}
```

Nodes import types via: `import type { Context } from "pipedreamer"`

## 4. Triggers

Triggers define how workflow execution is initiated. Each workflow specifies one or more triggers in its `triggers:` array.

| Type | Mechanism | Required Fields | K8s Resources | Status |
|------|-----------|----------------|---------------|--------|
| `manual` | HTTP POST `/run` | none | — | Implemented |
| `cron` | K8s CronJob → curl POST `/run` | `schedule`, optional `name` | CronJob | Implemented |
| `queue` | NATS subscription → execute | `subject` | — | Implemented |
| `webhook` | Future: gateway → NATS bridge | `path` | — | Roadmap |

### Named Triggers

Triggers can have an optional `name` field for parameterized execution. CronJobs POST `{"trigger": "<name>"}` to `/run`, and root nodes receive this as input. This supports workflows with multiple cron schedules that branch behavior based on `input.trigger`.

```yaml
triggers:
  - type: cron
    name: daily-digest
    schedule: "0 9 * * *"
  - type: cron
    name: hourly-check
    schedule: "0 * * * *"
```

### Cron Triggers

Cron triggers generate K8s CronJob manifests during `pipedreamer deploy`. Each CronJob uses `curlimages/curl` to POST to the workflow's ClusterIP service at `http://{name}.{namespace}.svc.cluster.local:8080/run`.

- **Naming**: `{wf}-cron` (single trigger) or `{wf}-cron-0`, `{wf}-cron-1` (multiple)
- **concurrencyPolicy**: `Forbid` (no overlapping runs)
- **historyLimits**: 3 successful, 3 failed
- **Labels**: `app.kubernetes.io/name` and `app.kubernetes.io/managed-by: pipedreamer`
- **Cleanup**: `pipedreamer undeploy` deletes CronJobs by label selector

### Queue Triggers (NATS)

Queue triggers subscribe to NATS subjects. Messages trigger workflow execution with the message payload as input.

- **Connection**: TLS + token auth via `config.nats_url` and `secrets.nats.token`
- **Dynamic import**: NATS library only loaded when queue triggers exist
- **Request-reply**: If a message has a reply subject, the execution result is sent back
- **Graceful shutdown**: SIGTERM/SIGINT drain NATS subscriptions before exit
- **Degradation**: Engine warns and skips if `nats_url` or `nats.token` is missing

### POST Body Passthrough

The `/run` endpoint parses POST body as JSON and passes it as initial input to root nodes (nodes with no incoming edges). GET requests and empty bodies default to `{}`.

## 5. Security Model

Five layers of defense-in-depth, from innermost to outermost:

### Layer 1: Distroless Base Image

Container uses `denoland/deno:distroless` — no shell, no package manager, no debugging tools. Attack surface is limited to the Deno runtime binary.

### Layer 2: Deno Permission Locking

Entrypoint flags restrict the Deno runtime:
- `--allow-net` — network access (required for ctx.fetch and HTTP server)
- `--allow-read=/app` — read-only access to /app (workflow files)
- `--allow-write=/tmp` — write access only to /tmp (ephemeral scratch)

No file system, subprocess, FFI, or environment variable access beyond these.

### Layer 3: gVisor Sandbox

Pods run with `runtimeClassName: gvisor`. gVisor intercepts syscalls via its application kernel (Sentry), preventing direct host kernel access. Even if a container escape is achieved, the attacker lands in gVisor's sandbox, not the host.

### Layer 4: Kubernetes SecurityContext

```yaml
securityContext:                    # Pod level
  runAsNonRoot: true
  runAsUser: 65534                  # nobody
  seccompProfile:
    type: RuntimeDefault

securityContext:                    # Container level
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
```

### Layer 5: Secrets as Volume Mounts

Secrets are mounted as read-only files at `/app/secrets` from a K8s Secret resource (`optional: true`). They are never exposed as environment variables — env vars are visible in `kubectl describe pod`, process listings, and crash dumps.

## 6. Deployment Pipeline

### Build Phase

```
pipedreamer build [dir]
  1. Parse workflow.yaml → derive image tag (name:version)
  2. Locate engine directory (relative to binary)
  3. Copy engine → .engine/ in build context
  4. Generate Dockerfile.pipedreamer
  5. docker build -f Dockerfile.pipedreamer -t <tag> .
  6. Cleanup: remove .engine/ and Dockerfile.pipedreamer
  7. (Optional) docker push with --push flag
```

### Deploy Phase

```
pipedreamer deploy [dir]
  1. Parse workflow.yaml → validate spec
  2. Derive image tag (with --cluster-registry or --registry prefix)
  3. GenerateK8sManifests() → Deployment + Service
  4. buildSecretManifest() → K8s Secret from .secrets/ or .secrets.yaml
  5. k8s.Client.Apply() → create-or-update all manifests
```

### Operations Phase

```
pipedreamer status <name>        Query Deployment readiness/replicas
  --detail                       Extended info: image, runtime, pods, events
pipedreamer run <name>           Trigger workflow via temp curl pod, return JSON result
  --timeout 30s                  Maximum wait time
pipedreamer logs <name>          View pod logs (last 100 lines by default)
  --follow/-f                    Stream logs in real time
  --tail N                       Number of recent lines
pipedreamer list                 List all pipedreamer-managed deployments
pipedreamer undeploy <name>      Remove Service, Deployment, and Secret
  --yes                          Skip confirmation prompt
pipedreamer cluster check        Preflight: API, gVisor, namespace, RBAC, secrets
  --fix                          Auto-create namespace if missing
```

### Generated K8s Resources

| Resource | Name | Key Fields |
|----------|------|------------|
| Deployment | `{workflow-name}` | 1 replica, gVisor RuntimeClass, security contexts, probes, resource limits |
| Service | `{workflow-name}` | ClusterIP, port 8080 |
| Secret | `{workflow-name}-secrets` | Opaque, stringData from .secrets/ or .secrets.yaml |
| CronJob | `{wf}-cron` or `{wf}-cron-{i}` | Per cron trigger. curlimages/curl, concurrencyPolicy: Forbid, historyLimit: 3 |

## 7. Secrets Management

### Cascade Precedence

Secrets are resolved in order, with later sources merging on top of earlier ones:

| Priority | Source | Description |
|----------|--------|-------------|
| 1 (highest) | `--secrets <path>` | Explicit flag — skips all other sources |
| 2 | `/app/secrets` | K8s Secret volume mount (always checked last, merges on top) |
| 3 | `.secrets.yaml` | YAML file in workflow directory |
| 4 (base) | `.secrets/` | Directory of files (K8s volume mount format) |

When no explicit path is given: `.secrets/` provides the base, `.secrets.yaml` merges on top, then `/app/secrets` merges on top of everything.

### File Formats

**YAML file** (`.secrets.yaml`):
```yaml
github:
  token: ghp_abc123
stripe:
  api_key: sk_test_xyz
```

**Directory** (`.secrets/`):
```
.secrets/
  github     → {"token": "ghp_abc123"}   (JSON parsed)
  api-token  → my-plain-token             (wrapped as {value: "..."})
```

### Deploy-Time Provisioning

`pipedreamer deploy` auto-provisions secrets to K8s:
1. Check for `.secrets/` directory → `buildSecretFromDir()` (files as stringData entries)
2. Fall back to `.secrets.yaml` → `buildSecretFromYAML()` (YAML keys as stringData entries)
3. Generated K8s Secret named `{workflow}-secrets`, applied alongside Deployment and Service

### Auth Injection

`ctx.fetch(service, path)` automatically injects auth headers from secrets:
- `secrets[service].token` → `Authorization: Bearer {token}`
- `secrets[service].api_key` → `X-API-Key: {api_key}`

## 8. Testing Architecture

### Go Tests (51 total)

| Package | File | Tests | Coverage |
|---------|------|-------|----------|
| `pkg/spec` | `parse_test.go` | 16 | Parser: valid spec, missing name, invalid name, cycles, edge refs, triggers, config extras, ToMap, trigger names, queue triggers |
| `pkg/builder` | `k8s_test.go` | 25 | K8s manifests: security contexts, probes, RuntimeClass, labels, volumes, resources, Dockerfile, CronJob generation/naming/labels/POST body |
| `pkg/cli` | `deploy_secrets_test.go` | 12 | Secret provisioning: dir/YAML cascade, hidden files, empty dirs, whitespace, error handling |
| `pkg/k8s` | `preflight_test.go` | 3 | CheckResultsJSON: warning omitempty, round-trip parsing |

Run: `go test ./pkg/...`

### Deno Tests (47 total)

| Module | File | Tests | Coverage |
|--------|------|-------|----------|
| `compiler` | `compiler_test.ts` | 9 | DAG compilation: single, chain, fan-out, fan-in, diamond, cycles, errors |
| `context` | `context_test.ts` | 12 | Context: fetch URL resolution, auth injection, logging, config, secrets |
| `context` | `secrets_test.ts` | 6 | Secret loading: YAML, directory, missing, hidden, invalid, plain text |
| `context` | `cascade_test.ts` | 7 | Cascade: explicit precedence, dir/YAML merge, empty, fallback, key preservation |
| `executor` | `simple_test.ts` | 7 | Execution: single, chain, failure, parallel, retry, retry exhaustion, timeout |
| `triggers` | `nats_test.ts` | 6 | NATS options validation: URL, token, triggers, subject, valid options |

Run: `deno test --allow-read --allow-write=/tmp --allow-net --allow-env` in `engine/`

### Testing Utilities

- **`engine/testing/mocks.ts`** — mock Context with stubbed fetch returning `{mock: true, service, path}`
- **`engine/testing/fixtures.ts`** — load JSON fixtures from `tests/fixtures/`
- **`engine/testing/runner.ts`** — CLI test runner for individual nodes and full pipelines

### CLI Test Command

```
pipedreamer test                      Run all pipeline tests
pipedreamer test myworkflow/fetch     Run single node test
pipedreamer test --pipeline           Run full workflow end-to-end
```

## 9. Data Flow

Trace of a workflow execution from spec to response:

```
workflow.yaml
    │
    ▼
spec.Parse()               Go CLI: validate YAML, check DAG acyclicity
    │
    ▼
compile(spec)              Deno Engine: Kahn's algorithm → topological sort → stages
    │
    ▼                      ┌─────────────────────────────────────────────┐
POST /run                  │ SimpleExecutor.execute()                    │
    │                      │                                             │
    ▼                      │   Stage 1: [fetch-repos]                   │
resolveInput()             │     → ctx.fetch("github", "/user/repos")   │
    │                      │     → output: { repos: [...] }             │
    ▼                      │                                             │
runner.run(nodeId, ctx,    │   Stage 2: [summarize]                     │
           input)          │     → input: { repos: [...] }              │
    │                      │     → output: { summary: "..." }           │
    ▼                      │                                             │
ExecutionResult            │   Stage 3: [notify]                        │
    │                      │     → input: { summary: "..." }            │
    ▼                      │     → output: { sent: true }               │
JSON Response              └─────────────────────────────────────────────┘
```

### Concrete Example: github-digest

```yaml
# workflow.yaml
name: github-digest
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch-repos:
    path: ./nodes/fetch-repos.ts
  summarize:
    path: ./nodes/summarize.ts
  notify:
    path: ./nodes/notify.ts
edges:
  - from: fetch-repos
    to: summarize
  - from: summarize
    to: notify
```

Compiles to:
- Stage 1: `[fetch-repos]` — fetches GitHub repos via `ctx.fetch("github", "/user/repos")`
- Stage 2: `[summarize]` — receives repos array, produces summary text
- Stage 3: `[notify]` — receives summary, sends notification

## 10. Infrastructure

### k0s Cluster

The target deployment environment is a k0s Kubernetes cluster — a lightweight, single-binary distribution suitable for edge and small-scale deployments.

### Container Registry

Dual-registry pattern:
- **Build registry** — external registry URL passed via `--registry` (used during `docker push`)
- **Cluster registry** — in-cluster Zot registry passed via `--cluster-registry` (used in K8s manifest image references)

This allows building and pushing from a CI machine while K8s pulls from the local Zot registry, avoiding egress traffic and improving pull latency.

### gVisor Setup

Located in `deploy/gvisor/`:
- `install.sh` — installs `runsc` and `containerd-shim-runsc-v1` on k0s nodes, configures containerd
- `runtimeclass.yaml` — K8s RuntimeClass resource (`handler: runsc`)
- `test-pod.yaml` — verification pod that runs `dmesg` to confirm gVisor kernel

Installation: `sudo bash deploy/gvisor/install.sh && kubectl apply -f deploy/gvisor/runtimeclass.yaml`

Preflight check: `pipedreamer cluster check` validates RuntimeClass exists (warning if missing, not a hard failure).

## 11. Extension Points

### Adding a CLI Command

1. Create `pkg/cli/mycommand.go` with `NewMyCommandCmd() *cobra.Command`
2. Register in `cmd/pipedreamer/main.go`: `root.AddCommand(cli.NewMyCommandCmd())`

### Adding a Workflow Node

1. Create `nodes/my-node.ts` exporting `default async function run(ctx: Context, input: T): Promise<U>`
2. Add node to `workflow.yaml` under `nodes:`
3. Add edges to connect it in the DAG
4. Create `tests/fixtures/my-node.json` with `{input, expected}` for testing

### Adding a Trigger Type

1. Add type name to `validTriggerTypes` in `pkg/spec/parse.go`
2. Add validation logic (e.g., required fields) in the trigger validation loop
3. Add fields to `Trigger` struct in `pkg/spec/types.go` and `engine/types.ts`
4. Implement trigger handling: K8s resource generation in `pkg/builder/k8s.go` (for external triggers like cron) or engine subscription in `engine/triggers/` (for in-process triggers like NATS queue)

### Adding an Executor Implementation

1. Implement the `WorkflowExecutor` interface in `engine/executor/`:
   ```typescript
   interface WorkflowExecutor {
     execute(graph: CompiledDAG, runner: NodeRunner, ctx: Context, input?: unknown): Promise<ExecutionResult>;
   }
   ```
2. Wire it into `engine/server.ts` in place of `SimpleExecutor`

### Adding a Preflight Check

1. Add check logic in `pkg/k8s/preflight.go` within `PreflightCheck()`
2. Return a `CheckResult` with Name, Passed, and optional Warning/Remediation
3. If auto-fixable, add fix logic gated by the `autoFix` parameter
