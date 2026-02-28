# Tentacular Architecture

## 1. System Overview

Tentacular is a workflow execution platform that runs TypeScript DAGs on Kubernetes with defense-in-depth sandboxing. Three components form the system: a Go CLI manages the full lifecycle, an in-cluster MCP server proxies all cluster operations through scoped RBAC, and a Deno engine executes workflow DAGs inside hardened containers with gVisor kernel isolation.

```
          Developer Machine                          Kubernetes Cluster
     ┌──────────────────────────┐     ┌──────────────────────────────────────────────┐
     │                          │     │  tentacular-system namespace                 │
     │  tntc CLI (Go)           │     │  ┌──────────────────────────────────────┐    │
     │  ┌────────────────────┐  │     │  │  tentacular-mcp (MCP Server)        │    │
     │  │ init / validate    │  │ MCP │  │  Bearer auth, scoped RBAC           │    │
     │  │ dev / test         │  │────>│  │  Streamable HTTP on :8080/mcp       │    │
     │  │ build / deploy     │  │     │  └──────────┬───────────────────────────┘    │
     │  │ status / cluster   │  │     │             │ K8s API                        │
     │  │ visualize          │  │     │             v                                │
     │  └────────────────────┘  │     │  ┌──────────────────────────┐                │
     │           │              │     │  │  Pod (gVisor sandbox)    │                │
     │      ┌────┴────┐        │     │  │  ┌──────────────────┐    │                │
     │      │ Docker   │        │     │  │  │ Deno Engine (TS) │    │                │
     │      │ Build    │        │     │  │  │ ┌──────────────┐ │    │                │
     │      └─────────┘        │     │  │  │ │ Workflow DAG │ │    │                │
     │                          │     │  │  │ └──────────────┘ │    │                │
     └──────────────────────────┘     │  │  └──────────────────┘    │                │
                                      │  │  /app/workflow (CM)      │                │
                                      │  │  /app/secrets (vol)      │                │
                                      │  └──────────────────────────┘                │
                                      │  ConfigMap, Secret, NetworkPolicy            │
                                      └──────────────────────────────────────────────┘
```

**CLI-to-MCP Handoff:** `tntc cluster install` is the only CLI command that communicates directly with the Kubernetes API. It bootstraps the MCP server, generates a bearer token, and saves the MCP endpoint and token to `~/.tentacular/config.yaml`. All subsequent cluster-facing commands (deploy, run, list, status, logs, undeploy, audit, cluster check) route through the MCP server using JSON-RPC 2.0 over Streamable HTTP.

| Directory | Purpose |
|-----------|---------|
| `cmd/tntc/` | CLI entry point (Cobra command tree) |
| `pkg/` | Go packages: spec parser, builder, K8s client, CLI commands |
| `engine/` | Deno TypeScript engine: compiler, executor, context, server, telemetry |
| `example-workflows/` | Runnable example workflows |
| `deploy/` | Infrastructure scripts (gVisor installation, RuntimeClass) |
| `openspec/` | Change tracking and specifications |
| `docs/` | Project documentation |

### Execution Isolation Model

Tentacular executes all nodes in a workflow within a **single Deno process**. This architectural decision prioritizes simplicity and performance while maintaining strong pod-level security boundaries.

**Process Model:**
- All nodes share the same Deno runtime and memory space
- Parallelism achieved via async/await and Promise.all(), not separate processes
- One compromised node = entire workflow process accessible
- Isolation provided at the **pod level**, not per-node

**Design Rationale:**
- Direct TypeScript execution with full Deno ecosystem access
- Simplified debugging and development workflow
- Lower overhead than inter-process communication or serialization
- Pod-level isolation via gVisor sufficient for trusted workflow code

**Security Boundaries (outer to inner):**
1. **Pod-level:** gVisor syscall interception prevents container escape
2. **Container-level:** Kubernetes SecurityContext (non-root, read-only filesystem, dropped capabilities)
3. **Runtime-level:** Deno permission locking (allow-list for network, filesystem, write)
4. **Cluster-level:** Network policies, RBAC, and namespace isolation

This design assumes workflow code is authored by trusted developers. For multi-tenant scenarios where untrusted code must execute, additional isolation layers (separate pods, namespaces, or clusters) should be added at the orchestration level.

