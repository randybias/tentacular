# Deployment Guide

Build, deploy, and manage Tentacular workflows on Kubernetes.

## Build

```bash
tntc build [dir]
```

Builds a container image for the workflow.

### What It Does

1. Parses and validates `workflow.yaml` (for project context validation).
2. Generates an engine-only `Dockerfile.tentacular` (temporary, deleted after build).
3. Copies the engine into the build context as `.engine/`.
4. Runs `docker build` to produce the base engine image.
5. Saves the image tag to `.tentacular/base-image.txt` for deploy to use.

### Generated Dockerfile

The generated Dockerfile produces an engine-only image (workflow code delivered separately via ConfigMap):

```dockerfile
FROM denoland/deno:distroless

WORKDIR /app

# Copy engine
COPY .engine/ /app/engine/

# Copy deno.json for import map resolution
COPY .engine/deno.json /app/deno.json

# Cache engine dependencies
RUN ["deno", "cache", "--no-lock", "engine/main.ts"]

# Set DENO_DIR for runtime caching of node dependencies
ENV DENO_DIR=/tmp/deno-cache

EXPOSE 8080

ENTRYPOINT ["deno", "run", "--no-lock", "--unstable-net", "--allow-net", "--allow-read=/app,/var/run/secrets", "--allow-write=/tmp", "--allow-env", "engine/main.ts", "--workflow", "/app/workflow/workflow.yaml", "--port", "8080"]
```

