# Pipedreamer v2 Roadmap

## Webhook Triggers via NATS Bridge

A single gateway workflow subscribes to HTTP webhooks and publishes events to NATS subjects. Downstream workflows subscribe to those subjects via queue triggers. This avoids per-workflow ingress configuration and centralizes webhook handling.

```
GitHub Webhook → Gateway Workflow → NATS publish("events.github.push") → Queue Trigger Workflows
```

Benefits: no per-workflow Ingress resources, single TLS termination point, centralized webhook verification.

## ConfigMap-Mounted Runtime Config Overrides

Mount a K8s ConfigMap at `/app/config` to override workflow config values at runtime without rebuilding the container. The engine merges ConfigMap values on top of workflow.yaml config.

## NATS JetStream Durable Subscriptions

Upgrade from core NATS (at-most-once) to JetStream (at-least-once delivery) for queue triggers. This provides:

- **Durable subscriptions**: messages persist if the workflow engine is offline
- **Acknowledgment**: messages are redelivered if not acknowledged within a timeout
- **Replay**: ability to replay historical messages for debugging or reprocessing

## Message Payload Passthrough as Workflow Input

Currently queue trigger messages are parsed as JSON and passed to root nodes. Future enhancement: support binary payloads, content-type negotiation, and schema validation for incoming messages.

## Rate Limiting / Concurrency Control for Queue Triggers

Add configurable concurrency limits for NATS-triggered executions:

- **Max concurrent executions**: prevent resource exhaustion from message bursts
- **Rate limiting**: token bucket or sliding window rate limiting
- **Backpressure**: slow down NATS subscription when at capacity

## Dead Letter Queue for Failed Executions

Failed NATS-triggered executions publish the original message to a configurable dead letter subject (e.g., `{subject}.dlq`). This enables:

- Retry from DLQ after fixing issues
- Alerting on DLQ depth
- Forensic analysis of failed messages

## Multi-Cluster Deployment

Support deploying workflows across multiple K8s clusters with a single command. The CLI discovers available clusters from kubeconfig contexts and generates manifests for each.

## Workflow Versioning and Canary Deploys

Support running multiple versions of a workflow simultaneously with traffic splitting. CronJobs and NATS subscriptions route to the active version. Canary deploys send a percentage of traffic to the new version.

## Workflow Version Tracking in Deployment Metadata

**Status:** IDENTIFIED (Feb 2026)

The `version` field in workflow.yaml is validated but never used for tracking or display. When you deploy a workflow, there's no way to know which version is running.

**Proposal:**
1. Add `app.kubernetes.io/version` label to all generated K8s resources (Deployment, Service, ConfigMap, CronJobs)
2. `pipedreamer status <name>` should display the deployed version
3. `pipedreamer list` should show version column

**Benefits:**
- Visibility into what's deployed
- Enables kubectl queries like `kubectl get deploy -l app.kubernetes.io/version=1.0`
- Follows K8s recommended labels standard

## Immutable Versioned ConfigMaps

**Status:** IDENTIFIED (Feb 2026)

ConfigMap is always named `{name}-code`. Updates overwrite content, destroying the previous version. No rollback capability.

**Proposal:**
1. Name ConfigMaps as `{name}-code-{version}` (e.g., `uptime-prober-code-1-0`)
2. Set `immutable: true` on ConfigMaps (K8s 1.21+)
3. Deployment references the versioned ConfigMap name
4. Old ConfigMaps are retained for rollback

**Benefits:**
- Rollback support: change Deployment to reference old ConfigMap, restart pods
- Audit trail: `kubectl get configmap` shows all historical versions
- Follows K8s immutable config best practice
- Enables blue-green deploys (two Deployments, different ConfigMap versions)

**Trade-offs:**
- ConfigMaps accumulate over time (need cleanup policy or `--prune` flag)
- Version bumps in workflow.yaml are now meaningful (not just cosmetic)

## Workflow Version History Command

**Status:** IDENTIFIED (Feb 2026)

No way to see what versions have been deployed or what changed between them.

**Proposal:**
Add `pipedreamer versions <name>` command:
- Lists all ConfigMaps matching `{name}-code-*` pattern
- Shows version, creation timestamp, size
- Optional `--diff v1 v2` flag to show code differences

**Benefits:**
- Discoverability of deployed versions
- Debugging aid ("which version had the bug?")
- Complements rollback feature

## Workflow Rollback Command

