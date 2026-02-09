## Context

Phase 1 (`base-engine-image`) produced an engine-only Docker image that contains the Deno runtime and engine code but no workflow code. The image's ENTRYPOINT defaults to `--workflow /app/workflow/workflow.yaml`, expecting workflow code to be mounted at `/app/workflow/`.

Current state:
- `GenerateK8sManifests()` in `pkg/builder/k8s.go` produces Deployment + Service (+ CronJobs). The Deployment has no code volume — it relied on workflow code baked into the Docker image.
- `deploy.go` derives the image tag from `wf.Name:wf.Version` or `--cluster-registry` prefix. It generates manifests, runs preflight checks, auto-provisions secrets, and applies via `client.Apply()`.
- `client.go` has `Apply()` (create-or-update semantics), `GetStatus()`, `DeleteResources()`, etc. The `findResource()` map already includes `"ConfigMap"`, so ConfigMap apply works out of the box.
- The Deployment template already mounts `/tmp` as `emptyDir` and `/app/secrets` as a K8s Secret volume.

## Goals / Non-Goals

**Goals:**
- Generate a K8s ConfigMap containing workflow.yaml + all nodes/*.ts files
- Mount the ConfigMap into the Deployment at `/app/workflow/`
- Rely on base image ENTRYPOINT defaults for `--workflow` and `--port` (no container `args` needed)
- Update deploy command to resolve base image via cascade (`--image` > file > default)
- Replace `--cluster-registry` flag with simpler `--image` flag
- Trigger rollout restart after apply to pick up ConfigMap changes
- Enforce 900KB size limit on ConfigMap data

**Non-Goals:**
- Binary file support in ConfigMap (only text files: .yaml, .ts, .json)
- ConfigMap immutability or versioning (future optimization)
- Init container for dependency pre-warming (future enhancement)
- Changes to the engine runtime or server.ts
- Watch/auto-deploy on file changes

## Decisions

### 1. ConfigMap key structure uses relative paths

**Decision:** ConfigMap data keys are relative paths from the workflow directory: `workflow.yaml`, `nodes/fetch.ts`, `nodes/summarize.ts`, etc. This mirrors the filesystem layout the engine expects.

**Rationale:** The engine's DAG compiler resolves node paths relative to the workflow.yaml location. By preserving the directory structure in ConfigMap keys and mounting at `/app/workflow/`, the engine sees the same layout as local development: `/app/workflow/workflow.yaml`, `/app/workflow/nodes/fetch.ts`, etc.

**Alternative considered:** Flat keys (e.g., `fetch.ts` instead of `nodes/fetch.ts`) — rejected because it breaks the engine's relative path resolution and would require engine changes.

**Note:** Kubernetes ConfigMaps support keys containing `/` slashes. When mounted as a volume, K8s automatically creates the necessary subdirectory structure (e.g., key `nodes/fetch.ts` creates `/app/workflow/nodes/fetch.ts`). This is confirmed K8s behavior — no `subPath` or `items` projection needed.

### 2. ConfigMap name is `{wf.Name}-code`

**Decision:** The ConfigMap is named `{workflow-name}-code`, consistent with the existing `{workflow-name}-secrets` naming pattern.

**Rationale:** Follows the established convention in the codebase. The `-code` suffix clearly distinguishes it from the `-secrets` Secret.

### 3. 900KB size limit with clear error

**Decision:** `GenerateCodeConfigMap()` returns an error if total data size exceeds 900KB.

**Rationale:** K8s ConfigMap hard limit is 1MB (etcd constraint). Using 900KB provides a ~100KB safety margin for metadata/encoding overhead. The function calculates total data size by summing all file contents and returns a descriptive error: "workflow code exceeds ConfigMap size limit (900KB): total size is X bytes".

**Alternative considered:** No validation (let K8s reject it) — rejected because the K8s API error is cryptic and doesn't explain the root cause. Also considered splitting into multiple ConfigMaps — rejected as over-engineering for the typical workflow size.

### 4. No container args — rely on ENTRYPOINT defaults

**Decision:** The Deployment does NOT include container `args`. The base image ENTRYPOINT already includes `--workflow /app/workflow/workflow.yaml --port 8080` as defaults, which match the ConfigMap mount path.

**Rationale:** Phase 1 sets the ENTRYPOINT to `["deno", "run", ..., "engine/main.ts", "--workflow", "/app/workflow/workflow.yaml", "--port", "8080"]`. Since `--workflow` and `--port` are baked into the ENTRYPOINT array, adding `args` in K8s would *append* additional flags (K8s `args` maps to Docker CMD, which appends to ENTRYPOINT). This would cause duplicate `--workflow` and `--port` flags. Instead, we rely on the ENTRYPOINT defaults which already point to the correct ConfigMap mount path.

**Alternative considered:** (a) Split ENTRYPOINT to end at `engine/main.ts` and pass `--workflow`/`--port` via Deployment `args` — rejected because it requires Phase 1 coordination, makes the standalone image unusable without args, and adds complexity. (b) The current approach is simpler: ENTRYPOINT has sensible defaults, Deployment needs no `args`.

### 5. Image resolution cascade

**Decision:** Deploy resolves the base image tag in this order:
1. `--image` flag (explicit override)
2. `.pipedreamer/base-image.txt` (written by `pipedreamer build`)
3. `pipedreamer-engine:latest` (hardcoded fallback)

**Rationale:** This provides flexibility while keeping the common case simple. After `pipedreamer build`, the tag is automatically available via the file. CI/CD pipelines can use `--image` for explicit control. The fallback ensures deploy works even without a prior build (e.g., using a pre-built image from a registry).

### 6. Replace `--cluster-registry` with `--image`

**Decision:** Remove the `--cluster-registry` flag and replace with `--image`. The old flag prepended a registry prefix to a workflow-derived tag. The new flag sets the full image reference directly.

**Rationale:** The old pattern (`--cluster-registry gcr.io/proj` → `gcr.io/proj/workflow:1-0`) doesn't work with engine-only images. `--image` is simpler and more explicit: `--image gcr.io/proj/pipedreamer-engine:latest`. The cascade (Decision 5) handles the common case where `--image` isn't needed.

### 7. Rollout restart via annotation patch

**Decision:** New `RolloutRestart(namespace, deploymentName string) error` method patches `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]` with the current timestamp.

**Rationale:** This is the same mechanism `kubectl rollout restart` uses. Patching an annotation triggers a rolling update because the pod template spec changed. Using `client-go` patch (strategic merge) is atomic and idempotent.

**Alternative considered:** Delete pods manually — rejected because it causes downtime and doesn't respect the deployment's rollout strategy.

### 8. ConfigMap applied before rollout restart

**Decision:** The deploy flow is: generate manifests (including ConfigMap) → apply all → rollout restart.

**Rationale:** The ConfigMap must exist and be updated before restarting pods, otherwise new pods mount the old ConfigMap data. The apply step uses create-or-update semantics, so re-deploying is idempotent.

## Risks / Trade-offs

**[Risk] ConfigMap mount propagation delay** — After ConfigMap update, kubelet takes up to 60s to propagate changes to mounted volumes (sync period).
→ **Mitigation:** Rollout restart creates new pods that mount the latest ConfigMap data immediately. Old pods are terminated after new ones are ready.

**[Risk] ConfigMap 1MB size limit** — Large workflows with many nodes or large node files may exceed the limit.
→ **Mitigation:** 900KB validation with clear error message. For very large workflows, a future enhancement could use a PVC or init container to fetch code.

**[Risk] ENTRYPOINT path must match ConfigMap mount** — The base image ENTRYPOINT hardcodes `--workflow /app/workflow/workflow.yaml`, which must match the ConfigMap mount path (`/app/workflow/`).
→ **Mitigation:** Resolved by design — Phase 1 ENTRYPOINT already uses `/app/workflow/workflow.yaml` and Phase 2 mounts the ConfigMap at `/app/workflow/`. No `args` override needed, eliminating the duplicate flags risk.

**[Risk] Breaking change: `--cluster-registry` removed** — Existing CI/CD scripts using `--cluster-registry` will break.
→ **Mitigation:** The flag was only recently introduced and not widely used. Print a clear deprecation error if `--cluster-registry` is used: "flag --cluster-registry is removed; use --image instead".

**[Risk] Rollout restart on every deploy** — Even if code hasn't changed, deploy always triggers a restart. If the Deployment spec also changed (e.g., image tag update), pods may restart twice.
→ **Mitigation:** Acceptable for now — restarts are rolling and zero-downtime. The extra restart is harmless. A future optimization could compare ConfigMap content hash and skip restart if unchanged, or detect whether the Deployment spec changed and skip the annotation patch.
