## 1. Builder Test File

- [x] 1.1 Create `pkg/builder/k8s_test.go` with `makeTestWorkflow` helper
- [x] 1.2 Add manifest count and kind tests (TestGenerateK8sManifestsReturnsTwoManifests)
- [x] 1.3 Add pod security context tests (runAsNonRoot, runAsUser, seccompProfile)
- [x] 1.4 Add container security context tests (readOnlyRootFilesystem, allowPrivilegeEscalation, capabilities drop)
- [x] 1.5 Add probe tests (liveness: /health, 8080, delay 5, period 10; readiness: delay 3, period 5)
- [x] 1.6 Add RuntimeClass conditional tests (included when set, omitted when empty)
- [x] 1.7 Add label, volume, resource, service, image tag, and namespace tests
- [x] 1.8 Add Dockerfile tests (distroless base, workdir, copy instructions, entrypoint, no CLI artifacts)

## 2. Deploy Secrets Test File

- [x] 2.1 Create `pkg/cli/deploy_secrets_test.go` with same-package access to unexported functions
- [x] 2.2 Add buildSecretFromDir tests (multiple files, hidden skipped, subdirs skipped, empty, trimming)
- [x] 2.3 Add buildSecretFromYAML tests (valid, empty, invalid)
- [x] 2.4 Add buildSecretManifest cascade tests (dir preferred, YAML fallback, no secrets, secret name)

## 3. Preflight Test File

- [x] 3.1 Create `pkg/k8s/preflight_test.go`
- [x] 3.2 Add CheckResultsJSON tests (warning present, warning omitted, all fields round-trip)

## 4. Verification

- [x] 4.1 Run `go test ./pkg/...` — 39 tests passing (6 spec + 18 builder + 12 secrets + 3 preflight)
- [x] 4.2 Verify `go build ./cmd/pipedreamer/` still compiles
