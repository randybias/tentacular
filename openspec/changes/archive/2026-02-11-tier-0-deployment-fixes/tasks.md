## 1. T0-1: Nested Secrets YAML Support

- [x] 1.1 In `pkg/cli/deploy.go`, add `"encoding/json"` to the import block
- [x] 1.2 In `buildSecretFromYAML()` (deploy.go:221), change `var secrets map[string]string` to `var secrets map[string]interface{}`
- [x] 1.3 Replace the `for k, v := range secrets` loop (deploy.go:231-233) with a type switch: string values use as-is, all other types get `json.Marshal`'d. Propagate `json.Marshal` errors rather than ignoring them.
- [x] 1.4 Add `TestBuildSecretFromYAMLNested` to `deploy_secrets_test.go` -- nested map `slack: { webhook_url: "..." }` produces JSON stringData
- [x] 1.5 Add `TestBuildSecretFromYAMLMixed` to `deploy_secrets_test.go` -- mix of flat strings and nested maps
- [x] 1.6 Add `TestBuildSecretFromYAMLDeeplyNested` to `deploy_secrets_test.go` -- multi-level nesting
- [x] 1.7 Verify existing tests still pass: `TestBuildSecretFromYAMLValid`, `TestBuildSecretFromYAMLInvalid`, `TestBuildSecretFromYAMLEmpty`

## 2. T0-2: ImagePullPolicy in Generated Deployments

- [x] 2.1 In `pkg/builder/k8s.go`, insert `imagePullPolicy: Always` line after `image: %s` (line 174) in the Deployment template string, with 10-space indent to match surrounding fields
- [x] 2.2 Add `TestK8sManifestImagePullPolicy` to `k8s_test.go` -- assert `imagePullPolicy: Always` is present in generated Deployment
- [x] 2.3 Verify existing tests still pass (especially `TestK8sManifestContainerSecurityContext`, `TestK8sManifestImageTag`)

## 3. T0-3: --no-lock on deno cache in Dockerfile

- [x] 3.1 In `pkg/builder/dockerfile.go` line 17, change `RUN ["deno", "cache", "engine/main.ts"]` to `RUN ["deno", "cache", "--no-lock", "engine/main.ts"]`
- [x] 3.2 Add `TestGenerateDockerfileNoLockOnCache` to `dockerfile_test.go` -- verify the `deno cache` line includes `--no-lock`
- [x] 3.3 Verify existing tests still pass (especially `TestDockerfileCacheAndEntrypoint`, `TestGenerateDockerfile_Entrypoint`)

## 4. Verification

- [x] 4.1 Run `go test ./pkg/cli/ -run TestBuildSecretFromYAML` -- all 6+ tests pass
- [x] 4.2 Run `go test ./pkg/builder/ -run TestK8sManifest` -- all tests pass including new ImagePullPolicy test
- [x] 4.3 Run `go test ./pkg/builder/ -run TestGenerateDockerfile` -- all tests pass including new --no-lock test
- [x] 4.4 Run full `go test ./pkg/...` to confirm no regressions