## 2. Go CLI Architecture

### Command Tree

```
tntc
├── init <name>         Scaffold new workflow project
├── validate [dir]      Validate workflow.yaml spec
├── dev [dir]           Local dev server with hot-reload
├── test [dir/node]     Run node or pipeline tests
├── build [dir]         Build container image
├── deploy [dir]        Deploy to Kubernetes (via MCP)
├── status <name>       Check deployment health (via MCP, --detail for extended info)
├── run <name>          Trigger a deployed workflow (via MCP)
├── logs <name>         View workflow pod logs (via MCP, snapshot only)
├── list                List deployed workflows (via MCP)
├── undeploy <name>     Remove a deployed workflow (via MCP)
├── audit <name>        Run security audit (via MCP: RBAC, netpol, PSA)
├── cluster install     Bootstrap MCP server and module proxy (direct K8s API)
├── cluster check       Preflight cluster validation (via MCP)
└── visualize [dir]     Generate Mermaid DAG diagram
```

Global flags: `--namespace`, `--registry`, `--output` (text|json)

Commands marked "(via MCP)" require a running MCP server. Run `tntc cluster install` first to bootstrap. The `logs` command returns a snapshot of recent lines; real-time streaming (`--follow`) is not supported through MCP.

### Package Layout

```
pkg/
├── spec/           Workflow YAML parser + validator
│   ├── types.go        Workflow, NodeSpec, Edge, Trigger, WorkflowConfig
│   └── parse.go        Parse(), checkCycles() — kebab-case, semver, DAG validation
├── builder/        Artifact generation (no runtime deps)
│   ├── dockerfile.go   GenerateDockerfile() → engine-only distroless Deno container
│   └── k8s.go          GenerateK8sManifests() → Deployment + Service YAML
├── cli/            Cobra command implementations
│   ├── init.go         Scaffold: workflow.yaml, nodes/hello.ts, fixtures
│   ├── validate.go     Parse + report errors
│   ├── dev.go          Spawn Deno engine with --watch, graceful shutdown
│   ├── test.go         Run Deno test runner against fixtures
│   ├── build.go        Docker build with engine copy, tag derivation
│   ├── deploy.go       Generate manifests, provision secrets, deploy via MCP
│   ├── status.go       Query deployment via MCP wf_status (--detail for extended info)
│   ├── run.go          Trigger deployed workflow via MCP wf_run
│   ├── logs.go         Tail pod logs via MCP wf_logs (snapshot, not streaming)
│   ├── list.go         List deployed workflows via MCP wf_list
│   ├── undeploy.go     Remove deployed workflow via MCP wf_remove
│   ├── audit.go        Security audit via MCP audit_rbac, audit_netpol, audit_psa
│   ├── cluster.go      cluster install (bootstrap) + cluster check (via MCP)
│   ├── resolve.go      resolveMCPClient(), requireMCPClient(), mcpErrorHint()
│   └── visualize.go    Mermaid graph output
├── mcp/            MCP client (JSON-RPC 2.0 over Streamable HTTP)
│   ├── client.go       Client struct, CallTool(), Ping()
│   ├── tools.go        Typed tool methods: WfApply, WfRemove, WfStatus, WfList,
│   │                   WfLogs, WfRun, ClusterPreflight, NsCreate, AuditResources
│   └── auth.go         Config resolution, LoadConfigFromCluster(), SaveConfig()
└── k8s/            Kubernetes client operations (bootstrap only)
    ├── client.go       NewClient(), Apply() — used only by cluster install
    ├── mcp_deploy.go   GenerateMCPServerManifests(), MCPEndpointInCluster()
    ├── mcp_token.go    Token generation for MCP auth
    └── preflight.go    PreflightCheck() — legacy, now proxied via MCP
```

### Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing for workflow specs and secrets
- `k8s.io/client-go` — K8s API client (used only by `cluster install` bootstrap)
- `k8s.io/apimachinery` — K8s types and API machinery
- `net/http` — MCP client transport (JSON-RPC 2.0 over HTTP)

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
10. Create TelemetrySink from TELEMETRY_SINK env var ("basic" default, "noop" for noop)
11. Start HTTP server on configured port (passes sink for /health?detail=1)
12. Start NATS triggers if queue triggers defined (dynamic import, passes sink)
13. Register signal handlers (SIGTERM/SIGINT) for graceful shutdown
14. (Optional) Start file watcher for hot-reload
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
- **Telemetry** — `node-start`, `node-complete`, and `node-error` events fire synchronously (fire-and-forget) into the TelemetrySink on each node execution

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
    "tentacular": "./mod.ts",
    "std/": "https://deno.land/std@0.224.0/"
  }
}
```

Nodes import types via: `import type { Context } from "tentacular"`

## 4. Triggers

Triggers define how workflow execution is initiated. Each workflow specifies one or more triggers in its `triggers:` array.

| Type | Mechanism | Required Fields | K8s Resources | Status |
|------|-----------|----------------|---------------|--------|
| `manual` | MCP server POSTs via K8s API service proxy | none | — | Implemented |
| `cron` | MCP server internal scheduler reads annotation, calls wf_run | `schedule`, optional `name` | Deployment annotation | Implemented |
| `queue` | NATS subscription → execute | `subject` | — | Implemented |
| `webhook` | Future: gateway → NATS bridge | `path` | — | Roadmap |

### Named Triggers

Triggers can have an optional `name` field for parameterized execution. The MCP
server's internal cron scheduler POSTs `{"trigger": "<name>"}` to `/run`, and
root nodes receive this as input. This supports workflows with multiple cron
schedules that branch behavior based on `input.trigger`.

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

Cron trigger schedules are stored in a `tentacular.dev/cron-schedule` annotation
on the Deployment during `tntc deploy`. No CronJob resources are created.

The MCP server's internal cron scheduler (`robfig/cron/v3`) reads this annotation
on startup and after each `wf_apply`/`wf_remove`. It fires `wf_run` internally
on schedule via the K8s API service proxy -- no ephemeral pods are created.

- **Annotation**: `tentacular.dev/cron-schedule` on the Deployment
- **Named trigger**: scheduler POSTs `{"trigger": "<name>"}` to `/run`
- **Cleanup**: `tntc undeploy` removes the Deployment and annotation; cron entries
  are dropped automatically on the MCP server's next sync

### Queue Triggers (NATS)

Queue triggers subscribe to NATS subjects. Messages trigger workflow execution with the message payload as input.

- **Connection**: TLS + token auth via `config.nats_url` and `secrets.nats.token`
- **Dynamic import**: NATS library only loaded when queue triggers exist
- **Request-reply**: If a message has a reply subject, the execution result is sent back
- **Graceful shutdown**: SIGTERM/SIGINT drain NATS subscriptions before exit
- **Degradation**: Engine warns and skips if `nats_url` or `nats.token` is missing

### POST Body Passthrough

The `/run` endpoint parses POST body as JSON and passes it as initial input to root nodes (nodes with no incoming edges). GET requests and empty bodies default to `{}`.

### Health Endpoint

`GET /health` returns `{"status":"ok"}` unconditionally (unchanged baseline).

`GET /health?detail=1` returns a `TelemetrySnapshot` from the active TelemetrySink:

```json
{
  "totalEvents": 45,
  "errorCount": 2,
  "errorRate": 0.044,
  "uptimeMs": 120000,
  "lastError": "fetch failed: 503",
  "lastErrorAt": 1709123456789,
  "recentEvents": [
    { "type": "request-in", "timestamp": 1709123456000 },
    { "type": "node-start", "timestamp": 1709123456001, "metadata": { "nodeId": "fetch-repos" } }
  ],
  "status": "ok",
  "lastRunFailed": false,
  "inFlight": 0
}
```

This endpoint is used by the MCP server's `wf_health` tool to classify workflow health as Green/Amber/Red without log parsing.

## 5. Security Model

Five layers of defense-in-depth, from innermost to outermost:

### Layer 1: Distroless Base Image

Container uses `denoland/deno:distroless` — no shell, no package manager, no debugging tools. Attack surface is limited to the Deno runtime binary.

### Layer 2: Deno Permission Locking

The base image ENTRYPOINT uses broad Deno permission flags as a fallback:
- `--allow-net` — broad network access (fallback for workflows without a contract)
- `--allow-read=/app,/var/run/secrets` — read-only access to workflow files and secrets
- `--allow-write=/tmp` — write access only to /tmp (ephemeral scratch, limited to 512Mi)
- `--allow-env` — environment variable access for runtime configuration

**Contract-derived scoping:** When a workflow declares `contract.dependencies`, the K8s Deployment manifest overrides the ENTRYPOINT with scoped flags via `command` and `args`. The `DeriveDenoFlags()` function generates `--allow-net=<host1>:<port>,<host2>:<port>,0.0.0.0:8080`, restricting network access to only declared dependency hosts and the trigger listener port. If any dependency has `type: dynamic-target`, the function falls back to broad `--allow-net` (no host restriction) since targets are resolved at runtime. The `--allow-env` flag is scoped to `DENO_DIR,HOME` only (narrower than the ENTRYPOINT fallback). Numeric args like port `8080` are YAML-quoted to prevent integer interpretation by K8s.

No subprocess, FFI, or unrestricted file system access beyond the declared paths.

### Layer 3: gVisor Sandbox

Pods run with `runtimeClassName: gvisor`. gVisor intercepts syscalls via its application kernel (Sentry), preventing direct host kernel access. Even if a container escape is achieved, the attacker lands in gVisor's sandbox, not the host.

**Isolation scope:** gVisor provides pod-level isolation, not per-node isolation. All nodes in a workflow execute within the same Deno process and share this gVisor boundary. If one node is compromised, the attacker has access to the entire workflow's memory space, but remains isolated from the host kernel and other pods.

### Layer 4: Kubernetes SecurityContext

```yaml
automountServiceAccountToken: false  # Pod level — no SA token exposed

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

