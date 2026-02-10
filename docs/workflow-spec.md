# Workflow Specification

Workflows are defined in `workflow.yaml` at the root of each workflow directory.

## Format

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

## Execution Model

Nodes within the same execution stage run in parallel. Stages execute sequentially based on the topological sort of the DAG.

For example, the spec above compiles to:
- **Stage 1:** `[fetch-data]`
- **Stage 2:** `[process]`
- **Stage 3:** `[notify]`

See [architecture.md](architecture.md) for details on the compilation pipeline and execution model.
