# Deployment Guide

Build, deploy, and manage Pipedreamer workflows on Kubernetes.

## Build

```bash
pipedreamer build [dir]
```

Builds a container image for the workflow.

### What It Does

1. Parses and validates `workflow.yaml`.
2. Generates a `Dockerfile.pipedreamer` (temporary, deleted after build).
3. Copies the engine into the build context as `.engine/`.
4. Runs `docker build` to produce the image.

### Generated Dockerfile

The generated Dockerfile uses the distroless Deno base image:

```dockerfile
FROM denoland/deno:distroless

WORKDIR /app

# Copy engine
COPY .engine/ /app/engine/

# Copy workflow files
COPY workflow.yaml /app/
COPY nodes/ /app/nodes/

# Copy deno.json for import map resolution
COPY .engine/deno.json /app/deno.json

# Cache dependencies
RUN ["deno", "cache", "engine/main.ts"]

EXPOSE 8080

ENTRYPOINT ["deno", "run", "--allow-net", "--allow-read=/app", "--allow-write=/tmp", "--allow-env", "engine/main.ts", "--workflow", "/app/workflow.yaml", "--port", "8080"]
```

### Image Tag

Default tag format: `<workflow-name>:<version-with-dashes>` (e.g., `my-workflow:1-0`).

Override with `--tag`:

```bash
pipedreamer build --tag my-workflow:v2.1
pipedreamer build -r registry.example.com   # prepends registry
```

When `--registry` is set, the tag becomes `<registry>/<tag>`.

## Deploy

```bash
pipedreamer deploy [dir]
```

Generates and applies Kubernetes manifests. Runs preflight checks automatically before applying.

### Generated Manifests

Manifests always include a Deployment and Service. CronJob manifests are added for each `type: cron` trigger.

**Deployment** with gVisor RuntimeClass:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: <workflow-name>
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <workflow-name>
    app.kubernetes.io/managed-by: pipedreamer
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: <workflow-name>
  template:
    spec:
      runtimeClassName: gvisor
      containers:
        - name: engine
          image: <image-tag>
          ports:
            - containerPort: 8080
          volumeMounts:
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
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: <workflow-name>
  ports:
    - port: 8080
      targetPort: 8080
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--namespace` | `-n` | `default` | Kubernetes namespace for deployment |
| `--registry` | `-r` | (none) | Container registry prefix for image tag |

```bash
pipedreamer deploy -n production -r gcr.io/my-project
```

## Cluster Check

```bash
pipedreamer cluster check
```

Runs preflight validation to ensure the cluster is ready for deployment.

### Checks Performed

- Kubernetes API reachability
- Target namespace exists
- gVisor RuntimeClass is available
- Required Secrets exist (convention: `<workflow-name>-secrets`)
- RBAC permissions (including `batch/cronjobs` and `batch/jobs` for cron triggers)

Preflight checks run automatically during `pipedreamer deploy`. Failures abort the deploy with remediation instructions.

### Flags

| Flag | Description |
|------|-------------|
| `--fix` | Auto-create namespace and apply basic RBAC if missing |
| `-n` / `--namespace` | Target namespace to check (default: `default`) |
| `-o` / `--output` | Output format: `text` or `json` |

```bash
pipedreamer cluster check --fix -n production
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
pipedreamer list -n production
pipedreamer list -n production -o json
```

Shows all pipedreamer-managed deployments with status, replicas, and age.

### Check Status

```bash
pipedreamer status my-workflow -n production
pipedreamer status my-workflow -n production --detail
```

Basic status shows readiness and replica count. `--detail` adds image, runtime class, resource limits, service endpoint, pod statuses, and recent K8s events.

### Trigger a Workflow

```bash
pipedreamer run my-workflow -n production
pipedreamer run my-workflow -n production --timeout 60s
```

Creates a temporary curl pod that POSTs to the workflow's ClusterIP service. Status messages go to stderr; the JSON result goes to stdout (pipe-friendly).

### View Logs

```bash
pipedreamer logs my-workflow -n production
pipedreamer logs my-workflow -n production --tail 50
pipedreamer logs my-workflow -n production -f
```

Shows logs from the first Running pod. `--tail` controls how many recent lines (default 100). `-f` streams logs in real time until interrupted.

### Remove a Workflow

```bash
pipedreamer undeploy my-workflow -n production
pipedreamer undeploy my-workflow -n production --yes
```

Deletes the Service, Deployment, Secret (`<name>-secrets`), and all CronJobs matching the workflow's labels. Prompts for confirmation unless `--yes` is passed. Resources that don't exist are silently skipped.

### Full Lifecycle (No kubectl)

```bash
pipedreamer validate my-workflow
pipedreamer build my-workflow -r localhost:30500 --push
pipedreamer deploy my-workflow -n production --cluster-registry registry.registry.svc.cluster.local:5000
pipedreamer list -n production
pipedreamer status my-workflow -n production --detail
pipedreamer run my-workflow -n production
pipedreamer logs my-workflow -n production --tail 20
pipedreamer undeploy my-workflow -n production --yes
```

## Triggers

### Cron Triggers

Cron triggers generate K8s CronJob manifests automatically during `pipedreamer deploy`.

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
- Labels: `app.kubernetes.io/name`, `app.kubernetes.io/managed-by: pipedreamer`

#### Viewing CronJobs

```bash
kubectl get cronjobs -n <namespace> -l app.kubernetes.io/managed-by=pipedreamer
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

`pipedreamer undeploy` removes all resources for a workflow:

- Service
- Deployment
- Secret (`{name}-secrets`)
- **All CronJobs** matching labels `app.kubernetes.io/name={name},app.kubernetes.io/managed-by=pipedreamer`

CronJob cleanup uses label selectors, so it catches all CronJobs regardless of how many triggers existed.

## Security Model (Fortress)

Pipedreamer uses a three-layer security model:

### Layer 1: Deno Permission Flags

The engine runs with restricted Deno permissions:

| Flag | Scope | Purpose |
|------|-------|---------|
| `--allow-net` | All network | Nodes make HTTP requests via ctx.fetch, NATS connections |
| `--allow-read=/app` | `/app` only | Read workflow files, engine code, secrets |
| `--allow-write=/tmp` | `/tmp` only | Temporary file operations only |
| `--allow-env` | All env vars | Environment variable access for NATS and runtime config |

No file system access outside `/app` (read) and `/tmp` (write). No subprocess spawning, no FFI.

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

### Local Development

Create a `.secrets.yaml` file in the workflow directory:

```yaml
github:
  token: "ghp_abc123"
slack:
  api_key: "xoxb-..."
  webhook_url: "https://hooks.slack.com/services/..."
```

The engine loads this file at startup. It is used by `ctx.secrets` and for `ctx.fetch` auth injection.

Use `.secrets.yaml.example` (generated by `pipedreamer init`) as a template. Add `.secrets.yaml` to `.gitignore`.

### Production (Kubernetes)

Secrets are mounted from a Kubernetes Secret as a volume at `/app/secrets`:

```bash
# Create the K8s Secret
kubectl create secret generic my-workflow-secrets \
  -n production \
  --from-file=github=./github-secrets.json \
  --from-file=slack=./slack-secrets.json
```

Each file in the Secret volume becomes a key in `ctx.secrets`. Files are parsed as JSON if possible; otherwise stored as `{ value: "<content>" }`.

The Deployment manifest mounts the Secret volume as read-only at `/app/secrets` with `optional: true` (deployment succeeds even without secrets, but `ctx.secrets` will be empty).

Convention for Secret naming: `<workflow-name>-secrets` (e.g., `my-workflow-secrets`).