The service account token is not mounted (`automountServiceAccountToken: false`), preventing compromised pods from authenticating to the K8s API. The `/tmp` emptyDir volume has a `sizeLimit: 512Mi` to prevent disk exhaustion attacks.

### Layer 5: Secrets as Volume Mounts

Secrets are mounted as read-only files at `/app/secrets` from a K8s Secret resource (`optional: true`). They are never exposed as environment variables — env vars are visible in `kubectl describe pod`, process listings, and crash dumps.

## 6. Deployment Pipeline

### Build Phase

```
tntc build [dir]
  1. Parse and validate workflow.yaml
  2. Locate engine directory (relative to binary)
  3. Copy engine → .engine/ in build context
  4. Generate engine-only Dockerfile.tentacular (no workflow code)
  5. docker build -f Dockerfile.tentacular -t <tag> .
     Default tag: tentacular-engine:latest (override with --tag)
  6. Cleanup: remove .engine/ and Dockerfile.tentacular
  7. (Optional) docker push with --push flag
  8. Save image tag to .tentacular/base-image.txt for deploy cascade
```

### Deploy Phase (via MCP)

```
tntc deploy [dir]
  1. Parse workflow.yaml → validate spec
  2. Resolve base image tag via cascade:
     --image flag > env config image > .tentacular/base-image.txt > tentacular-engine:latest
  3. GenerateCodeConfigMap() → ConfigMap with workflow.yaml + nodes/*.ts
  4. GenerateK8sManifests() → Deployment + Service + NetworkPolicy (cron schedules stored as annotation)
  5. buildSecretManifest() → K8s Secret from .secrets/ or .secrets.yaml
  6. MCP ns_create → ensure namespace exists with PSA labels and NetworkPolicy
  7. MCP wf_apply → create-or-update all manifests via dynamic client
  8. Rollout restart triggered automatically for updates (skipped on fresh deploy)
```

### Operations Phase (via MCP)

All operations-phase commands route through the MCP server. The CLI
resolves the MCP client from config (env vars > project config > user
config) and fails with an actionable error if MCP is not configured.

