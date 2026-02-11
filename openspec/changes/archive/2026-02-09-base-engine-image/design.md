## Context

The current `tntc build` produces a monolithic Docker image containing both the Deno engine and workflow code (workflow.yaml + nodes/*.ts). Every code change requires a full Docker rebuild. This Phase 1 change decouples the engine image from workflow code, producing a reusable base image that Phase 2 will pair with a ConfigMap for workflow delivery.

Current state:
- `GenerateDockerfile(workflowDir string)` scans `nodes/` to generate per-node `deno cache` lines, then COPYs workflow.yaml, nodes/, and engine/ into the image
- `build.go` requires a workflow directory, parses `workflow.yaml` to derive the image tag from `wf.Name:wf.Version`
- ENTRYPOINT hardcodes `--workflow /app/workflow.yaml` with permissions `--allow-net`, `--allow-read=/app,/var/run/secrets`, `--allow-write=/tmp`, `--allow-env`
- K8s Deployment mounts `/tmp` as `emptyDir` and sets `readOnlyRootFilesystem: true`

## Goals / Non-Goals

**Goals:**
- Produce an engine-only Docker image with no workflow code baked in
- Remove the `workflowDir` parameter from `GenerateDockerfile()`
- Cache engine dependencies at build time; defer node dependency caching to runtime via `DENO_DIR`
- Set default ENTRYPOINT `--workflow /app/workflow/workflow.yaml` (ConfigMap mount path for Phase 2)
- Default image tag `tentacular-engine:latest` when `--tag` not specified
- Persist the built image tag to `.tentacular/base-image.txt` in project root for Phase 2 consumption
- Update 5 existing Dockerfile tests in `k8s_test.go` and add new dedicated `dockerfile_test.go`

**Non-Goals:**
- ConfigMap generation or mounting (Phase 2)
- Deployment manifest changes for volume mounts or args overrides (Phase 2)
- Changes to `deploy.go` or K8s manifest generation
- Changes to the engine runtime (`main.ts`, `server.ts`)
- Multi-arch build support changes
- Workflow-specific image tagging (moves to deploy phase)

## Decisions

### 1. `GenerateDockerfile()` becomes parameterless

**Decision:** Remove the `workflowDir` parameter entirely.

**Rationale:** The parameter existed solely to scan `nodes/` for per-node `deno cache` lines. Since nodes are no longer copied into the image, there's nothing to scan. The function becomes a pure template with no filesystem interaction.

**Alternative considered:** Keep the param but ignore it — rejected as misleading API surface.

### 2. Engine deps cached at build time, node deps at runtime via DENO_DIR

**Decision:** Keep `RUN ["deno", "cache", "engine/main.ts"]` in the Dockerfile. Add `ENV DENO_DIR=/tmp/deno-cache` so that when nodes import third-party deps at runtime, Deno caches them in the writable `/tmp` volume.

**Rationale:** Engine deps are static and known at build time — caching them avoids cold-start latency. Node deps vary per workflow and aren't available at build time. `/tmp` is already mounted as `emptyDir` in the K8s manifest and permitted by `readOnlyRootFilesystem`.

**Alternative considered:** Cache nothing at build time (pure runtime caching) — rejected because engine deps are ~50% of cold-start time and are always the same.

### 3. ENTRYPOINT retains `--workflow /app/workflow/workflow.yaml` as default

**Decision:** Keep the `--workflow` flag in ENTRYPOINT pointing to `/app/workflow/workflow.yaml`.

**Rationale:** The ConfigMap (Phase 2) will mount workflow code at `/app/workflow/`, so this is the correct default path. Having a sensible default means: (a) the image produces a clear "file not found at /app/workflow/workflow.yaml" error if run standalone, rather than a cryptic usage message; (b) the K8s Deployment only needs to override `args:` if using a non-standard mount path.

**Alternative considered:** Drop `--workflow` from ENTRYPOINT — rejected per architect review because it makes the image unusable without explicit args. Also considered keeping `/app/workflow.yaml` (old path) — rejected because `/app/workflow/` is the correct ConfigMap mount point.

### 4. Default image tag is `tentacular-engine:latest`

**Decision:** When `--tag` is not specified, default to `tentacular-engine:latest`.

**Rationale:** The current code derives the tag from `wf.Name + ":" + wf.Version`, but since the base image is decoupled from any specific workflow, a workflow-derived tag doesn't make sense. Workflow-specific versioning moves to the ConfigMap/deploy phase.

**Alternatives considered:** (a) Use CLI version constant (`tentacular-engine:<cli-version>`) — adds coupling between image tag and CLI versioning. (b) Require `--tag` explicitly — too much friction for the common case. Team-lead confirmed `tentacular-engine:latest` as the right default.

### 5. Tag persisted to `.tentacular/base-image.txt` in project root

**Decision:** After successful build (and optional push), write the full image tag to `.tentacular/base-image.txt` relative to the current working directory (project root). Create `.tentacular/` if it doesn't exist.

**Rationale:** Phase 2's `deploy` command needs to know which base image to reference in the Deployment manifest. Storing in project root (not a workflow dir) reflects the decoupling — one engine image serves multiple workflows.

**Alternative considered:** Store in a config file (JSON/YAML) — rejected as over-engineering for a single value. A plain text file is simpler and trivially readable by shell scripts and CI.

### 6. Build still validates workflow.yaml but no longer uses it for tagging

**Decision:** The build command still accepts an optional `[dir]` argument and validates workflow.yaml exists, but no longer uses `wf.Name`/`wf.Version` for the default tag.

**Rationale:** Validation ensures the user is building in a valid project context. The workflow spec parsing can be relaxed in a future change if we want a truly workflow-independent build. For Phase 1, we keep the existing CLI contract.

### 7. Existing permissions preserved, not added

**Decision:** `--allow-env` and `--allow-read=/app,/var/run/secrets` remain as-is in the ENTRYPOINT.

**Rationale:** Both are already present in the current Dockerfile (line 45 of `dockerfile.go`). The architect confirmed these are not new additions. Documented as "preserved" to avoid confusion.

## Risks / Trade-offs

**[Risk] Node cold-start latency for third-party imports** — First request after deployment may be slow while Deno fetches and caches node dependencies.
→ **Mitigation:** DENO_DIR persists in the emptyDir volume across requests within a pod's lifetime. Only the very first invocation pays the cost. A future enhancement could pre-warm the cache via an init container.

**[Risk] DENO_DIR in `/tmp` lost on pod restart** — If a pod restarts, cached node deps are lost and re-fetched.
→ **Mitigation:** Acceptable for now. Node deps are typically small. A persistent volume could be added later if needed, but emptyDir keeps the security posture simple.

**[Risk] Existing tests break** — Five tests in `k8s_test.go` assert current Dockerfile content (workflow COPY, nodes COPY, `--workflow /app/workflow.yaml` in entrypoint).
→ **Mitigation:** Explicitly update these tests as part of implementation. `TestDockerfileCopyInstructions` assertions change from checking presence of workflow.yaml/nodes/ to asserting their absence. `TestDockerfileCacheAndEntrypoint` updated to check for `--workflow /app/workflow/workflow.yaml`. New `dockerfile_test.go` provides dedicated coverage.

**[Risk] `.tentacular/` directory may not be in `.gitignore`** — Contains local build state that shouldn't be committed.
→ **Mitigation:** Verify during implementation; add to `.gitignore` if not present.

**[Risk] Base image alone can't run a workflow** — Expected and by design. The image prints a clear error when run without ConfigMap.
→ **Mitigation:** Phase 2 delivers the ConfigMap. Documentation should clarify the two-step flow.
