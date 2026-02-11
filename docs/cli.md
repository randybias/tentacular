# CLI Reference

The `tntc` CLI manages the full workflow lifecycle — from scaffolding to deployment and operations.

## Workflow Lifecycle

| Command | Usage | Description |
|---------|-------|-------------|
| `init` | `tntc init <name>` | Scaffold a new workflow directory |
| `validate` | `tntc validate [dir]` | Validate workflow.yaml (DAG acyclicity, naming, edges) |
| `dev` | `tntc dev [dir]` | Local dev server with hot-reload |
| `test` | `tntc test [dir][/<node>]` | Run node or pipeline tests against fixtures |
| `build` | `tntc build [dir]` | Build container image (distroless Deno base) |
| `deploy` | `tntc deploy [dir]` | Generate K8s manifests and apply to cluster |
| `visualize` | `tntc visualize [dir]` | Generate Mermaid diagram of the workflow DAG |

## Operations

| Command | Usage | Description |
|---------|-------|-------------|
| `status` | `tntc status <name>` | Check deployment readiness; `--detail` for extended info |
| `run` | `tntc run <name>` | Trigger a deployed workflow, return JSON result |
| `logs` | `tntc logs <name>` | View pod logs; `-f` to stream in real time |
| `list` | `tntc list` | List all deployed workflows in a namespace |
| `undeploy` | `tntc undeploy <name>` | Remove deployed workflow (Deployment, Service, Secret) |
| `cluster check` | `tntc cluster check` | Preflight cluster validation; `--fix` auto-remediates |

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | `default` | Kubernetes namespace |
| `--registry` | `-r` | (none) | Container registry URL |
| `--output` | `-o` | `text` | Output format: `text` or `json` |

## Key Command Flags

```bash
# Build
tntc build -t custom:tag           # custom image tag
tntc build -r reg.io --push        # build and push
tntc build --platform linux/arm64  # cross-platform build

# Deploy
tntc deploy --cluster-registry registry.svc.cluster.local:5000
tntc deploy --runtime-class gvisor  # default; use "" to disable

# Test
tntc test my-workflow/fetch-data   # test single node
tntc test --pipeline               # full end-to-end pipeline test

# Logs
tntc logs my-workflow --tail 50    # last 50 lines
tntc logs my-workflow -f           # stream live

# Run
tntc run my-workflow --timeout 60s

# Undeploy
tntc undeploy my-workflow --yes    # skip confirmation
```