```
tntc status <name>        Query Deployment readiness/replicas (MCP: wf_status)
  --detail                       Extended info: image, runtime, pods, events
tntc run <name>           Trigger workflow via MCP wf_run, return JSON result
  --timeout 30s                  Maximum wait time
tntc logs <name>          View pod logs (MCP: wf_logs, snapshot only)
  --tail N                       Number of recent lines
                                 Note: --follow not supported through MCP
tntc list                 List all tentacular-managed deployments (MCP: wf_list)
tntc undeploy <name>      Remove all deployment resources (MCP: wf_remove)
  --yes                          Skip confirmation prompt
tntc audit <name>         Security audit (MCP: audit_rbac, audit_netpol, audit_psa)
tntc cluster install      Bootstrap MCP server + module proxy (direct K8s API)
tntc cluster check        Preflight validation (MCP: cluster_preflight)
```

### Generated K8s Resources

| Resource | Name | Key Fields |
|----------|------|------------|
| ConfigMap | `{workflow-name}-code` | Contains workflow.yaml + nodes/*.ts (max 900KB). Mounted at /app/workflow |
| Deployment | `{workflow-name}` | 1 replica, gVisor RuntimeClass, code volume mount at /app/workflow, security contexts, probes, resource limits |
| Service | `{workflow-name}` | ClusterIP, port 8080 |
| Secret | `{workflow-name}-secrets` | Opaque, stringData from .secrets/ or .secrets.yaml |
| NetworkPolicy | `{workflow-name}` | Default-deny + contract-derived egress + control-plane ingress (10.0.0.0/8:8080) |

### Build and Deployment Modes

Tentacular supports two deployment workflows:

**Mode 1: Full Build + Deploy**
```bash
tntc build --push    # Creates workflow-specific image
tntc deploy          # Uses freshly built image
```
- Generates Dockerfile with engine embedded
- Builds unique image per workflow
- Image includes: Deno runtime + engine code
- Workflow code delivered via ConfigMap

**Mode 2: Deploy-Only (Fast Iteration)**
```bash
tntc deploy --image <existing-image>
```
- Reuses existing base image
- Only updates ConfigMap with code changes
- ~5-10 second deployment time
- Ideal for development iteration

Both modes use ConfigMap for workflow code delivery, enabling rapid updates without Docker rebuilds in Mode 2.

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

`tntc deploy` auto-provisions secrets to K8s:
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
| `telemetry` | `telemetry_test.ts` | 20 | NoopSink, BasicSink counters, factory modes, executor integration |
| `triggers` | `nats_test.ts` | 6 | NATS options validation: URL, token, triggers, subject, valid options |

Run: `deno test --allow-read --allow-write=/tmp --allow-net --allow-env` in `engine/`

### Testing Utilities

- **`engine/testing/mocks.ts`** — mock Context with stubbed fetch returning `{mock: true, service, path}`
- **`engine/testing/fixtures.ts`** — load JSON fixtures from `tests/fixtures/`
- **`engine/testing/runner.ts`** — CLI test runner for individual nodes and full pipelines

### CLI Test Command

```
tntc test                      Run all pipeline tests
tntc test myworkflow/fetch     Run single node test
tntc test --pipeline           Run full workflow end-to-end
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

Images are pushed to whatever external registry the user configures via `--registry` or the `registry` field in `.tentacular/config.yaml`. There is no in-cluster registry component — K8s pulls images from the configured external registry.

### gVisor Setup

Located in `deploy/gvisor/`:
- `install.sh` — installs `runsc` and `containerd-shim-runsc-v1` on k0s nodes, configures containerd
- `runtimeclass.yaml` — K8s RuntimeClass resource (`handler: runsc`)
- `test-pod.yaml` — verification pod that runs `dmesg` to confirm gVisor kernel

Installation: `sudo bash deploy/gvisor/install.sh && kubectl apply -f deploy/gvisor/runtimeclass.yaml`

Preflight check: `tntc cluster check` validates RuntimeClass exists (warning if missing, not a hard failure).

## 11. Extension Points

### Adding a CLI Command

1. Create `pkg/cli/mycommand.go` with `NewMyCommandCmd() *cobra.Command`
2. Register in `cmd/tntc/main.go`: `root.AddCommand(cli.NewMyCommandCmd())`

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

Preflight checks are now executed by the MCP server's `cluster_preflight` tool. To add a new check:

1. Add check logic in `tentacular-mcp/pkg/k8s/preflight.go`
2. Return a `CheckResult` with Name, Passed, and optional Warning/Remediation
3. The CLI invokes the check via MCP and displays results
