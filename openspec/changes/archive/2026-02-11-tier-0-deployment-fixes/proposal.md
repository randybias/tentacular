## Why

Three deployment-blocking bugs prevent correct workflow deployment on Kubernetes: secrets YAML parsing breaks on nested maps (required by the engine's JSON-based secret loading), generated Deployments use the default `IfNotPresent` pull policy causing stale images on re-push, and the generated Dockerfile's `deno cache` command is missing the `--no-lock` flag causing build failures. These are Tier 0 items that block any new developer from deploying their first workflow.

## What Changes

- **T0-1: Nested secrets YAML support** -- `buildSecretFromYAML()` changes from `map[string]string` to `map[string]interface{}` with JSON serialization for nested values. Flat string values remain as-is. Adds `encoding/json` import to `deploy.go`.
- **T0-2: ImagePullPolicy in generated Deployments** -- Add `imagePullPolicy: Always` to the Deployment container spec template in `k8s.go`, ensuring cluster nodes always pull the latest image for a given tag.
- **T0-3: --no-lock on deno cache** -- Add the missing `--no-lock` flag to the `deno cache` RUN command in the generated Dockerfile template (`dockerfile.go`), matching the existing ENTRYPOINT which already has it.

## Capabilities

### New Capabilities

(none -- all changes are fixes to existing capabilities)

### Modified Capabilities

- `k8s-deploy`: Generated Deployment manifests now include `imagePullPolicy: Always` on the engine container. Generated K8s Secret manifests now handle nested YAML maps by JSON-serializing them into stringData values (matching engine's `loadSecretsFromDir()` JSON parsing contract).
- `base-engine-image`: Generated Dockerfile `deno cache` command now includes `--no-lock` flag, consistent with the ENTRYPOINT `deno run` command.

## Impact

- **Code**: `pkg/cli/deploy.go` (secret builder), `pkg/builder/k8s.go` (Deployment template), `pkg/builder/dockerfile.go` (Dockerfile template)
- **Tests**: `pkg/cli/deploy_secrets_test.go` (3 new tests), `pkg/builder/k8s_test.go` (1 new test), `pkg/builder/dockerfile_test.go` (1 new test)
- **APIs**: No API changes. All changes are to generated K8s manifests and Docker build artifacts.
- **Dependencies**: New `encoding/json` import in `deploy.go` (stdlib only).
- **Breaking**: None. Flat string secrets still work identically. The ImagePullPolicy and --no-lock additions are purely additive to generated output.
