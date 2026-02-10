# CLI Reference

The `pipedreamer` CLI manages the full workflow lifecycle — from scaffolding to deployment and operations.

## Workflow Lifecycle

| Command | Usage | Description |
|---------|-------|-------------|
| `init` | `pipedreamer init <name>` | Scaffold a new workflow directory |
| `validate` | `pipedreamer validate [dir]` | Validate workflow.yaml (DAG acyclicity, naming, edges) |
| `dev` | `pipedreamer dev [dir]` | Local dev server with hot-reload |
| `test` | `pipedreamer test [dir][/<node>]` | Run node or pipeline tests against fixtures |
| `build` | `pipedreamer build [dir]` | Build container image (distroless Deno base) |
| `deploy` | `pipedreamer deploy [dir]` | Generate K8s manifests and apply to cluster |
| `visualize` | `pipedreamer visualize [dir]` | Generate Mermaid diagram of the workflow DAG |

## Operations

| Command | Usage | Description |
|---------|-------|-------------|
| `status` | `pipedreamer status <name>` | Check deployment readiness; `--detail` for extended info |
| `run` | `pipedreamer run <name>` | Trigger a deployed workflow, return JSON result |
| `logs` | `pipedreamer logs <name>` | View pod logs; `-f` to stream in real time |
| `list` | `pipedreamer list` | List all deployed workflows in a namespace |
| `undeploy` | `pipedreamer undeploy <name>` | Remove deployed workflow (Deployment, Service, Secret) |
| `cluster check` | `pipedreamer cluster check` | Preflight cluster validation; `--fix` auto-remediates |

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | `default` | Kubernetes namespace |
| `--registry` | `-r` | (none) | Container registry URL |
| `--output` | `-o` | `text` | Output format: `text` or `json` |

## Key Command Flags

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
