# CLI Reference

Complete reference for `tntc` commands, flags, and configuration.

## Command Reference

| Command | Usage | Key Flags | Description |
|---------|-------|-----------|-------------|
| `init` | `tntc init <name>` | | Scaffold a new workflow directory with workflow.yaml, example node, test fixture, .secrets.yaml.example |
| `validate` | `tntc validate [dir]` | | Parse and validate workflow.yaml (name, version, triggers, nodes, edges, DAG acyclicity, contract structure, derived artifacts) |
| `dev` | `tntc dev [dir]` | `-p` port (default 8080) | Start Deno engine locally with hot-reload (`--watch`). POST /run triggers execution |
| `test` | `tntc test [dir][/<node>]` | `--pipeline`, `--live`, `--env`, `--keep`, `--timeout`, `--warn` | Run node-level tests from fixtures, full pipeline test with `--pipeline`, or live cluster test with `--live`. `--warn` downgrades contract violations to warnings. |
| `build` | `tntc build [dir]` | `-t` tag, `-r` registry, `--push`, `--platform` | Generate Dockerfile (distroless Deno base), build container image via `docker build` |
| `deploy` | `tntc deploy [dir]` | `-n` namespace, `--env`, `--image`, `--runtime-class`, `--force`, `--verify`, `--warn` | Generate K8s manifests and apply to cluster. `--env` targets a named environment (context, namespace, image). Auto-gates on live test if dev env configured; `--force` skips. `--warn` downgrades contract violations to warnings. Namespace resolves: CLI > env > workflow.yaml > config > default |
| `audit` | `tntc audit <workflow-dir>` | `-n` namespace, `-o` json | Audit deployed resources against contract expectations |
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

## Global Flags

- `-n`/`--namespace` (default "default")
- `-r`/`--registry`
- `-o`/`--output` (text|json)

## Namespace Resolution

Namespace resolution order: CLI `-n` flag >
`workflow.yaml deployment.namespace` > config file
default > `default`. When `--env` is used (deploy,
test --live), environment namespace is inserted after
CLI `-n`.

## Config Files

- `~/.tentacular/config.yaml` (user-level)
- `.tentacular/config.yaml` (project-level)
- Project overrides user.

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

![sep-tracker workflow diagram](../../example-workflows/sep-tracker/workflow-diagram.png)

The sep-tracker workflow demonstrates a full contract
with four external dependencies (GitHub API, Postgres,
Azure Blob Storage, Slack webhook). The rich
visualization shows how each dependency connects to
the nodes that use it, along with derived secrets and
network policy rules.