The engine-only image contains no workflow code. Workflow code (workflow.yaml + nodes/*.ts) is mounted at `/app/workflow` via ConfigMap during deployment.

### Image Tag

Default tag: `tentacular-engine:latest` (engine-only, no workflow-specific versioning).

Override with `--tag`:

```bash
tntc build --tag my-engine:v2.1
tntc build -r registry.example.com --tag my-engine:v2.1
```

When `--registry` is set (via `-r` flag or config file), the tag becomes `<registry>/<tag>`. If `-r` is not explicitly passed, the build command falls back to the `registry` value from the config file (`~/.tentacular/config.yaml` or `.tentacular/config.yaml`).

The tag is saved to `.tentacular/base-image.txt` for `tntc deploy` to use. Workflow-specific versioning is handled separately at deploy time via ConfigMap updates.

## Deploy

```bash
tntc deploy [dir]
```

Generates and applies Kubernetes manifests. Runs preflight checks automatically before applying.

### Generated Manifests

Manifests always include a ConfigMap, Deployment, and Service. CronJob manifests are added for each `type: cron` trigger.

**ConfigMap** with workflow code:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <workflow-name>-code
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <workflow-name>
    app.kubernetes.io/version: "<version>"
    app.kubernetes.io/managed-by: tentacular
data:
  workflow.yaml: |
    name: my-workflow
    version: "1.0"
    ...
  nodes__fetch.ts: |
    export default async function run(ctx, input) {
      ...
    }
```

**Note on ConfigMap key naming:** Kubernetes ConfigMap data keys cannot contain forward slashes (validation regex: `[-._a-zA-Z0-9]+`). Node files use `__` as a directory separator (e.g., `nodes__fetch.ts`). The Deployment's ConfigMap volume uses the `items` field to map these flattened keys back to proper paths when mounted (e.g., `nodes__fetch.ts` → `nodes/fetch.ts`).

**Deployment** with gVisor RuntimeClass, security hardening, and code volume:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: <workflow-name>
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <workflow-name>
    app.kubernetes.io/version: "<version>"
    app.kubernetes.io/managed-by: tentacular
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: <workflow-name>
  template:
    metadata:
      labels:
        app.kubernetes.io/name: <workflow-name>
        app.kubernetes.io/version: "<version>"
        app.kubernetes.io/managed-by: tentacular
    spec:
      runtimeClassName: gvisor
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: engine
          image: <image-tag>
          imagePullPolicy: Always
          ports:
            - containerPort: 8080
              protocol: TCP
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 3
            periodSeconds: 5
          volumeMounts:
            - name: code
              mountPath: /app/workflow
              readOnly: true
            - name: secrets
              mountPath: /app/secrets
              readOnly: true
            - name: tmp
              mountPath: /tmp
          resources:
            requests:
              memory: "64Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "500m"
      volumes:
        - name: code
          configMap:
            name: <workflow-name>-code
            items:
              - key: workflow.yaml
                path: workflow.yaml
              - key: nodes__fetch.ts
                path: nodes/fetch.ts
              - key: nodes__summarize.ts
                path: nodes/summarize.ts
        - name: secrets
          secret:
            secretName: <workflow-name>-secrets
            optional: true
        - name: tmp
          emptyDir: {}
```

**Service** (ClusterIP on port 8080):

```yaml
apiVersion: v1
kind: Service
metadata:
  name: <workflow-name>
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <workflow-name>
    app.kubernetes.io/version: "<version>"
    app.kubernetes.io/managed-by: tentacular
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: <workflow-name>
  ports:
    - port: 8080
      targetPort: 8080
      protocol: TCP
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | (cascade) | Kubernetes namespace. Resolves via: `-n` flag > `workflow.yaml deployment.namespace` > config file > `default` |
| `--image` | | (cascade) | Base engine image tag. Resolves via: `--image` flag > `.tentacular/base-image.txt` > `tentacular-engine:latest` |
| `--runtime-class` | | `gvisor` | RuntimeClass name for pod sandboxing (empty to disable) |
| `--force` | | false | Skip the live test gate (alias: `--skip-live-test`) |
| `--verify` | | false | Run post-deploy verification (trigger workflow once after deploy) |

```bash
tntc deploy                       # namespace from workflow.yaml or config
tntc deploy -n production         # explicit namespace override
tntc deploy --image reg.io/engine:v2
tntc deploy --runtime-class ""    # disable gVisor
tntc deploy --force               # skip live test gate
tntc deploy -o json               # structured output with all phases
```

The `--cluster-registry` flag has been deprecated. Use `--image` to specify the full image reference.

### Namespace Resolution

Namespace is resolved in this order (first non-empty wins):

1. CLI `-n` / `--namespace` flag
2. `deployment.namespace` in workflow.yaml
3. `namespace` in config file (`~/.tentacular/config.yaml` or `.tentacular/config.yaml`)
4. `default`

Set a per-workflow namespace in workflow.yaml:

```yaml
deployment:
  namespace: pd-my-workflow
```

### Version Tracking

All generated K8s resources include an `app.kubernetes.io/version` label from the workflow.yaml `version` field. The version value is always YAML-quoted (e.g., `"1.0"`) to prevent YAML parsers from interpreting it as a float. `tntc list` displays a VERSION column alongside NAME, NAMESPACE, STATUS, REPLICAS, and AGE.

## Environment Configuration

Named environments extend the config cascade with cluster-specific settings for dev, staging, production, etc. Environments are defined in `~/.tentacular/config.yaml` (user-level) or `.tentacular/config.yaml` (project-level).

### Config File Format

```yaml
# Base config (applies to all environments)
registry: reg.io
namespace: default

# Named environments
environments:
  dev:
    context: kind-dev                # kubeconfig context name
    namespace: dev-workflows         # target namespace
    image: tentacular-engine:latest  # engine image
    runtime_class: ""                # no gVisor (kind cluster)
    config_overrides:                # merged into workflow config
      timeout: 60s
      debug: true
    secrets_source: .secrets.yaml    # secrets file path
  staging:
    context: staging-cluster
    namespace: staging
    image: reg.io/tentacular-engine:v2.1
    runtime_class: gvisor
  production:
    context: prod-cluster
    namespace: production
    image: reg.io/tentacular-engine:v2.1
    runtime_class: gvisor
```

### Environment Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `context` | string | No | Kubeconfig context name. If set, the CLI switches to this context before operating. |
| `namespace` | string | No | Target K8s namespace for this environment |
| `image` | string | No | Engine image tag to use |
| `runtime_class` | string | No | RuntimeClass name. Empty string disables gVisor. |
| `config_overrides` | map | No | Key-value pairs merged into workflow config for this environment |
| `secrets_source` | string | No | Path to the secrets file for this environment |

### Resolution Order

Environment values fit into the existing config cascade. The full resolution order is:

1. CLI flags (`-n`, `--image`, `--runtime-class`)
2. Environment config (from `--env <name>`)
3. `workflow.yaml` settings (e.g., `deployment.namespace`)
4. Project config (`.tentacular/config.yaml`)
5. User config (`~/.tentacular/config.yaml`)
6. Defaults

### Config Overrides

The `config_overrides` map is merged into the workflow's `config` section at deploy time. This enables environment-specific values without modifying workflow.yaml.

For example, a workflow with `config.timeout: 30s` deployed with the `dev` environment above gets `config.timeout: 60s` and `config.debug: true` in its ConfigMap.

### Usage

Environments are used by `tntc test --live` and by `tntc deploy` (for the live test gate):

```bash
tntc test --live --env dev         # live test against dev environment
tntc test --live --env staging     # live test against staging
tntc deploy                        # auto-runs live test against dev (if configured)
```

## Kind Cluster Detection

When deploying to a [kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker) cluster, the CLI auto-detects it and adjusts deployment parameters. Detection is fully automatic -- no manual configuration required.

### Detection Method

A cluster is identified as kind when the kubeconfig context has:
- A name prefixed with `kind-` (e.g., `kind-dev`, `kind-tentacular`)
- A server address pointing to localhost

### Adjustments

When a kind cluster is detected, the CLI makes these changes:

| Parameter | Normal | Kind |
|-----------|--------|------|
| RuntimeClass | `gvisor` | (empty -- kind does not support gVisor) |
| ImagePullPolicy | `Always` | `IfNotPresent` (images loaded locally) |

A diagnostic message is printed:

```
Detected kind cluster 'dev', adjusted: no gVisor, imagePullPolicy=IfNotPresent
```

### Image Loading

After `tntc build`, the CLI detects kind and loads the image directly into the kind cluster using `kind load docker-image`. This avoids the need for a container registry during local development.

```bash
tntc build                         # builds image + loads into kind automatically
```

### Kind with Environments

When using named environments, set `runtime_class: ""` for kind-based environments:

```yaml
environments:
  dev:
    context: kind-dev
    namespace: dev-workflows
    runtime_class: ""              # explicit: kind has no gVisor
```

The auto-detection still runs and overrides RuntimeClass and ImagePullPolicy regardless of config, but setting `runtime_class: ""` in config makes the intent explicit.

## Deploy Gate

When a dev environment is configured, `tntc deploy` automatically runs a live test before applying manifests. This prevents deploying broken workflows to production.

### How It Works

1. Deploy checks if a `dev` environment exists in the config cascade.
2. If found (and `--force` is not set): runs `tntc test --live --env dev` internally.
3. If the live test passes: proceeds with the normal deploy flow.
4. If the live test fails: aborts with a structured error and does not apply manifests.

### Skipping the Gate

Use `--force` (alias `--skip-live-test`) to deploy without running the live test:

```bash
tntc deploy --force                # skip live test, deploy directly
tntc deploy --skip-live-test       # same thing
```

This is useful for:
- Hotfixes that need immediate deployment
- Workflows where the dev environment is unavailable
- Cases where the live test is known to fail for environment-specific reasons

### Post-Deploy Verification

When `--verify` is passed, deploy runs post-deploy verification after applying manifests. It waits for the rollout to complete, triggers the deployed workflow once, and validates the result. This flag is opt-in (default: false).

### JSON Output

With `-o json`, deploy emits a structured result using the standard command envelope:

```json
{
  "version": "1",
  "command": "deploy",
  "status": "pass",
  "summary": "deployed my-workflow to production",
  "hints": [],
  "timing": {
    "startedAt": "2026-02-16T09:00:00Z",
    "durationMs": 17000
  }
}
```

When `--verify` is used and verification fails, the `execution` field contains the workflow result:

```json
{
  "version": "1",
  "command": "deploy",
  "status": "fail",
  "summary": "verification: workflow returned success=false",
  "execution": {
    "success": false,
    "errors": ["node fetch-data threw: GitHub API returned 401"]
  },
  "hints": [
    "use --force to skip pre-deploy live test",
    "check deployment logs with: tntc logs <workflow-name>"
  ],
  "timing": {
    "startedAt": "2026-02-16T09:00:00Z",
    "durationMs": 17000
  }
}
```

### Failure Output

When the live test gate or deploy fails, the CLI returns a non-zero exit code with a structured error. The `hints` array provides actionable remediation steps:

```json
{
  "version": "1",
  "command": "deploy",
  "status": "fail",
  "summary": "deploy failed: pre-deploy live test failed (use --force to skip): workflow returned success=false",
  "hints": [
    "use --force to skip pre-deploy live test",
    "check deployment logs with: tntc logs <workflow-name>"
  ],
  "timing": {
    "startedAt": "2026-02-16T09:00:00Z",
    "durationMs": 12200
  }
}
```

## Fast Iteration

The engine-only image architecture enables rapid code iteration without Docker rebuilds.

### Workflow

1. **Build the engine image once:**
   ```bash
   tntc build --push -r my-registry.com
   ```
   This produces `my-registry.com/tentacular-engine:latest` and saves the tag to `.tentacular/base-image.txt`.

2. **Edit workflow code** (workflow.yaml or nodes/*.ts files)

3. **Deploy the changes:**
   ```bash
   tntc deploy
   ```
   This updates the ConfigMap with new code and triggers a rollout restart. No Docker build needed!

### What Happens on Deploy

- `GenerateCodeConfigMap()` reads current workflow.yaml and nodes/*.ts
- ConfigMap is created/updated via K8s API
- `RolloutRestart()` patches the Deployment to trigger a pod restart
- New pods mount the updated ConfigMap at `/app/workflow`
- Engine loads the new workflow code at startup

### Time Comparison

**Old flow (monolithic image):**
```
Edit code → docker build (30-60s) → docker push (10-30s) → kubectl apply → rollout (15s) = ~1-2 min
```

**New flow (ConfigMap):**
```
Edit code → tntc deploy (ConfigMap update + rollout) = ~5-10s
```

The ConfigMap update is instant (YAML over HTTP), and the rollout restart triggers a pod restart without re-pulling the image.

## Cluster Check

```bash
tntc cluster check
```

Runs preflight validation to ensure the cluster is ready for deployment.

### Checks Performed

- Kubernetes API reachability
- Target namespace exists
- gVisor RuntimeClass is available
- RBAC permissions (including `batch/cronjobs` and `batch/jobs` for cron triggers)

When run standalone (`tntc cluster check`), it also verifies the `<workflow-name>-secrets` Secret exists. During `tntc deploy`, the secret existence check is skipped when local secrets (`.secrets/` or `.secrets.yaml`) are present, since they will be auto-provisioned in the same deploy. This prevents first-deploy failures where the secret doesn't exist yet.

Preflight checks run automatically during `tntc deploy`. Failures abort the deploy with remediation instructions.

### Flags

| Flag | Description |
|------|-------------|
| `--fix` | Auto-create namespace and apply basic RBAC if missing |
| `-n` / `--namespace` | Target namespace to check (default: `default`) |
| `-o` / `--output` | Output format: `text` or `json` |

```bash
tntc cluster check --fix -n production
```

Output format:

```
  ✓ Kubernetes API reachable
  ✓ Namespace "production" exists
  ✓ gVisor RuntimeClass available
  ✗ Secret "my-workflow-secrets" not found
    -> Create secret: kubectl create secret generic my-workflow-secrets -n production

✓ Cluster is ready for deployment
```

## Operations

Post-deploy commands for managing workflows without kubectl.

### List Deployed Workflows

```bash
tntc list -n production
tntc list -n production -o json
```

Shows all tentacular-managed deployments with version, status, replicas, and age:

```
NAME                     VERSION  NAMESPACE        STATUS     REPLICAS   AGE
uptime-prober            1.0      pd-uptime-prober ready      1/1        2d
cluster-health-collector 1.0      pd-cluster-health ready     1/1        5h
```

### Check Status

```bash
tntc status my-workflow -n production
tntc status my-workflow -n production --detail
```

Basic status shows readiness and replica count. `--detail` adds image, runtime class, resource limits, service endpoint, pod statuses, and recent K8s events.

### Trigger a Workflow

```bash
tntc run my-workflow -n production
tntc run my-workflow -n production --timeout 60s
```

Creates a temporary curl pod that POSTs to the workflow's ClusterIP service. Status messages go to stderr; the JSON result goes to stdout (pipe-friendly).

### View Logs

```bash
tntc logs my-workflow -n production
tntc logs my-workflow -n production --tail 50
tntc logs my-workflow -n production -f
```

Shows logs from the first Running pod. `--tail` controls how many recent lines (default 100). `-f` streams logs in real time until interrupted.

### Remove a Workflow

```bash
tntc undeploy my-workflow -n production
tntc undeploy my-workflow -n production --yes
```

Deletes the Service, Deployment, Secret (`<name>-secrets`), ConfigMap (`<name>-code`), and all CronJobs matching the workflow's labels. Prompts for confirmation unless `--yes` is passed. Resources that don't exist are silently skipped.

### Full Lifecycle (No kubectl)

```bash
# Initial setup: validate and build engine image (one time)
tntc validate my-workflow
tntc build my-workflow -r my-registry.com --push

# Deploy workflow code (repeatable, fast)
tntc deploy my-workflow -n production --image my-registry.com/tentacular-engine:latest

# Operations
tntc list -n production
tntc status my-workflow -n production --detail
tntc run my-workflow -n production
tntc logs my-workflow -n production --tail 20

# Edit workflow code, then redeploy (no build needed!)
# ... edit nodes/fetch.ts ...
tntc deploy my-workflow -n production

# Cleanup
tntc undeploy my-workflow -n production --yes
```

After the initial `build`, subsequent code changes only require `deploy` (no Docker build/push).

## Audit Deployed Resources

```bash
tntc audit <workflow-dir> -n <namespace>
```

Compares deployed K8s resources against contract-derived expectations.

### What It Checks

| Resource | Comparison |
|----------|-----------|
| NetworkPolicy | Deployed vs derived egress/ingress rules |
| Secrets | Deployed secret keys vs derived secret inventory (at service-name level) |
| CronJobs | Count of deployed CronJobs vs cron triggers in workflow |

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | (workflow) | Kubernetes namespace |
| `--output` | `-o` | `text` | Output format: `text` or `json` |

## Triggers

### Cron Triggers

Cron triggers generate K8s CronJob manifests automatically during `tntc deploy`.

#### Setup

```yaml
# workflow.yaml
triggers:
  - type: cron
    name: daily-digest
    schedule: "0 9 * * *"
  - type: cron
    name: hourly-check
    schedule: "0 * * * *"
```

#### What Gets Generated

Each cron trigger produces a CronJob that curls the workflow's ClusterIP service:

- **Single cron**: CronJob named `{wf}-cron`
- **Multiple crons**: CronJobs named `{wf}-cron-0`, `{wf}-cron-1`, etc.
- **Named trigger**: POSTs `{"trigger": "<name>"}` to `/run`
- **Unnamed trigger**: POSTs `{}` to `/run`

CronJob properties:
- Image: `curlimages/curl:latest`
- Target: `http://{wf}.{ns}.svc.cluster.local:8080/run`
- `concurrencyPolicy: Forbid` (no overlapping runs)
- `successfulJobsHistoryLimit: 3`, `failedJobsHistoryLimit: 3`
- Labels: `app.kubernetes.io/name`, `app.kubernetes.io/managed-by: tentacular`

#### Viewing CronJobs

```bash
kubectl get cronjobs -n <namespace> -l app.kubernetes.io/managed-by=tentacular
```

#### Parameterized Execution

With named triggers, the first node receives `{"trigger": "daily-digest"}` as input. Use this to branch behavior:

```typescript
export default async function run(ctx: Context, input: { trigger?: string }) {
  if (input.trigger === "daily-digest") {
    // Full digest logic
  } else if (input.trigger === "hourly-check") {
    // Quick health check
  }
}
```

### Queue Triggers (NATS)

Queue triggers subscribe to NATS subjects. Messages trigger workflow execution.

#### Setup

```yaml
# workflow.yaml
triggers:
  - type: queue
    subject: events.github.push

config:
  nats_url: "nats.ospo-dev.miralabs.dev:18453"
```

Secrets (`.secrets.yaml`):
```yaml
nats:
  token: "your-nats-token"
```

#### NATS Connection

- **Server**: Specified in `config.nats_url`
- **Authentication**: Token from `secrets.nats.token`
- **TLS**: Uses system CA trust store. Let's Encrypt certificates work automatically — no special TLS configuration needed.
- **Graceful degradation**: If `nats_url` or `nats.token` is missing, the engine warns and skips NATS setup (HTTP triggers still work).

#### Message Flow

1. Message published to NATS subject (e.g., `events.github.push`)
2. Engine receives message, parses payload as JSON
3. Payload passed as input to root nodes
4. If message has a reply subject, execution result is sent back (request-reply)

#### Graceful Shutdown

On SIGTERM/SIGINT, the engine:
1. Drains NATS subscriptions (in-flight messages complete)
2. Shuts down the HTTP server
3. Exits cleanly

### Undeploy Cleanup

`tntc undeploy` removes the following resources for a workflow:

- Service
- Deployment
- Secret (`{name}-secrets`)
- **All CronJobs** matching labels `app.kubernetes.io/name={name},app.kubernetes.io/managed-by=tentacular`

CronJob cleanup uses label selectors, so it catches all CronJobs regardless of how many triggers existed.

**Note:** All workflow resources including the ConfigMap (`{name}-code`) are deleted by `tntc undeploy`.

## Security Model (Fortress)

Tentacular uses a three-layer security model:

### Layer 1: Deno Permission Flags

The engine runs with restricted Deno permissions:

| Flag | Scope | Purpose |
|------|-------|---------|
| `--allow-net` | All network | Nodes make HTTP requests via ctx.fetch, NATS connections |
| `--allow-read=/app,/var/run/secrets` | `/app` and `/var/run/secrets` | Read workflow files, engine code, secrets, K8s service account tokens |
| `--allow-write=/tmp` | `/tmp` only | Temporary file operations only |
| `--allow-env` | All env vars | Environment variable access for NATS and runtime config |
| `--unstable-net` | Network | Enables `Deno.createHttpClient()` for custom TLS (e.g., in-cluster K8s API calls) |

No file system access outside `/app` and `/var/run/secrets` (read) and `/tmp` (write). No subprocess spawning, no FFI.

### Layer 2: Distroless Container

The container image is based on `denoland/deno:distroless`:

- No shell, no package manager, no system utilities.
- Minimal attack surface -- only the Deno runtime binary.
- No way to install additional software at runtime.

### Layer 3: gVisor Sandbox

Kubernetes Deployment uses `runtimeClassName: gvisor`:

- gVisor intercepts all system calls from the container.
- Provides an additional kernel-level isolation boundary.
- Prevents container escapes even if Deno or the workflow code is compromised.

## Secrets Management

Required secrets are derived from `contract.dependencies` -- each dependency's `auth.secret` ref (e.g., `github.token`) tells the platform what secret keys are needed. There is no separate secrets declaration to author.

### Quick Setup

```bash
tntc secrets check my-workflow    # see what's required vs. provisioned
tntc secrets init my-workflow     # create .secrets.yaml from template
```

### Local Development

Create a `.secrets.yaml` file in the workflow directory. Keys must match the `auth.secret` refs in the contract:

```yaml
github:
  token: "ghp_abc123"
slack:
  api_key: "xoxb-..."
  webhook_url: "https://hooks.slack.com/services/..."
```

The engine loads this file at startup. Secret values are resolved by `ctx.dependency("name").secret` at call time.

Use `.secrets.yaml.example` (generated by `tntc init`) as a template. Add `.secrets.yaml` to `.gitignore`.

### Shared Secrets Pool

Place shared secrets at the repo root in `.secrets/`:

```
.secrets/
  slack      # contains: {"webhook_url": "https://hooks.slack.com/..."}
```

Reference them in workflow `.secrets.yaml` with `$shared.<name>`:

```yaml
slack: $shared.slack
```

During `tntc deploy`, `$shared.` references are resolved by reading the corresponding file from `<repo-root>/.secrets/`.

### Production (Kubernetes)

`tntc deploy` automatically provisions secrets to Kubernetes. Nested YAML maps in `.secrets.yaml` are JSON-serialized into K8s Secret `stringData` entries, matching the engine's `loadSecretsFromDir()` JSON parsing.

Secrets are mounted from a Kubernetes Secret as a volume at `/app/secrets`:

```bash
# Manual creation (alternative to tntc deploy auto-provisioning)
kubectl create secret generic my-workflow-secrets \
  -n production \
  --from-file=github=./github-secrets.json \
  --from-file=slack=./slack-secrets.json
```

Each file in the Secret volume becomes a key in `ctx.secrets`. Files are parsed as JSON if possible; otherwise stored as `{ value: "<content>" }`.

The Deployment manifest mounts the Secret volume as read-only at `/app/secrets` with `optional: true` (deployment succeeds even without secrets, but `ctx.secrets` will be empty).

Convention for Secret naming: `<workflow-name>-secrets` (e.g., `my-workflow-secrets`).

## Deployment Step Details

The recommended agentic deployment flow validates
workflows through six steps. Each step produces
structured JSON output (with `-o json`) for automation.

### 1. Validate

Parses workflow.yaml, checks name/version format,
trigger definitions, node paths, edge references, DAG
acyclicity, and contract structure. Also displays
derived artifacts: secret inventory and network policy
summary. Catches spec and contract errors before any
execution.

### 2. Review Contract Artifacts

Generates rich visualization showing DAG topology,
dependency graph, derived secrets, and network intent.
Agent and user review these artifacts before proceeding
to test or build. See [Pre-Build Review Gate](#pre-build-review-gate).

### 3. Mock Test

Runs node-level tests from fixtures using the mock
context. No cluster or credentials required. Validates
node logic, data flow, and contract drift (undeclared
deps, dead deps, API bypass). Fails in strict mode on
any drift.

### 4. Live Test

Deploys the workflow to a configured dev environment,
triggers it, validates the result, and cleans up.
Requires a `dev` environment in config (see
[Environment Configuration](#environment-configuration)).
Use `--env <name>` to target a different environment.
Add `--keep` to skip cleanup for debugging. Default
timeout is 120 seconds (`--timeout` to override).

### 5. Deploy

Generates K8s manifests including auto-derived
NetworkPolicy from contract dependencies and trigger
types. When a dev environment is configured, deploy
automatically runs a live test first and aborts if it
fails. Use `--force` (alias `--skip-live-test`) to
skip the live test gate. Add `--verify` for post-deploy
verification. Deploy aborts before manifest apply if
contract validation fails in strict mode.

### 6. Post-Deploy Verification

Triggers the deployed workflow once and validates the
result. Confirms the workflow runs successfully in its
target environment.

## Pre-Build Review Gate

Before any `build`, `test --live`, or `deploy`, the
agent MUST run a contract review loop:

1. `tntc validate` -- confirm contract parses cleanly
   and derived artifacts are correct.
2. `tntc visualize --rich --write` -- generate and
   persist rich diagram and contract summary to the
   workflow directory.
3. Review with user -- present the diagram and derived
   artifacts. Confirm:
   - Dependency targets (hosts, ports, protocols)
   - Secret key references match provisioned secrets
   - Derived network policy matches expected access
4. `tntc test` -- run mock tests with drift detection.
   Resolve any contract violations before proceeding.

**Required checklist before build/deploy:**

- [ ] Contract section present in workflow.yaml
- [ ] `tntc validate` passes (contract + spec)
- [ ] `tntc visualize --rich --write` reviewed with user
- [ ] Dependency hosts and ports confirmed
- [ ] Secret refs match `.secrets.yaml` keys
- [ ] Derived NetworkPolicy matches expected access
- [ ] `tntc test` passes with zero drift
- [ ] `workflow-diagram.md` and `contract-summary.md` committed to repo

Agents MUST fail closed when contract or diagram
artifacts are missing or stale. Do not proceed to
build or deploy without completing this gate.

## Structured Output

All commands support `-o json` for agent-consumable output. Every JSON response uses a common envelope:

```json
{
  "version": "1",
  "command": "validate|test|deploy|run",
  "status": "pass|fail",
  "summary": "human-readable one-liner",
  "hints": ["actionable suggestion if failed"],
  "timing": {
    "startedAt": "2026-02-16T09:00:00Z",
    "durationMs": 1234
  }
}
```

Commands add their own fields to the envelope:

- **validate**: validation errors array
- **test**: per-node test results with pass/fail, expected vs. actual, execution time
- **test --live**: `execution` field with the workflow run result (success, outputs, errors, timing)
- **deploy**: `execution` field on verification failure; `hints` with remediation steps on failure
- **run**: execution result with outputs, errors, and workflow timing
