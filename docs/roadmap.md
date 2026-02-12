# Tentacular Roadmap

Prioritized around the new-developer critical path: **how fast can someone go from `git clone` to a running deployed workflow?**

---

## Tier 2: First Hour (Secrets and Testing)

### Secrets Rotation and Sync (Phase C)

Phases A and B are resolved. Remaining:
- `tntc secrets sync <workflow>` -- pushes local secrets to K8s without full redeploy
- `tntc secrets diff <workflow>` -- shows what's local vs. what's in-cluster

---

## Tier 3: First Week (Production Confidence)

### 9. Immutable Versioned ConfigMaps

ConfigMap is always named `{name}-code`. Updates overwrite content, destroying the previous version. No rollback capability.

**Fix:** Name ConfigMaps as `{name}-code-{version}`, set `immutable: true` (K8s 1.21+), Deployment references the versioned name. Old ConfigMaps retained for rollback.

Trade-off: ConfigMaps accumulate (need cleanup policy or `--prune` flag). Version bumps in workflow.yaml become meaningful.

### 10. Rollback Command

`tntc rollback <name> --version <version>` — finds ConfigMap `{name}-code-{version}`, patches Deployment to reference it, runs rollout restart. Depends on immutable versioned ConfigMaps (#9).

### 11. Version History Command

`tntc versions <name>` — lists all ConfigMaps matching `{name}-code-*`, shows version, creation timestamp, size. Optional `--diff v1 v2` to show code changes between versions.

### 12. RBAC Scaffolding

Workflows needing K8s API access (e.g., cluster-health-collector) require manually creating ServiceAccount, ClusterRole, and ClusterRoleBinding. The generated Deployment doesn't set `serviceAccountName` even when the SA exists.

**Fix:** Add `deployment.serviceAccount` and optional `deployment.rbac` to workflow.yaml:

```yaml
deployment:
  serviceAccount: cluster-health-collector
  rbac:
    clusterRole:
      rules:
        - apiGroups: [""]
          resources: ["nodes", "pods", "namespaces"]
          verbs: ["get", "list"]
```

`tntc deploy` creates the SA/ClusterRole/Binding if specified and wires `serviceAccountName` into the Deployment.

### 13. Deployment Metadata Enrichment

**Replaces the proposed local deployment registry.** The Kubernetes cluster IS the deployment registry. A local state store (SQLite/JSON) creates a second source of truth that drifts from the cluster. If you have kubectl access, you have the state. If you don't, a local file wouldn't help because you can't deploy either.

**Approach:**
- Add annotations to all generated K8s resources: `tentacular.io/deployed-by`, `tentacular.io/git-sha`, `tentacular.io/deployed-at`, `tentacular.io/source-path`
- `tntc list --all-namespaces` queries the cluster directly for all tentacular-managed resources
- Local `.tentacular/last-deploy.json` (git-ignored) caches the last deploy per-workflow for convenience (timestamp, namespace, cluster, image tag) — not a registry, just a "what did I last do from this machine" cache

---

## Tier 4: First Month (Scaling)

Items listed in approximate priority order within this tier.

### 14. ConfigMap-Mounted Runtime Config Overrides

Mount a K8s ConfigMap at `/app/config` to override workflow config values at runtime without rebuilding the container. The engine merges ConfigMap values on top of workflow.yaml config.

### 15. NATS JetStream Durable Subscriptions

Upgrade from core NATS (at-most-once) to JetStream (at-least-once delivery) for queue triggers: durable subscriptions, acknowledgment with timeout-based redelivery, and replay for debugging or reprocessing.

### 16. Rate Limiting / Concurrency Control for Queue Triggers

Max concurrent executions, token bucket / sliding window rate limiting, and backpressure to slow down NATS subscription when at capacity.

### 17. Dead Letter Queue for Failed Executions

Failed NATS-triggered executions publish the original message to `{subject}.dlq`. Enables retry from DLQ, alerting on DLQ depth, and forensic analysis.

### 18. Multi-Cluster Deployment

Deploy workflows across multiple K8s clusters with a single command. CLI discovers available clusters from kubeconfig contexts and generates manifests for each.

### 19. Canary Deploys / Traffic Splitting

Run multiple versions of a workflow simultaneously. CronJobs and NATS subscriptions route to the active version. Canary sends a percentage of traffic to the new version.

### 20. Webhook Triggers via NATS Bridge

A single gateway workflow subscribes to HTTP webhooks and publishes events to NATS subjects. Downstream workflows subscribe via queue triggers. Avoids per-workflow Ingress, centralizes webhook handling and TLS termination.

### 21. Message Payload Passthrough

Support binary payloads, content-type negotiation, and schema validation for incoming NATS trigger messages (currently JSON-only).

### 22. Multi-Workflow Namespace Coordination

`tntc deploy` treats each workflow independently, but related workflows (e.g., collector + reporter) often share a namespace and secrets. A `tntc deploy-group` command or project-level manifest could coordinate multiple related workflows. The per-workflow namespace feature (#5) solves the single-workflow case; this addresses the multi-workflow orchestration case.

### 23. JSR Import Migration

The engine uses `deno.land/std` URL-based imports, but third-party JSR libraries import `@std/*` bare specifiers that the engine's import map doesn't cover. Current fix is whack-a-mole (adding mappings as failures surface). Long-term fix: migrate engine to JSR imports (`jsr:@std/yaml`, `jsr:@std/path`, etc.) so JSR resolution works naturally.

---

## Archive (Resolved)

Items verified as fixed in the current codebase. Kept for historical context.

### Nested Secrets YAML Support

**Resolved Feb 2026.** `buildSecretFromYAML()` now unmarshals into `map[string]interface{}` and JSON-serializes nested maps into K8s Secret `stringData` entries. Matches the engine's `loadSecretsFromDir` JSON parsing.

### ImagePullPolicy in Generated Deployments

**Resolved Feb 2026.** Generated Deployment manifests now include `imagePullPolicy: Always`, ensuring Kubernetes always pulls the latest image on redeploy.

### Dockerfile `--no-lock` on `deno cache`

**Resolved Feb 2026.** The `deno cache` command in the generated Dockerfile now includes `--no-lock`, matching the `deno run` ENTRYPOINT. Node dependency caching via per-node `deno cache` was intentionally not implemented -- the engine image remains generic and code-free, with node dependencies downloaded at runtime into the writable `DENO_DIR=/tmp/deno-cache`.

### Initial Configuration (`tntc configure`)

**Resolved Feb 2026.** `tntc configure` sets default registry, namespace, and runtime class. Supports user-level (`~/.tentacular/config.yaml`) and project-level (`.tentacular/config.yaml`) config files. All commands read these defaults; CLI flags override. Resolution: CLI flag > project config > user config.

### Per-Workflow Namespace in workflow.yaml

**Resolved Feb 2026.** Workflows can declare `deployment.namespace` in workflow.yaml. Namespace resolution: CLI `-n` flag > `workflow.yaml deployment.namespace` > config file default > `default`.

### Version Tracking in K8s Metadata

**Resolved Feb 2026.** All generated K8s resources (ConfigMap, Deployment, Service, CronJob) include `app.kubernetes.io/version` label from workflow.yaml `version` field. `tntc list` displays a VERSION column.

### Secrets Management Overhaul (Phase A + B)

**Resolved Feb 2026.** Phase A: `tntc secrets check` scans node source for `ctx.secrets` references and reports gaps against provisioned secrets. `tntc secrets init` creates `.secrets.yaml` from `.secrets.yaml.example`. Phase B: Shared secrets pool at repo root (`.secrets/`) with `$shared.<name>` references in workflow `.secrets.yaml`, resolved during `tntc deploy`. Phase C (sync/diff) remains on the roadmap.

### Fixture Config/Secrets Support

**Resolved Feb 2026.** Test fixture format extended with optional `config` and `secrets` fields. The test runner passes these to `createMockContext()`, enabling meaningful testing of nodes that read `ctx.config` or `ctx.secrets`.

### Pre-Built Base Image with Dynamic Workflow Loading

**Resolved Feb 2026.** `tntc build` generates an engine-only Docker image with no workflow code baked in. `tntc deploy` creates a ConfigMap with `workflow.yaml` and `nodes/*.ts`, mounts at `/app/workflow/`, and triggers a rollout restart. Code changes deploy in ~5-10 seconds without Docker rebuilds.

### Preflight Secret Provisioning Ordering

**Resolved Feb 2026.** Deploy command preflight checks no longer check for K8s secret existence when local secrets (`.secrets/` or `.secrets.yaml`) will be auto-provisioned during the same deploy. When no local secrets exist, the check is also skipped. Fixes first-deploy failures where the secret didn't exist yet.

### K8s Secret Volume Symlink Bug

**Resolved Feb 2026.** `loadSecretsFromDir()` now checks `!entry.isFile && !entry.isSymlink` instead of `!entry.isFile`, correctly handling Kubernetes symlink-based Secret volume mounts.

### Read-Only Filesystem Lockfile Error (Runtime)

**Resolved Feb 2026.** `deno run` in the generated Dockerfile ENTRYPOINT includes `--no-lock`, preventing lockfile write failures on the read-only distroless filesystem. (Note: `deno cache` at build time still needs `--no-lock` — tracked in Tier 0 item #3.)

### `--allow-read` Path Widened

**Resolved Feb 2026.** ENTRYPOINT includes `--allow-read=/app,/var/run/secrets`, enabling workflows to read K8s service account tokens and CA certificates.

### `--unstable-net` Flag Added

**Resolved Feb 2026.** ENTRYPOINT includes `--unstable-net`, enabling `Deno.createHttpClient()` for custom TLS (e.g., in-cluster K8s API calls with cluster CA).

### Version Label YAML Quoting

**Resolved Feb 2026.** The `app.kubernetes.io/version` label value is now quoted in generated YAML (e.g., `"1.0"`) to prevent YAML parsers from interpreting semver strings like `1.0` as floating-point numbers.

### AGENTS.md Example Workflows Convention

**Resolved Feb 2026.** All workflows consolidated into `example-workflows/`. AGENTS.md, README.md, and architecture.md updated. Multi-arch build convention (`docker buildx`) added.
