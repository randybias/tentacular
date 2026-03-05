<p align="center">
  <img src="assets/banner.png" alt="Tentacular — Security-First Workflow Engine for Kubernetes" width="100%">
</p>

# Tentacular

Tentacular is a secure workflow build and execution system for AI agents. Instead of long-lived monolithic automation stacks or generic node catalogs, you build purpose-fit TypeScript DAG workflows for each job, then iterate or replace them quickly.

It runs those workflows on Kubernetes with defense-in-depth sandboxing: distroless runtime images, Deno permission locking, hardened pod security context, secrets as mounted files (not env vars), and optional gVisor kernel isolation.

Three components form the system: a Go CLI manages the full lifecycle, an in-cluster MCP server proxies all cluster operations through scoped RBAC, and a Deno engine executes workflow DAGs inside hardened containers.

## Ecosystem

| Repository | Purpose |
|------------|---------|
| [tentacular](https://github.com/randybias/tentacular) | Go CLI (`tntc`) + Deno workflow engine |
| [tentacular-mcp](https://github.com/randybias/tentacular-mcp) | In-cluster MCP server (Helm chart, 32 tools) |
| [tentacular-skill](https://github.com/randybias/tentacular-skill) | Agent skill definition for AI assistants |
| [tentacular-catalog](https://github.com/randybias/tentacular-catalog) | Workflow template catalog + GitHub Pages site |

## Overview

![System Architecture](docs/diagrams/system-architecture.svg)

All CLI commands that interact with the cluster route through the MCP server via HTTP. The MCP server is installed separately using its Helm chart (`helm install tentacular-mcp`).

## Features

- **DAG-based workflows** — define multi-step pipelines as TypeScript functions connected by edges
- **Five-layer security** — distroless containers, Deno permission locking, gVisor kernel isolation, K8s SecurityContext, secrets-as-volumes
- **Local development** — hot-reload dev server with `tntc dev`
- **Fixture-based testing** — test individual nodes or full pipelines against JSON fixtures
- **One-command deploy** — build, push, and deploy to Kubernetes with automatic secret provisioning
- **No kubectl required** — full operational lifecycle (deploy, status, run, logs, undeploy) through the CLI via in-cluster MCP server
- **Runtime telemetry** — in-memory event tracking with `GET /health?detail=1` for execution telemetry snapshots, used by MCP health tools for G/A/R classification

## Prerequisites

- [Go](https://go.dev/dl/) 1.22+ — build the CLI
- [Deno](https://deno.land/) 2.x — execute workflow engine locally, run tests
- [Docker](https://docs.docker.com/get-docker/) 20+ — build container images
- [kubectl](https://kubernetes.io/docs/tasks/tools/) 1.28+ — Kubernetes cluster access
- A Kubernetes cluster 1.28+ as deployment target
- **Optional:** [gVisor](https://gvisor.dev/) on cluster nodes for kernel-level sandboxing (see [docs/gvisor-setup.md](docs/gvisor-setup.md))

## Installation

### Recommended

```bash
curl -fsSL https://raw.githubusercontent.com/randybias/tentacular/main/install.sh | sh
```

This installs the `tntc` binary and the Deno engine to `~/.local/bin` and `~/.tentacular/engine`.

### Build from Source

```bash
git clone git@github.com:randybias/tentacular.git
cd tentacular
make install        # builds with version info and installs to ~/.local/bin/
tntc version        # verify
```

> **Note:** `make build-cli` (or `make install`) embeds version, commit, and build date via
> ldflags. A bare `go build ./cmd/tntc` produces a dev build with `version=dev`.

## Quick Start

### 1. Scaffold a new workflow

```bash
tntc init my-workflow
```

### 2. Validate and test locally

```bash
tntc validate my-workflow
tntc test my-workflow
```

### 3. Run the dev server

```bash
tntc dev
# POST http://localhost:8080/run to trigger, GET /health to check
```

### 4. Install the MCP server (one-time per cluster)

```bash
# Clone the MCP server repo
git clone git@github.com:randybias/tentacular-mcp.git

# Generate a token and install via Helm
TOKEN=$(openssl rand -hex 32)
kubectl create namespace tentacular-support
helm install tentacular-mcp ./tentacular-mcp/charts/tentacular-mcp \
  --namespace tentacular-system --create-namespace \
  --set auth.token="${TOKEN}"
```

See the [tentacular-mcp README](https://github.com/randybias/tentacular-mcp) for Helm values and configuration options.

### 5. Configure the CLI

```bash
tntc configure --registry registry.example.com
# Add MCP endpoint and token to ~/.tentacular/config.yaml
```

### 6. Build and deploy

```bash
tntc build my-workflow -r registry.example.com --push
tntc deploy my-workflow -n my-namespace -r registry.example.com
```

### 7. Operate

```bash
tntc status my-workflow -n my-namespace
tntc run my-workflow -n my-namespace
tntc logs my-workflow -n my-namespace
tntc undeploy my-workflow -n my-namespace
```

## Node Contract

Every node is a TypeScript file with a single default export:

```typescript
import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const resp = await ctx.fetch("github", "/user/repos");
  ctx.log.info("Fetched repos");
  return { repos: await resp.json() };
}
```

See [docs/node-contract.md](docs/node-contract.md) for the full Context API, auth injection, and testing fixtures.

## Template Catalog

Production-ready workflow templates are available in the [tentacular-catalog](https://github.com/randybias/tentacular-catalog):

```bash
# Browse available templates
tntc catalog list
tntc catalog search monitoring
tntc catalog info hn-digest

# Scaffold from a template
tntc catalog init hn-digest my-news-digest
cd my-news-digest
tntc validate
tntc dev
```

| Template | Description | Complexity |
|----------|-------------|-----------|
| `word-counter` | Simple word counting example | simple |
| `hn-digest` | Fetch and filter top Hacker News stories | moderate |
| `uptime-prober` | Probe HTTP endpoints on cron, alert to Slack | moderate |
| `github-digest` | Fetch GitHub repos and create a summary digest | moderate |
| `pr-review` | Automated PR review with parallel security scans | advanced |
| `cluster-health-collector` | Collect K8s cluster health, store to Postgres | moderate |

See `tntc catalog list` for the full catalog.

## Architecture

| Directory | Purpose |
|-----------|---------|
| `cmd/tntc/` | CLI entry point |
| `pkg/` | Go packages: spec parser, builder, MCP client, CLI commands |
| `engine/` | Deno TypeScript engine: compiler, executor, context, server, telemetry |
| `pkg/catalog/` | Catalog client for fetching workflow templates |
| `deploy/` | Infrastructure scripts (gVisor installation, RuntimeClass) |
| `docs/` | Reference documentation |

### Namespace Model

| Namespace | Purpose | Protection |
|-----------|---------|------------|
| `tentacular-system` | MCP server, control plane, cron scheduler | Protected from deletion by self-guard |
| `tentacular-support` | esm.sh module proxy, caches jsr/npm modules | Protected from deletion by self-guard |
| Workflow namespaces | One per deployment, `managed-by: tentacular` label | Created/deleted via MCP tools |

Workflow pods run in their own namespaces with default-deny NetworkPolicy and contract-derived egress rules. They never run inside `tentacular-system`. See [ESM Module Proxy](docs/esm-module-proxy.md) for the module proxy architecture.

### Infrastructure Setup

![System Bootstrapping](docs/diagrams/namespace-bootstrapping.svg)

Infrastructure is created in three stages:

1. **Helm install** creates the `tentacular-system` namespace with the MCP server Deployment, ServiceAccount, ClusterRole/Binding, auth Secret, and Service. The `tentacular-support` namespace must be pre-created (`kubectl create namespace tentacular-support`).

2. **MCP server startup** triggers the proxy reconciler, which auto-creates the esm.sh Deployment and Service inside `tentacular-support`. The reconciler re-checks every 5 minutes.

3. **`tntc deploy`** (on-demand) calls `ns_create` to create a workflow namespace with default-deny NetworkPolicy, DNS-allow policy, ResourceQuota, LimitRange, and workflow ServiceAccount/Role/RoleBinding. Then `wf_apply` adds the workflow-specific ConfigMap, Deployment, Service, Secret, and contract-derived NetworkPolicy.

### Security Model

Five layers of defense-in-depth, from innermost to outermost:

1. **Distroless base image** — no shell, no package manager, minimal attack surface
2. **Deno permission locking** — `--allow-net`, `--allow-read=/app`, `--allow-write=/tmp` only
3. **gVisor sandbox** — kernel-level syscall interception via `runtimeClassName: gvisor`
4. **K8s SecurityContext** — `runAsNonRoot`, `readOnlyRootFilesystem`, `drop: ALL` capabilities
5. **Secrets as volumes** — never environment variables, mounted read-only at `/app/secrets`

**Execution Model:** All nodes in a workflow execute within a single Deno process and share memory. Stages run sequentially while nodes within each stage run concurrently via async/await. Isolation is provided at the pod level through gVisor's syscall interception, Deno's permission controls, and Kubernetes SecurityContext hardening. This single-process design prioritizes simplicity and performance while maintaining strong container-level security boundaries.

See [docs/architecture.md](docs/architecture.md) for the full architecture reference including data flow, execution model, and extension points.

## Documentation

| Document | Content |
|----------|---------|
| [Architecture](docs/architecture.md) | System design, data flow, execution model, extension points |
| [CLI Reference](docs/cli.md) | Commands, flags, and usage examples |
| [Workflow Spec](docs/workflow-spec.md) | workflow.yaml format and field reference |
| [Node Contract](docs/node-contract.md) | Context API, auth injection, testing fixtures |
| [Secrets](docs/secrets.md) | Local and production secrets management |
| [Testing](docs/testing.md) | Go, Deno, and workflow test commands |
| [gVisor Setup](docs/gvisor-setup.md) | gVisor installation and verification |
| [Roadmap](docs/roadmap.md) | Project roadmap and future plans |

## License

Proprietary. Copyright Mirantis, Inc. All rights reserved. See LICENSE.
