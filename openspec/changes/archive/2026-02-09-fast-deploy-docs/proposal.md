## Why

The Fast Deploy feature (Phase 1: base engine image, Phase 2+3: ConfigMap code delivery) fundamentally changes the build and deploy pipeline. The existing documentation (`docs/architecture.md` and `tentacular-skill/references/deployment-guide.md`) describes the old monolithic flow where workflow code is baked into the Docker image. Developers using these docs will be confused by the new behavior — build no longer includes workflow code, deploy now creates ConfigMaps, and `--cluster-registry` is replaced by `--image`. Documentation must be updated to reflect the new architecture.

## What Changes

### `docs/architecture.md`
- **Deployment Pipeline > Build Phase**: Update to reflect engine-only base image build. No workflow code in build context. Default tag `tentacular-engine:latest`. Image tag saved to `.tentacular/base-image.txt`.
- **Deployment Pipeline > Deploy Phase**: Update to show ConfigMap generation from workflow code, image resolution cascade (`--image` > file > default), rollout restart after apply. Replace `--cluster-registry` reference.
- **Generated K8s Resources table**: Add ConfigMap row (`{workflow-name}-code`). Update Deployment row to note code volume mount.
- **System Overview diagram**: Update to show ConfigMap in the K8s cluster alongside Deployment.
- **Dockerfile reference** (line 68): Update `GenerateDockerfile()` description to note engine-only output.

### `tentacular-skill/references/deployment-guide.md`
- **Build section**: Update "What It Does" steps, Generated Dockerfile example (engine-only, no workflow COPY, adds DENO_DIR), image tag format (`tentacular-engine:latest`).
- **Deploy section**: Update Generated Manifests to include ConfigMap and code volume mount in Deployment. Update flags table (replace `--cluster-registry` with `--image`). Add ConfigMap YAML example.
- **New "Fast Iteration" section**: Explain the edit code → `tntc deploy` → done workflow (no Docker rebuild needed).
- **Full Lifecycle example**: Update to show new flow (`build` once, then `deploy` for code changes).

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `docs-architecture`: Update Deployment Pipeline section (build phase, deploy phase, generated resources table, diagram) to reflect engine-only base image + ConfigMap code delivery.
- `docs-deployment-guide`: Update build section (engine-only), deploy section (ConfigMap, --image flag, volume mount), add Fast Iteration section, update full lifecycle example.

## Impact

- **`docs/architecture.md`**: Sections 1 (diagram), 6 (Deployment Pipeline), and the package table (line 68) need updates.
- **`tentacular-skill/references/deployment-guide.md`**: Build section, Deploy section, and Full Lifecycle example need updates. New Fast Iteration section added.
- No code changes — documentation only.
