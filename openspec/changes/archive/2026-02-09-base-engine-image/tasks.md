## 1. Dockerfile Generation

- [x] 1.1 Change `GenerateDockerfile(workflowDir string) string` to `GenerateDockerfile() string` in `pkg/builder/dockerfile.go` — remove the `workflowDir` parameter and all `nodes/` scanning logic (cacheLines, os.ReadDir, etc.)
- [x] 1.2 Remove `COPY workflow.yaml /app/` and `COPY nodes/ /app/nodes/` from the generated Dockerfile template
- [x] 1.3 Remove the per-node `deno cache` section from the generated Dockerfile (the `cacheLines` variable concatenation)
- [x] 1.4 Add `ENV DENO_DIR=/tmp/deno-cache` to the generated Dockerfile (before the EXPOSE line)
- [x] 1.5 Change the ENTRYPOINT `--workflow` path from `/app/workflow.yaml` to `/app/workflow/workflow.yaml`
- [x] 1.6 Verify the ENTRYPOINT preserves existing permissions: `--allow-net`, `--allow-read=/app,/var/run/secrets`, `--allow-write=/tmp`, `--allow-env`

## 2. Build Command Updates

- [x] 2.1 Update `build.go` to call `builder.GenerateDockerfile()` with no arguments (line 73)
- [x] 2.2 Change default tag from `wf.Name + ":" + version` to `tentacular-engine:latest` when `--tag` is not specified
- [x] 2.3 After successful build (and push if requested), write the image tag to `.tentacular/base-image.txt` — create `.tentacular/` directory with `os.MkdirAll` if it doesn't exist
- [x] 2.4 Remove the unused `fmt`, `os`, `path/filepath`, `strings` imports if they become unreferenced after the tag derivation change (keep only what's needed)

## 3. Update Existing Tests

- [x] 3.1 Update `TestDockerfileDistrolessBase` in `k8s_test.go` — change `GenerateDockerfile("")` to `GenerateDockerfile()`
- [x] 3.2 Update `TestDockerfileWorkdir` — change call signature to `GenerateDockerfile()`
- [x] 3.3 Update `TestDockerfileCopyInstructions` — change call to `GenerateDockerfile()` and update assertions: assert `.engine/` and `deno.json` present, assert `workflow.yaml` and `nodes/` are ABSENT
- [x] 3.4 Update `TestDockerfileCacheAndEntrypoint` — change call to `GenerateDockerfile()` and update assertion for `--workflow /app/workflow/workflow.yaml` (new path)
- [x] 3.5 Update `TestDockerfileNoCLIArtifacts` — change call to `GenerateDockerfile()`

## 4. New Dedicated Test File

- [x] 4.1 Create `pkg/builder/dockerfile_test.go` with `TestGenerateDockerfile_NoWorkflowCopy` — assert Dockerfile does NOT contain `COPY workflow.yaml` or `COPY nodes/`
- [x] 4.2 Add `TestGenerateDockerfile_EngineOnly` — assert Dockerfile contains `COPY .engine/` and `COPY .engine/deno.json` and does NOT contain workflow/node references
- [x] 4.3 Add `TestGenerateDockerfile_Entrypoint` — assert ENTRYPOINT contains `--workflow /app/workflow/workflow.yaml`, `--port 8080`, `--allow-net`, `--allow-env`, `--allow-read=/app,/var/run/secrets`, `--allow-write=/tmp`
- [x] 4.4 Add `TestGenerateDockerfile_DenoDir` — assert Dockerfile contains `ENV DENO_DIR=/tmp/deno-cache`

## 5. Verification

- [x] 5.1 Run `go test ./pkg/builder/...` and verify all tests pass
- [x] 5.2 Run `go test ./pkg/cli/...` and verify build tests still compile (call signature change)
- [x] 5.3 Run `go build -o tntc ./cmd/tntc/` and verify the binary compiles
- [x] 5.4 Verify `.tentacular/` is in `.gitignore` — add if not present
