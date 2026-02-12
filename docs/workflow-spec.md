# Workflow Specification

Workflows are defined in `workflow.yaml` at the root of each workflow directory.

## Format

```yaml
name: my-workflow        # kebab-case, required
version: "1.0"           # semver, required
description: "What it does"

triggers:
  - type: manual         # manual | cron | queue
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

deployment:                          # optional
  namespace: pd-my-workflow          # target K8s namespace
```

## Deployment Section

The optional `deployment` block configures deployment-specific settings.

| Field | Type | Description |
|-------|------|-------------|
| `deployment.namespace` | string | Target Kubernetes namespace for this workflow |

Namespace resolution order: CLI `-n` flag > `workflow.yaml deployment.namespace` > config file default > `default`.

```yaml
deployment:
  namespace: pd-cluster-health
```

## Execution Model

Nodes within the same execution stage run in parallel. Stages execute sequentially based on the topological sort of the DAG.

For example, the spec above compiles to:
- **Stage 1:** `[fetch-data]`
- **Stage 2:** `[process]`
- **Stage 3:** `[notify]`

See [architecture.md](architecture.md) for details on the compilation pipeline and execution model.