**Status:** IDENTIFIED (Feb 2026)

No way to revert to a previous version after a bad deploy.

**Proposal:**
Add `pipedreamer rollback <name> --version <version>` command:
1. Finds ConfigMap `{name}-code-{version}`
2. Patches Deployment to reference it
3. Runs rollout restart

**Requirements:**
- Depends on immutable versioned ConfigMaps
- Should fail fast if target version ConfigMap doesn't exist

**Benefits:**
- Fast recovery from bad deploys
- Reduces risk of rapid iteration (easy to undo)

## Pre-Built Base Image with Dynamic Workflow Loading

**Status:** RESOLVED (Feb 2026 — base-engine-image + configmap-code-deploy features)

`pipedreamer build` now generates an engine-only Docker image (`pipedreamer-engine:latest`) with no workflow code baked in. `pipedreamer deploy` creates a ConfigMap with workflow.yaml and nodes/*.ts, mounts it at `/app/workflow/`, and triggers a rollout restart. Code changes deploy in ~5-10 seconds without Docker rebuilds.

## Preflight Secret Provisioning Ordering

**Status:** RESOLVED (Feb 2026)

The deploy command previously ran preflight checks that unconditionally required a `<name>-secrets` Secret to exist in the cluster, even for workflows with no secrets. This caused first-deploy failures and blocked secret-free workflows entirely.

**Fix applied** (`pkg/cli/deploy.go`): The secret preflight check now only runs when local secrets are detected (`.secrets/` directory or `.secrets.yaml` file). When no local secrets exist, the check is skipped and an informational message is logged. This also resolves the chicken-and-egg ordering issue — workflows with local secrets still get verified, but provisioning happens in the same deploy command after preflight passes.

## Nested Secrets YAML Support

`buildSecretFromYAML()` in `deploy.go` unmarshals `.secrets.yaml` into `map[string]string`, which fails for the common nested format (`slack: { webhook_url: "..." }`). Support nested YAML by JSON-serializing nested maps into the K8s Secret's stringData entries, matching what the engine's `loadSecretsFromDir` expects.

## ImagePullPolicy in Generated Deployments

The generated Deployment manifests don't set `imagePullPolicy`. When using a mutable tag (e.g., `uptime-prober:1-0`), Kubernetes caches the image on the node and won't pull updates on redeploy. Fix: set `imagePullPolicy: Always` in the generated Deployment spec, or use digest-based image references.

**Workaround for local kind clusters:** kind nodes cannot pull from the host Docker daemon. After building the engine image, load it into kind and patch the pull policy:

```bash
# Load the image into kind
kind load docker-image pipedreamer-engine:latest --name <cluster-name>

# Patch the deployment to never pull (image is already on the node)
kubectl patch deployment <name> -n <namespace> \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"engine","imagePullPolicy":"Never"}]}}}}'
```

---

# Issues Discovered During Workflow Development (Feb 2026)

The following items were discovered while building and deploying the `uptime-prober`, `cluster-health-collector`, and `cluster-health-reporter` workflows to a k0s cluster. They are organized by component.

## Engine Bugs (Fixed, Need Upstream Commit)

### K8s Secret Volume Symlink Bug

**File**: `engine/context/secrets.ts`, function `loadSecretsFromDir()`

**Problem**: Kubernetes mounts Secret volumes using a symlink structure. Each key in the Secret becomes a symlink under the mount path (e.g., `/app/secrets/slack` is a symlink to `..data/slack`, which itself is a symlink to `..2024_01_01.../slack`). The original code used `entry.isFile` to filter directory entries, but Deno's `readDir()` returns `isFile: false` for symlinks. This caused all secrets to be silently skipped when running on a cluster, even though they loaded fine during local development (where `.secrets/` contains real files).

**Symptom**: `ctx.secrets` was empty on the cluster. Nodes using secrets (e.g., Slack webhook delivery) silently failed with `delivered: false`. No error was logged because the secrets directory existed and was readable — it just appeared empty.

**Fix applied**:
```typescript
// Before (broken):
if (!entry.isFile) continue;

// After (fixed):
if (!entry.isFile && !entry.isSymlink) continue;
```

**Scope**: This affects every workflow that uses secrets on Kubernetes. It is a critical bug that should be committed upstream immediately.

### Read-Only Filesystem Lockfile Error

**File**: `pkg/builder/dockerfile.go` (Dockerfile generator)

**Problem**: The distroless Deno container image has a read-only filesystem. When `deno run` executes, it attempts to write or update a lockfile at `/app/engine/deno.lock`. Since `/app` is read-only (only `/tmp` is writable), this fails with: `Failed writing lockfile '/app/engine/deno.lock': Permission denied (os error 13): Read-only file system`.

**Symptom**: Pods crash immediately on startup with the lockfile error. The engine never reaches the "listening" state.

**Fix applied**: Added `--no-lock` flag to both `deno cache` (build time) and `deno run` (runtime) commands in the generated Dockerfile:
```dockerfile
RUN ["deno", "cache", "--no-lock", "nodes/fetch-cluster-state.ts"]
# ...
ENTRYPOINT ["deno", "run", "--no-lock", "--unstable-net", "--allow-net", ...]
```

**Alternative approaches**:
- Generate the lockfile at build time and make it read-only (would need `--lock=<path>` pointed at a writable location at runtime, or just accept `--no-lock`)
- Mount a writable volume at `/app/engine/` (undermines read-only security posture)
- Set `--lock-write` during `deno cache` at build time, then `--lock` (read-only) at runtime

**Recommendation**: `--no-lock` is the simplest and most correct fix for a production container. Dependencies are cached at build time; there is no value in lockfile management at runtime.

## CLI / Builder Issues

### Node Dependency Caching in Dockerfile

**File**: `pkg/builder/dockerfile.go`, function `GenerateDockerfile()`

**Problem**: The original generated Dockerfile only ran `deno cache engine/main.ts`, which caches the engine's own dependencies. It did not cache dependencies imported by individual workflow nodes. Third-party libraries used in nodes (e.g., `jsr:@db/postgres@0.19.5`) were not pre-cached, causing either slow cold starts (if network is available) or outright failures at runtime in the distroless container (no network access to download on first run in some environments).

**Fix applied**: `GenerateDockerfile()` now accepts a `workflowDir` parameter, scans the `nodes/` directory for `.ts` files, and generates a `deno cache --no-lock` command for each:
```go
func GenerateDockerfile(workflowDir string) string {
    entries, err := os.ReadDir(filepath.Join(workflowDir, "nodes"))
    // generates: RUN ["deno", "cache", "--no-lock", "nodes/<file>.ts"]
    // for each .ts file found
}
```

This ensures all third-party imports are resolved and cached in the container image at build time. The function signature change also required updating `pkg/cli/build.go` (passes `absDir`) and `pkg/builder/k8s_test.go` (passes `""`).

### `--allow-read` Path Too Narrow

**File**: `pkg/builder/dockerfile.go`

**Problem**: The original ENTRYPOINT used `--allow-read=/app`, which only permits reading files under `/app`. Workflows that access in-cluster Kubernetes resources need to read the service account token and CA certificate from `/var/run/secrets/kubernetes.io/serviceaccount/`. Without this, any `Deno.readTextFile()` call to that path fails with a permission error.

**Fix applied**: Widened to `--allow-read=/app,/var/run/secrets`.

**Consideration**: This is a reasonable default for any workflow running in a Kubernetes pod. The SA token path is standard and read-only. However, this could also be made configurable per workflow via a `permissions:` block in workflow.yaml for workflows that need more restrictive sandboxing.

### `--unstable-net` Required for Custom TLS

**File**: `pkg/builder/dockerfile.go`

**Problem**: `Deno.createHttpClient()` is an unstable API that requires the `--unstable-net` flag. Any workflow that needs to make HTTPS requests with a custom CA certificate (e.g., calling the in-cluster Kubernetes API with the cluster's CA from `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`) requires this flag.

**Fix applied**: Added `--unstable-net` to the ENTRYPOINT in the generated Dockerfile.

**Consideration**: This is currently applied globally to all workflows. A more targeted approach would be to make it opt-in via workflow.yaml (e.g., `permissions: { unstable_net: true }`) or to detect whether any node imports Deno unstable APIs. However, the flag has minimal security impact — it only enables additional Deno APIs, not additional system access — so applying it globally is pragmatic.

### JSR Import Map Resolution for Third-Party Libraries

**File**: `engine/deno.json`

**Problem**: Some third-party Deno libraries on `deno.land/x` internally use JSR-style bare specifiers (e.g., `import { bold } from "@std/fmt/colors"`). These work when Deno resolves them from the JSR registry, but fail in the pipedreamer build context because the engine's `deno.json` import map doesn't include mappings for `@std/*` packages.

Specifically, `deno.land/x/postgres@v0.19.5` failed because it imports `@std/fmt/colors`, `@std/io`, and `@std/bytes` — none of which were in the import map.

**Fix applied**: Added explicit mappings in `engine/deno.json`:
```json
{
  "@std/fmt/colors": "https://deno.land/std@0.224.0/fmt/colors.ts",
  "@std/io": "https://deno.land/std@0.224.0/io/mod.ts",
  "@std/bytes": "https://deno.land/std@0.224.0/bytes/mod.ts"
}
```

**Problem with this fix**: This is whack-a-mole. Every new third-party library that uses a `@std/*` bare specifier we haven't mapped will break. The root cause is that the engine's `deno.json` acts as the import map for the entire runtime, and JSR bare specifiers are not automatically resolved when using URL-based imports.

**Better solutions**:
1. **Switch engine to JSR imports entirely** — use `jsr:@std/yaml`, `jsr:@std/path`, etc. instead of `deno.land/std` URLs. This would make JSR resolution work naturally.
2. **Use a workspace or separate deno.json per workflow** — let each workflow bring its own import map that is merged with the engine's at build time.
3. **Pre-resolve all `@std/*` packages** — add comprehensive mappings for the full `@std` surface area in the engine's deno.json. Brittle but simple.

**Recommendation**: Option 1 (migrate engine to JSR imports) is the cleanest long-term fix.

## Deployment / RBAC Gaps

### No RBAC Scaffolding or Generation

**Problem**: Workflows that need access to the Kubernetes API (e.g., cluster-health-collector reading nodes and pods) require a ServiceAccount, ClusterRole, and ClusterRoleBinding. Currently there is no CLI support for generating these resources. We had to create them manually:

```yaml
# ServiceAccount in the workflow namespace
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cluster-health-collector
  namespace: pd-cluster-health

# ClusterRole with read-only access
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-health-reader
rules:
  - apiGroups: [""]
    resources: ["nodes", "pods", "namespaces"]
    verbs: ["get", "list"]

# Binding
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-health-collector-binding
subjects:
  - kind: ServiceAccount
    name: cluster-health-collector
    namespace: pd-cluster-health
roleRef:
  kind: ClusterRole
  name: cluster-health-reader
  apiGroup: rbac.authorization.k8s.io
```

**Proposal**: Add a `serviceAccount` field to workflow.yaml and optional RBAC generation:

```yaml
# workflow.yaml
deployment:
  serviceAccount: cluster-health-collector
  rbac:
    clusterRole:
      rules:
        - apiGroups: [""]
          resources: ["nodes", "pods", "namespaces"]
          verbs: ["get", "list"]
```

`pipedreamer deploy` would then:
1. Create the ServiceAccount if it doesn't exist
2. Create the ClusterRole/ClusterRoleBinding if specified
3. Set `spec.template.spec.serviceAccountName` in the Deployment

### ServiceAccount Not Wired to Deployment

**Problem**: Even if a ServiceAccount exists in the namespace, `pipedreamer deploy` generates the Deployment without `serviceAccountName`, so it uses the `default` SA which has no RBAC privileges. We had to manually patch the Deployment:

```bash
kubectl patch deployment cluster-health-collector -n pd-cluster-health \
  --type=json -p='[{"op":"add","path":"/spec/template/spec/serviceAccountName","value":"cluster-health-collector"}]'
```

**Fix**: The generated Deployment should include `serviceAccountName` when specified in workflow.yaml (see RBAC proposal above).

### No Multi-Workflow Namespace Coordination

**Problem**: The `cluster-health-collector` and `cluster-health-reporter` both deploy to `pd-cluster-health` and share Postgres secrets. But `pipedreamer deploy` treats each workflow independently — each call wants to create the namespace, each has its own secret (`<name>-secrets`). We had to manually create shared secrets and coordinate deployment order.

**Consideration**: This may be out of scope for the CLI, which is workflow-centric. But documenting the pattern of shared namespaces and secrets would help. Alternatively, a `pipedreamer deploy-group` command or a project-level manifest could coordinate multiple related workflows.

## Testing Gaps

### No Way to Mock In-Cluster APIs Locally

**Problem**: The `fetch-cluster-state.ts` node reads the service account token from `/var/run/secrets/kubernetes.io/serviceaccount/token` and calls the Kubernetes API. There is no way to run this locally without:
1. Having an actual kubeconfig or SA token at that path
2. Mocking the filesystem reads
3. Providing an environment variable override for the API server

The test framework's `createMockContext()` provides mock `fetch` but cannot mock `Deno.readTextFile()` calls that happen outside the context API.

**Proposals**:
1. **Environment variable override**: If `KUBERNETES_SERVICE_HOST` is set (as it is in-cluster), use the SA token. Otherwise, look for a kubeconfig-style config in `ctx.config` or `ctx.secrets`.
2. **Filesystem mock in test context**: Extend `createMockContext()` with a `_setFileContent(path, content)` method that intercepts `Deno.readTextFile()`.
3. **K8s helper in Context API**: Add `ctx.k8s.get(path)` that handles auth, TLS, and local/cluster detection automatically.

### Fixture Tests Cannot Test Config-Dependent Nodes

**Problem**: Nodes that read from `ctx.config` (e.g., `probe-endpoints.ts` reads `ctx.config.endpoints`) cannot be meaningfully tested with fixtures because `createMockContext()` provides empty `config: {}`. The fixture format has `input` and `expected` fields but no way to specify the config that should be injected.

**Fix**: Extend the fixture format to support `config` and `secrets` overrides:

```json
{
  "input": {},
  "config": {
    "endpoints": ["https://example.com"]
  },
  "secrets": {
    "slack": { "webhook_url": "https://hooks.slack.com/..." }
  },
  "expected": { "results": [{ "url": "https://example.com", "ok": true }] }
}
```

The test runner would pass `config` and `secrets` to `createMockContext()` when present in the fixture.

### Pipeline Tests Use Mock Fetch

**Problem**: `pipedreamer test --pipeline` runs the full DAG but still uses `createMockContext()`, which returns mock responses for all `ctx.fetch()` calls. This means pipeline tests never validate real HTTP behavior, Slack delivery, database writes, etc. For workflows that depend on external services (which is most of them), the pipeline test only validates that nodes can pass data through edges without errors — not that the workflow actually works.

**Consideration**: This is arguably by design (tests should be hermetic), but there should be an integration test mode that uses real services. A `--live` flag or separate `pipedreamer integration-test` command could run the full pipeline with real context.

## Documentation / Skill Gaps

### Deployment Guide Shows Stale Dockerfile

**File**: `pipedreamer-skill/references/deployment-guide.md`

The "Generated Dockerfile" section shows the original Dockerfile without:
- Per-node `deno cache` commands
- `--no-lock` flag
- `--unstable-net` flag
- `--allow-read` including `/var/run/secrets`

This needs to be updated to match the actual generated output.

### No Documentation on In-Cluster Patterns

There is no documentation or examples showing how to:
- Access the Kubernetes API from within a workflow (SA token, CA cert, `Deno.createHttpClient`)
- Use Postgres or other databases from workflow nodes (connection patterns, JSR imports)
- Set up RBAC for workflows that need cluster access
- Share resources between related workflows in the same namespace

These are common real-world patterns. The `cluster-health-collector` and `cluster-health-reporter` workflows serve as reference implementations, but the patterns should be documented in the skill references.

### Skill SKILL.md Missing In-Cluster and Database Patterns

**File**: `pipedreamer-skill/SKILL.md`

The skill document covers HTTP-based workflows well (ctx.fetch with GitHub, Slack, etc.) but does not cover:
- Workflows that call the Kubernetes API using in-cluster credentials
- Database access patterns (Postgres, etc.)
- Custom TLS handling (`Deno.createHttpClient` with custom CA certs)
- Workflows that need RBAC/ServiceAccount configuration
- Multi-workflow architectures (collector + reporter sharing state)

### AGENTS.md Missing Example Workflows Directory Convention

**Status**: RESOLVED. All workflows consolidated into `example-workflows/`. AGENTS.md, README.md, and architecture.md updated.

## Uptime-Prober v2 (Pending)

The uptime-prober v1 is deployed and working with hardcoded endpoint configuration in workflow.yaml. The v2 upgrade depends on the ConfigMap-mounted runtime config feature (being developed separately). Once that feature lands:

1. Create a ConfigMap with the endpoint list
2. Mount it at `/app/config/endpoints.yaml` (or similar)
3. Update `probe-endpoints.ts` to read from the mounted config, falling back to `ctx.config.endpoints`
4. Rebuild, deploy over v1, validate that endpoints can be changed without rebuilding the container
