# Pipedreamer v2

Pipedreamer is a workflow execution engine that runs TypeScript DAGs on Kubernetes with defense-in-depth sandboxing. You define workflows as directed acyclic graphs of TypeScript functions, and Pipedreamer handles compilation, local development, container packaging, and Kubernetes deployment with gVisor kernel isolation.

A Go CLI manages the full lifecycle while a Deno engine executes workflow DAGs inside hardened containers.

## Table of Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [Workflow Specification](#workflow-specification)
- [Node Contract](#node-contract)
- [Secrets Management](#secrets-management)
- [Examples](#examples)
- [Architecture](#architecture)
- [Testing](#testing)
- [License](#license)

## Features

- **DAG-based workflows** — define multi-step pipelines as TypeScript functions connected by edges
- **Five-layer security** — distroless containers, Deno permission locking, gVisor kernel isolation, K8s SecurityContext, secrets-as-volumes
- **Local development** — hot-reload dev server with `pipedreamer dev`
- **Fixture-based testing** — test individual nodes or full pipelines against JSON fixtures
- **One-command deploy** — build, push, and deploy to Kubernetes with automatic secret provisioning
- **No kubectl required** — full operational lifecycle (deploy, status, run, logs, undeploy) through the CLI

## Prerequisites

| Tool | Minimum Version | Purpose |
|------|----------------|---------|
| [Go](https://go.dev/dl/) | 1.22+ | Build the CLI |
| [Deno](https://deno.land/) | 2.x | Execute workflow engine locally, run tests |
| [Docker](https://docs.docker.com/get-docker/) | 20+ | Build container images |
| [kubectl](https://kubernetes.io/docs/tasks/tools/) | 1.28+ | Kubernetes cluster access |
| A Kubernetes cluster | 1.28+ | Deployment target |

**Optional but recommended:**

- [gVisor](https://gvisor.dev/) installed on cluster nodes — provides kernel-level sandboxing (see `deploy/gvisor/`)
- A container registry accessible from your cluster (e.g., Zot, Docker Hub, GHCR)

### Installing Deno

```bash
curl -fsSL https://deno.land/install.sh | sh
# Add to PATH: export PATH="$HOME/.deno/bin:$PATH"
```

### Verifying prerequisites

```bash
go version          # go1.22+ required
deno --version      # 2.x required
docker --version    # 20+ required
kubectl version --client
```

## Installation

```bash
# Clone the repository
git clone git@github.com:randybias/pipedreamer2.git
cd pipedreamer2

# Build the CLI
go build -o pipedreamer ./cmd/pipedreamer/

# Verify
./pipedreamer --help
```

To make the CLI available system-wide:

```bash
sudo cp pipedreamer /usr/local/bin/
```

## Quick Start

This walkthrough creates a workflow, tests it locally, then deploys to Kubernetes.

### 1. Scaffold a new workflow

```bash
pipedreamer init my-workflow
cd my-workflow
```

This creates:
```
my-workflow/
  workflow.yaml          — workflow definition
  nodes/hello.ts         — example node
  tests/fixtures/        — test fixtures
  .secrets.yaml.example  — secrets template
```

### 2. Validate and test locally

```bash
pipedreamer validate
pipedreamer test
```

### 3. Run the dev server

```bash
pipedreamer dev
# Starts on http://localhost:8080 with hot-reload
# POST /run to trigger, GET /health to check
```

In another terminal:
```bash
curl -s http://localhost:8080/run | jq .
```

### 4. Build the container image

```bash
# Build locally
pipedreamer build

# Build and push to a registry
pipedreamer build -r registry.example.com --push
```

### 5. Deploy to Kubernetes

```bash
# Verify cluster readiness (auto-create namespace with --fix)
pipedreamer cluster check -n my-namespace --fix

# Deploy (use --cluster-registry if your cluster pulls from an internal registry)
pipedreamer deploy -n my-namespace -r registry.example.com
```

### 6. Operate

```bash
pipedreamer status my-workflow -n my-namespace
pipedreamer run my-workflow -n my-namespace
pipedreamer logs my-workflow -n my-namespace
pipedreamer list -n my-namespace
pipedreamer undeploy my-workflow -n my-namespace
```

## CLI Reference

### Workflow Lifecycle

| Command | Usage | Description |
|---------|-------|-------------|
| `init` | `pipedreamer init <name>` | Scaffold a new workflow directory |
| `validate` | `pipedreamer validate [dir]` | Validate workflow.yaml (DAG acyclicity, naming, edges) |
| `dev` | `pipedreamer dev [dir]` | Local dev server with hot-reload |
| `test` | `pipedreamer test [dir][/<node>]` | Run node or pipeline tests against fixtures |
| `build` | `pipedreamer build [dir]` | Build container image (distroless Deno base) |
| `deploy` | `pipedreamer deploy [dir]` | Generate K8s manifests and apply to cluster |
| `visualize` | `pipedreamer visualize [dir]` | Generate Mermaid diagram of the workflow DAG |

### Operations

| Command | Usage | Description |
|---------|-------|-------------|
| `status` | `pipedreamer status <name>` | Check deployment readiness; `--detail` for extended info |
| `run` | `pipedreamer run <name>` | Trigger a deployed workflow, return JSON result |
| `logs` | `pipedreamer logs <name>` | View pod logs; `-f` to stream in real time |
| `list` | `pipedreamer list` | List all deployed workflows in a namespace |
| `undeploy` | `pipedreamer undeploy <name>` | Remove deployed workflow (Deployment, Service, Secret) |
| `cluster check` | `pipedreamer cluster check` | Preflight cluster validation; `--fix` auto-remediates |

### Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | `default` | Kubernetes namespace |
| `--registry` | `-r` | (none) | Container registry URL |
| `--output` | `-o` | `text` | Output format: `text` or `json` |

### Key Command Flags

```bash
# Build
pipedreamer build -t custom:tag           # custom image tag
pipedreamer build -r reg.io --push        # build and push
pipedreamer build --platform linux/arm64  # cross-platform build

# Deploy
pipedreamer deploy --cluster-registry registry.svc.cluster.local:5000
pipedreamer deploy --runtime-class gvisor  # default; use "" to disable

# Test
pipedreamer test my-workflow/fetch-data   # test single node
pipedreamer test --pipeline               # full end-to-end pipeline test

# Logs
pipedreamer logs my-workflow --tail 50    # last 50 lines
pipedreamer logs my-workflow -f           # stream live

# Run
pipedreamer run my-workflow --timeout 60s

# Undeploy
pipedreamer undeploy my-workflow --yes    # skip confirmation
```

## Workflow Specification

Workflows are defined in `workflow.yaml`:

```yaml
name: my-workflow        # kebab-case, required
version: "1.0"           # semver, required
description: "What it does"

triggers:
  - type: manual         # manual | cron
  - type: cron
    schedule: "0 9 * * *"

nodes:
  fetch-data:
    path: ./nodes/fetch-data.ts
    capabilities:
      net: "github.com"
  process:
    path: ./nodes/process.ts
  notify:
    path: ./nodes/notify.ts
    capabilities:
      net: "slack.com"

edges:
  - from: fetch-data
    to: process
  - from: process
    to: notify

config:
  timeout: 30s
  retries: 1
```

Nodes within the same execution stage run in parallel. Stages execute sequentially based on the topological sort of the DAG.

## Node Contract

Every node is a TypeScript file with a single default export:

```typescript
import type { Context } from "pipedreamer";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const resp = await ctx.fetch("github", "/user/repos");
  ctx.log.info("Fetched repos");
  return { repos: await resp.json() };
}
```

### Context API

| Member | Type | Description |
|--------|------|-------------|
| `ctx.fetch(service, path, init?)` | `Promise<Response>` | HTTP request to `https://api.<service>.com<path>` with automatic auth injection from secrets |
| `ctx.log` | `Logger` | Structured logging (`info`, `warn`, `error`, `debug`) prefixed with `[nodeId]` |
| `ctx.config` | `Record<string, unknown>` | Workflow-level config from `config:` in workflow.yaml |
| `ctx.secrets` | `Record<string, Record<string, string>>` | Secrets keyed by service name |

Auth injection: `ctx.fetch` automatically adds `Authorization: Bearer <token>` or `X-API-Key: <api_key>` based on matching secrets.

### Testing Nodes

Create a JSON fixture at `tests/fixtures/<node-name>.json`:

```json
{
  "input": { "query": "test" },
  "expected": { "results": [] }
}
```

Run with `pipedreamer test` (all nodes) or `pipedreamer test my-workflow/node-name` (single node).

## Secrets Management

### Local Development

Copy the generated template and fill in values:

```bash
cp .secrets.yaml.example .secrets.yaml
```

```yaml
# .secrets.yaml (gitignored)
github:
  token: "ghp_..."
slack:
  webhook_url: "https://hooks.slack.com/services/..."
anthropic:
  api_key: "sk-ant-..."
```

The engine loads `.secrets.yaml` at startup. Values are available via `ctx.secrets` and used for `ctx.fetch` auth injection.

### Production (Kubernetes)

`pipedreamer deploy` automatically provisions secrets to Kubernetes from:
1. `.secrets/` directory (files as secret entries), or
2. `.secrets.yaml` file (YAML keys as secret entries)

The K8s Secret is mounted read-only at `/app/secrets` inside the container. Secrets are **never** exposed as environment variables.

To manage secrets manually:

```bash
kubectl create secret generic my-workflow-secrets \
  -n my-namespace \
  --from-file=github=./github-token.json \
  --from-file=slack=./slack-config.json
```

Convention: secrets are named `<workflow-name>-secrets`.

## Examples

Three working examples in `examples/`:

| Example | Description | Secrets Required |
|---------|-------------|-----------------|
| `hn-digest` | Fetch and filter top Hacker News stories | None |
| `github-digest` | Fetch GitHub repos and create a summary digest | GitHub token, Slack webhook |
| `pr-digest` | Summarize PRs with Claude and send to Slack | GitHub token, Anthropic API key, Slack webhook |

```bash
# Try the no-secrets example
pipedreamer validate examples/hn-digest
pipedreamer test examples/hn-digest
pipedreamer dev examples/hn-digest

# Visualize the DAG
pipedreamer visualize examples/github-digest
```

## Architecture

```
pipedreamer
├── init <name>         Scaffold new workflow project
├── validate [dir]      Validate workflow.yaml spec
├── dev [dir]           Local dev server with hot-reload
├── test [dir/node]     Run node or pipeline tests
├── build [dir]         Build container image
├── deploy [dir]        Deploy to Kubernetes
├── status <name>       Check deployment health
├── run <name>          Trigger a deployed workflow
├── logs <name>         View workflow pod logs
├── list                List deployed workflows
├── undeploy <name>     Remove a deployed workflow
├── cluster check       Preflight cluster validation
└── visualize [dir]     Generate Mermaid DAG diagram
```

### Project Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/pipedreamer/` | CLI entry point |
| `pkg/` | Go packages: spec parser, builder, K8s client, CLI commands |
| `engine/` | Deno TypeScript engine: compiler, executor, context, server |
| `examples/` | Runnable example workflows |
| `deploy/` | Infrastructure scripts (gVisor installation, RuntimeClass) |
| `docs/` | Detailed architecture reference |

### Security Model

Five layers of defense-in-depth, from innermost to outermost:

1. **Distroless base image** — no shell, no package manager, minimal attack surface
2. **Deno permission locking** — `--allow-net`, `--allow-read=/app`, `--allow-write=/tmp` only
3. **gVisor sandbox** — kernel-level syscall interception via `runtimeClassName: gvisor`
4. **K8s SecurityContext** — `runAsNonRoot`, `readOnlyRootFilesystem`, `drop: ALL` capabilities
5. **Secrets as volumes** — never environment variables, mounted read-only at `/app/secrets`

See [docs/architecture.md](docs/architecture.md) for the full architecture reference including data flow, execution model, and extension points.

### gVisor Setup

For clusters without gVisor:

```bash
# Install on k0s nodes
sudo bash deploy/gvisor/install.sh

# Apply the RuntimeClass
kubectl apply -f deploy/gvisor/runtimeclass.yaml

# Verify
kubectl apply -f deploy/gvisor/test-pod.yaml
kubectl logs gvisor-test
```

gVisor is recommended but optional. Use `--runtime-class ""` with `deploy` to skip it.

## Testing

### Go tests

```bash
go test ./pkg/...
```

Covers spec parsing, K8s manifest generation, secret provisioning, and preflight checks.

### Deno engine tests

```bash
cd engine && deno test --allow-read --allow-write=/tmp --allow-net --allow-env
```

Covers DAG compilation, context/fetch/auth injection, secrets cascade, and executor behavior (retry, timeout, parallel stages).

### Workflow tests

```bash
pipedreamer test                      # all node fixtures
pipedreamer test my-workflow/fetch    # single node
pipedreamer test --pipeline           # full DAG end-to-end
```

## License

Copyright Mirantis, Inc. All rights reserved.
