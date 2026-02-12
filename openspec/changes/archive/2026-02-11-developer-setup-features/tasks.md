## 1. T1-6: Version Tracking in K8s Metadata

- [x] 1.1 In `pkg/builder/k8s.go`, update the labels string at lines 82-83 (ConfigMap) to include `app.kubernetes.io/version: %s` using `wf.Version`. The format call for `labels` needs a second `%s` and `wf.Version` argument.
- [x] 1.2 In `pkg/builder/k8s.go`, update the labels string at lines 119-120 (Deployment/Service/CronJob) identically to include `app.kubernetes.io/version`.
- [x] 1.3 In `pkg/k8s/client.go`, add `Version string` field to `WorkflowInfo` struct (after `Namespace` field, around line 195)
- [x] 1.4 In `ListWorkflows()` (client.go:366), extract `dep.Labels["app.kubernetes.io/version"]` and set it on `WorkflowInfo.Version`
- [x] 1.5 In `pkg/cli/list.go`, update header format (line 49) to include VERSION column: `"%-24s %-8s %-16s %-10s %-10s %s\n"` with "VERSION" field
- [x] 1.6 In `pkg/cli/list.go`, update row format (line 56) to include `w.Version` in output
- [x] 1.7 Update `TestK8sManifestLabels` in `k8s_test.go` to assert `app.kubernetes.io/version: 1.0` is present
- [x] 1.8 Update `TestK8sManifestCronTriggerLabels` in `k8s_test.go` to assert version label on CronJob

## 2. T1-5: Per-Workflow Namespace in workflow.yaml

- [x] 2.1 In `pkg/spec/types.go`, add `DeploymentConfig` struct with `Namespace string` field
- [x] 2.2 In `pkg/spec/types.go`, add `Deployment DeploymentConfig` field to `Workflow` struct with `yaml:"deployment,omitempty"` tag
- [x] 2.3 Add `TestParseDeploymentNamespace` to `parse_test.go` -- verify `deployment.namespace` is parsed
- [x] 2.4 Add `TestParseNoDeploymentSection` to `parse_test.go` -- verify zero-value when absent

## 3. T1-4: `tntc configure` Command

- [x] 3.1 Create `pkg/cli/config.go` with `TentacularConfig` struct, `LoadConfig()`, and `mergeConfig()` functions
- [x] 3.2 Create `pkg/cli/configure.go` with `NewConfigureCmd()` and `runConfigure()` that writes config YAML
- [x] 3.3 In `cmd/tntc/main.go`, register `cli.NewConfigureCmd()` with `root.AddCommand()`
- [x] 3.4 Create `pkg/cli/config_test.go` with tests: `TestLoadConfigUserLevel`, `TestLoadConfigProjectOverridesUser`, `TestLoadConfigMissing`, `TestMergeConfigPartial`

## 4. Config Integration with Existing Commands

- [x] 4.1 In `pkg/cli/deploy.go` `runDeploy()`, implement namespace cascade: after getting namespace flag, check `Changed("namespace")`, then fall back to `wf.Deployment.Namespace`, then `LoadConfig().Namespace`
- [x] 4.2 In `pkg/cli/deploy.go` `runDeploy()`, apply `LoadConfig().RuntimeClass` when `--runtime-class` is not Changed
- [x] 4.3 In `pkg/cli/build.go` `runBuild()`, apply `LoadConfig().Registry` when `--registry` is not Changed

## 5. Verification

- [x] 5.1 Run `go test ./pkg/builder/ -run TestK8sManifestLabels` -- version label tests pass
- [x] 5.2 Run `go test ./pkg/spec/ -run TestParse` -- deployment namespace tests pass
- [x] 5.3 Run `go test ./pkg/cli/ -run TestLoadConfig` -- config loading tests pass
- [x] 5.4 Run full `go test ./pkg/...` to confirm no regressions
