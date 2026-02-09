## Why

The Go CLI codebase has 6 tests covering only the spec parser. The builder (K8s manifests, Dockerfile), CLI deploy secrets provisioning, and K8s preflight check serialization have zero automated test coverage. This change adds comprehensive tests for these packages to catch regressions and validate the security-critical manifest generation logic.

## What Changes

- **`pkg/builder/k8s_test.go`** — 18 tests covering `GenerateK8sManifests()` and `GenerateDockerfile()`: security contexts, probes, RuntimeClass, labels, volumes, resources, service, image tags, namespace, Dockerfile contents
- **`pkg/cli/deploy_secrets_test.go`** — 12 tests covering `buildSecretManifest`, `buildSecretFromDir`, `buildSecretFromYAML`: cascade precedence, hidden file exclusion, subdirectory exclusion, empty handling, whitespace trimming, YAML parsing errors
- **`pkg/k8s/preflight_test.go`** — 3 tests covering `CheckResultsJSON` serialization: warning field omitempty behavior, round-trip JSON parsing

## Capabilities

### New Capabilities
- `go-test-builder`: Automated testing of K8s manifest generation including security hardening fields (runAsNonRoot, readOnlyRootFilesystem, capabilities drop, seccomp), probe configuration, gVisor RuntimeClass conditional inclusion, and Dockerfile structure
- `go-test-deploy-secrets`: Automated testing of secret provisioning cascade (directory vs YAML, precedence), filesystem edge cases (hidden files, subdirs, empty dirs, whitespace), and YAML parse error handling
- `go-test-preflight`: Automated testing of CheckResult JSON serialization including omitempty behavior and round-trip parsing

### Modified Capabilities
_(none — these are new test files)_

## Impact

- **New files**: `pkg/builder/k8s_test.go`, `pkg/cli/deploy_secrets_test.go`, `pkg/k8s/preflight_test.go`
- **Modified files**: none
- **Test count**: 6 existing → 39 total (33 new)
- **Dependencies**: none (uses only stdlib `testing`, `strings`, `os`, `encoding/json`)
