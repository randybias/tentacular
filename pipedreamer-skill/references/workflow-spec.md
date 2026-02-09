# Workflow Specification Reference

Complete reference for the Pipedreamer v2 `workflow.yaml` format.

## Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Workflow name. Must be kebab-case: `^[a-z][a-z0-9]*(-[a-z0-9]+)*$` |
| `version` | string | Yes | Semver format: `MAJOR.MINOR` (e.g., `"1.0"`, `"2.3"`). Regex: `^[0-9]+\.[0-9]+$` |
| `description` | string | No | Human-readable description |
| `triggers` | array | Yes | At least one trigger. Defines how the workflow is invoked. |
| `nodes` | map | Yes | At least one node. Keys are node names matching `^[a-z][a-z0-9_-]*$`. |
| `edges` | array | Yes | Data flow edges between nodes. Can be empty `[]` for single-node workflows. |
| `config` | object | No | Workflow-level configuration (timeout, retries). |

## Triggers

Each trigger has a `type` field. Additional fields depend on the type.

### manual

No additional fields. Workflow is triggered via `POST /run`.

```yaml
triggers:
  - type: manual
```

### cron

Requires `schedule` field with a cron expression.

```yaml
triggers:
  - type: cron
    schedule: "0 */6 * * *"  # every 6 hours
```

### webhook

Requires `path` field defining the HTTP endpoint path.

```yaml
triggers:
  - type: webhook
    path: /hooks/incoming
```

Multiple triggers can be combined:

```yaml
triggers:
  - type: manual
  - type: cron
    schedule: "0 9 * * 1"
  - type: webhook
    path: /hooks/deploy
```

## Nodes

Each node is a key-value entry in the `nodes` map.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | Yes | Relative path to the TypeScript file (e.g., `./nodes/fetch-data.ts`) |
| `capabilities` | map | No | Key-value pairs declaring what this node needs (informational) |

```yaml
nodes:
  fetch-data:
    path: ./nodes/fetch-data.ts
    capabilities:
      network: "github.com"
  transform:
    path: ./nodes/transform.ts
```

Node names must match `^[a-z][a-z0-9_-]*$`.

## Edges

Edges define data flow between nodes. Each edge has `from` and `to` fields referencing node names.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `from` | string | Yes | Source node name (must be defined in `nodes`) |
| `to` | string | Yes | Target node name (must be defined in `nodes`) |

```yaml
edges:
  - from: fetch-data
    to: transform
  - from: transform
    to: notify
```

### Validation Rules

1. **Reference integrity**: Both `from` and `to` must reference nodes defined in the `nodes` map.
2. **No self-loops**: `from` and `to` cannot be the same node.
3. **DAG acyclicity**: The graph formed by all edges must be a directed acyclic graph. Cycles cause a validation error.

The compiler uses Kahn's algorithm for topological sorting and groups nodes into parallel execution stages. Nodes in the same stage have no mutual dependencies and run concurrently via `Promise.all`.

## Config

Optional workflow-level configuration.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | string | `"30s"` | Per-node execution timeout. Format: `<number>s` or `<number>m` (e.g., `"30s"`, `"5m"`) |
| `retries` | number | `0` | Number of retry attempts per node on failure. Uses exponential backoff (100ms, 200ms, 400ms, ...) |

```yaml
config:
  timeout: 60s
  retries: 2
```

## Complete Annotated Example

```yaml
# Workflow name — must be kebab-case
name: github-issue-digest

# Version — semver format (MAJOR.MINOR)
version: "1.0"

# Optional human-readable description
description: "Fetches open GitHub issues, summarizes them, and posts to Slack"

# How the workflow is triggered — at least one required
triggers:
  - type: manual                    # can be triggered via POST /run
  - type: cron
    schedule: "0 9 * * 1"          # every Monday at 9 AM

# Node definitions — each maps a name to a TypeScript file
nodes:
  fetch-issues:
    path: ./nodes/fetch-issues.ts
    capabilities:
      network: "api.github.com"     # informational: declares external access

  summarize:
    path: ./nodes/summarize.ts      # pure transform, no capabilities needed

  post-slack:
    path: ./nodes/post-slack.ts
    capabilities:
      network: "hooks.slack.com"

# Data flow between nodes — must form a DAG (no cycles)
edges:
  - from: fetch-issues              # fetch-issues output -> summarize input
    to: summarize
  - from: summarize                 # summarize output -> post-slack input
    to: post-slack

# Workflow-level configuration
config:
  timeout: 60s                      # per-node timeout
  retries: 1                        # retry once on failure
```

This example produces two execution stages:
- **Stage 1**: `fetch-issues` (no dependencies, runs first)
- **Stage 2**: `summarize` (depends on fetch-issues)
- **Stage 3**: `post-slack` (depends on summarize)

If two nodes have no edges between them, they run in the same stage concurrently.
