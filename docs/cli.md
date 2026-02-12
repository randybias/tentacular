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

## Configuration

| Command | Usage | Description |
|---------|-------|-------------|
| `configure` | `tntc configure` | Set default configuration (registry, namespace, runtime class) |
| `secrets check` | `tntc secrets check [dir]` | Check secrets provisioning against node requirements |
| `secrets init` | `tntc secrets init [dir]` | Initialize `.secrets.yaml` from `.secrets.yaml.example` |

## Operations

| Command | Usage | Description |
|---------|-------|-------------|
| `status` | `tntc status <name>` | Check deployment readiness; `--detail` for extended info |
| `run` | `tntc run <name>` | Trigger a deployed workflow, return JSON result |
| `logs` | `tntc logs <name>` | View pod logs; `-f` to stream in real time |
| `list` | `tntc list` | List all deployed workflows with version, status, and age |
| `undeploy` | `tntc undeploy <name>` | Remove deployed workflow (Deployment, Service, Secret, CronJobs) |
| `cluster check` | `tntc cluster check` | Preflight cluster validation; `--fix` auto-remediates |

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | `default` | Kubernetes namespace |
| `--registry` | `-r` | (none) | Container registry URL |
| `--output` | `-o` | `text` | Output format: `text` or `json` |

Namespace resolution order: CLI `-n` flag > `workflow.yaml deployment.namespace` > config file default > `default`.

## Key Command Flags

```bash
# Configure
tntc configure --registry reg.io --namespace prod  # user-level defaults
tntc configure --registry reg.io --project         # project-level defaults

# Secrets
tntc secrets check my-workflow      # verify all secrets provisioned
tntc secrets init my-workflow       # create .secrets.yaml from template
tntc secrets init my-workflow --force  # overwrite existing .secrets.yaml

# Build
tntc build -t custom:tag           # custom image tag
tntc build -r reg.io --push        # build and push
tntc build --platform linux/arm64  # cross-platform build

# Deploy
tntc deploy --runtime-class gvisor  # default; use "" to disable
tntc deploy --image reg.io/engine:v2  # explicit image

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

## Configure Command

`tntc configure` sets default values for registry, namespace, and runtime class. Values are saved to YAML config files and used as defaults by other commands.

| Flag | Description |
|------|-------------|
| `--registry` | Default container registry URL |
| `--namespace` | Default Kubernetes namespace |
| `--runtime-class` | Default RuntimeClass name |
| `--project` | Write to project config (`.tentacular/config.yaml`) instead of user config |

Config resolution order: CLI flags > project config (`.tentacular/config.yaml`) > user config (`~/.tentacular/config.yaml`).

```yaml
# ~/.tentacular/config.yaml (or .tentacular/config.yaml)
registry: nats.ospo-dev.miralabs.dev:30500
namespace: default
runtime_class: gvisor
```

## Secrets Commands

### `tntc secrets check [dir]`

Scans `nodes/*.ts` for `ctx.secrets` references and compares against locally provisioned secrets (`.secrets.yaml` or `.secrets/` directory).

```
$ tntc secrets check example-workflows/uptime-prober
Secrets check for uptime-prober:
  slack  provisioned (.secrets.yaml)
  All 1 required secret(s) provisioned.
```

### `tntc secrets init [dir]`

Copies `.secrets.yaml.example` to `.secrets.yaml`, uncommenting example values.

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing `.secrets.yaml` |
