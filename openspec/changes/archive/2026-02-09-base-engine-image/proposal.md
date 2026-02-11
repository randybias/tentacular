## Why

The current `tntc build` command bakes workflow code (workflow.yaml + nodes/*.ts) into the Docker image alongside the Deno engine. This means every code change — even a one-line fix to a node — requires a full Docker rebuild, push, and redeployment. By splitting the image into an engine-only base image (Phase 1 of Fast Deploy), we enable workflow code to be delivered separately via ConfigMap (Phase 2), eliminating Docker rebuilds for code-only changes.

## What Changes

- **BREAKING**: `GenerateDockerfile()` no longer copies workflow code into the image. The generated Dockerfile produces an engine-only image containing the Deno runtime, engine code, and pre-cached engine dependencies only. Engine deps are still cached at build time (`RUN deno cache engine/main.ts`); only node/workflow deps shift to runtime caching.
- The generated ENTRYPOINT **retains** `--workflow /app/workflow/workflow.yaml` as the default path. The ConfigMap (Phase 2) will mount workflow code at `/app/workflow/`, so this is the correct default. The Deployment `args:` field can override if needed. This ensures the image produces a clear "file not found" error at a known path if run without args, rather than a cryptic usage message.
- Existing permissions `--allow-env` and `--allow-read=/app,/var/run/secrets` are **preserved as-is** — these are already present in the current ENTRYPOINT (not new additions).
- The generated Dockerfile adds `ENV DENO_DIR=/tmp/deno-cache` for runtime caching of third-party dependencies imported by workflow nodes.
- **BREAKING**: `GenerateDockerfile(workflowDir string)` becomes `GenerateDockerfile()` with no params. The `workflowDir` param was used to scan `nodes/` for cache lines; since nodes are no longer copied, this param is unnecessary.
- `tntc build` no longer requires a workflow directory argument. After successful build+push, saves image tag to `.tentacular/base-image.txt` in the **project root** (not workflow dir, since we're decoupling from workflows). `build.go` creates `.tentacular/` directory if it doesn't exist.
- Default image tag changes from `<workflow-name>:<version>` to `tentacular-engine:latest`. If `--tag` is not specified, the default is `tentacular-engine:latest`. Workflow-specific versioning moves to the ConfigMap/deploy phase.
- Existing Dockerfile-related tests updated; new dedicated test file for Dockerfile generation. Specifically:
  - `TestDockerfileCopyInstructions`: Updated to assert engine-only COPY (no workflow.yaml, no nodes/).
  - `TestDockerfileCacheAndEntrypoint`: Updated to assert `--workflow /app/workflow/workflow.yaml` still present in ENTRYPOINT.
  - New `pkg/builder/dockerfile_test.go`: Dedicated assertions for no-workflow-copy, engine-only content, correct entrypoint with default --workflow, DENO_DIR env.

## Capabilities

### New Capabilities
- `base-engine-image`: Engine-only Dockerfile generation that produces a reusable base image without workflow code, with pre-cached engine dependencies, runtime DENO_DIR for node deps, and secure minimal permissions.

### Modified Capabilities
- `container-build`: Dockerfile generation no longer copies workflow.yaml or nodes/. `GenerateDockerfile()` signature drops `workflowDir` param. Build command no longer requires workflow directory. Default tag is `tentacular-engine:latest` (workflow-specific versioning moves to deploy phase). ENTRYPOINT retains `--workflow /app/workflow/workflow.yaml` as default (overridable via Deployment args in Phase 2).

## Impact

- **`pkg/builder/dockerfile.go`**: `GenerateDockerfile()` signature changes (removes `workflowDir` param). Output Dockerfile removes workflow/nodes COPY instructions. Adds `ENV DENO_DIR=/tmp/deno-cache`. ENTRYPOINT keeps `--workflow /app/workflow/workflow.yaml` as default. Engine deps still cached at build time.
- **`pkg/cli/build.go`**: Updated to call parameterless `GenerateDockerfile()`. Build context excludes workflow code. Saves image tag to `.tentacular/base-image.txt` in project root (creates dir if needed). Default tag is `tentacular-engine:latest`.
- **`pkg/builder/k8s_test.go`**: Existing TestDockerfile* tests updated for new signature/content — engine-only COPY assertions, default --workflow still present.
- **New `pkg/builder/dockerfile_test.go`**: Dedicated test file with engine-only Dockerfile assertions (no workflow copy, engine-only, entrypoint with default --workflow, DENO_DIR).
- **Downstream impact**: Phase 2 (ConfigMap) depends on this change being complete — Deployment manifests will mount workflow code via ConfigMap and override `--workflow` path via container `args:`.
