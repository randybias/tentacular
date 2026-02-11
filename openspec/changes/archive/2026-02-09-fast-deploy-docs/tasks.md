## 1. Update docs/architecture.md

- [x] 1.1 Update "Build Phase" section (line 275): replace monolithic build steps with engine-only flow — generate engine-only Dockerfile, copy engine, docker build, default tag `tentacular-engine:latest`, save to `.tentacular/base-image.txt`
- [x] 1.2 Update "Deploy Phase" section (line 288): replace old flow with ConfigMap flow — parse workflow.yaml, resolve image via cascade (`--image` > file > default), generate ConfigMap from code, generate K8s manifests, apply all, rollout restart
- [x] 1.3 Update "Generated K8s Resources" table (line 316): add ConfigMap row (`{workflow-name}-code`, workflow.yaml + nodes/*.ts). Update Deployment row to mention code volume mount at `/app/workflow`
- [x] 1.4 Update package table entry for `dockerfile.go` (line 68): change description from monolithic to engine-only Dockerfile generation
- [x] 1.5 Update system overview diagram (line 7): add ConfigMap to K8s cluster side, show code delivery path separate from image push

## 2. Update deployment-guide.md

- [x] 2.1 Update Build > "What It Does" (line 13): replace steps with engine-only build flow
- [x] 2.2 Update Build > "Generated Dockerfile" (line 20): replace Dockerfile example with engine-only version (no COPY workflow.yaml, no COPY nodes/, add ENV DENO_DIR=/tmp/deno-cache, ENTRYPOINT with --workflow /app/workflow/workflow.yaml)
- [x] 2.3 Update Build > "Image Tag" (line 47): change default tag to `tentacular-engine:latest`
- [x] 2.4 Update Deploy > "Generated Manifests" (line 68): add ConfigMap YAML example (`{workflow-name}-code`). Add `code` volume and volumeMount to Deployment example
- [x] 2.5 Update Deploy > "Flags" table (line 135): replace `--cluster-registry` with `--image` flag. Add description for image resolution cascade
- [x] 2.6 Add new "Fast Iteration" section after Deploy: explain edit code → `tntc deploy` → ConfigMap updated + rollout restart → done (no Docker build needed)
- [x] 2.7 Update "Full Lifecycle" example (line 238): show `build` once for engine image, then `deploy` for code changes. Replace `--cluster-registry` with `--image`

## 3. Verification

- [x] 3.1 Verify no broken markdown links or formatting in architecture.md
- [x] 3.2 Verify no broken markdown links or formatting in deployment-guide.md
- [x] 3.3 Verify all YAML examples are syntactically valid
