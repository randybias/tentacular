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
| `undeploy` | `tntc undeploy <name>` | Remove deployed workflow (Deployment, Service, Secret, ConfigMap, NetworkPolicy) |
| `cluster check` | `tntc cluster check` | Preflight cluster validation; `--fix` auto-remediates |
| `cluster profile` | `tntc cluster profile` | Capability snapshot for agent workflow design; auto-runs on `configure` |

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

## Cluster Profile Command

`tntc cluster profile` generates a capability snapshot of a target environment, giving
AI agents the context they need before designing tentacles. Unlike `cluster check` (which
validates readiness), `cluster profile` answers: **what can I build here?**

Profiles are generated automatically when running `tntc configure` and stored at
`.tentacular/envprofiles/<env>.md` and `.tentacular/envprofiles/<env>.json`.

### What Is Profiled

| Category | Details |
|----------|---------|
| Identity | K8s version, distribution (EKS/GKE/AKS/kind/vanilla) |
| Nodes | Count, architecture, labels, taints |
| Runtime | Available RuntimeClasses; gVisor detected |
| CNI | Plugin name, NetworkPolicy support, egress support |
| Storage | StorageClasses, CSI drivers, RWX capability |
| Extensions | Istio, cert-manager, Prometheus Operator, External Secrets, ArgoCD, Gateway API, Metrics Server |
| Namespace | Resource quotas, LimitRanges, Pod Security Admission level |
| Guidance | Derived agent-readable instructions for tentacle design |

### Flags

| Flag | Description |
|------|-------------|
| `--env <name>` | Environment from `.tentacular/config.yaml` (default: current context) |
| `--all` | Profile every configured environment |
| `--output markdown\|json` | Output format (default: markdown) |
| `--save` | Write to `~/.tentacular/envprofiles/` (user config) or `.tentacular/envprofiles/` (project config) |
| `--force` | Bypass the 1h re-run guard (prevents hammering the cluster on repeated `--save` calls) |

**Freshness thresholds — two distinct concepts:**
- **1-hour CLI guard**: `--save` skips re-profiling if a profile was written less than 1 hour ago. Use `--force` to override.
- **7-day agent staleness**: Agents should treat any profile older than 7 days as potentially stale and trigger a re-profile. This is a heuristic — re-profile sooner after any cluster change.

### Examples

```bash
# Profile the current context (stdout)
tntc cluster profile

# Profile a named environment and save
tntc cluster profile --env prod --save

# Re-profile all environments after a cluster upgrade
tntc cluster profile --all --save --force

# Machine-readable JSON for scripting
tntc cluster profile --env staging --output json

# Auto-triggered on first configure
tntc configure --project  # → writes config, then profiles all environments
```

### Profile Storage

```
.tentacular/
  config.yaml
  envprofiles/
    dev.md        ← agent loads this before building tentacles for dev
    dev.json      ← JSON sidecar for programmatic use
    staging.md
    prod.md
```

Profiles contain no secrets, but **node labels are included verbatim**. On managed cloud
clusters (EKS, GKE, AKS), labels routinely include account IDs, region identifiers, and
internal topology metadata. Review the profile before committing it to a shared repository.
Teams with strict infosec postures should add `envprofiles/` to `.gitignore` and treat
profiles as generated artifacts, not source files.

### Drift Detection

Re-run `tntc cluster profile --env <name> --save --force` when the agent encounters any of:

- `unknown RuntimeClass` error during deploy
- NetworkPolicy blocking traffic the profile says is allowed
- PVC binding failures where the profile shows the provisioner as available
- `cluster check` passes but deploy produces unexpected resource errors
- Profile `generatedAt` is older than 7 days
- K8s version in profile does not match `kubectl version`
- New CRD-based features needed that aren't reflected in the profile
