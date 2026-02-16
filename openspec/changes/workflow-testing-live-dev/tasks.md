## 1. WI-1: Store-Report Cleanup

- [x] 1.1 In `example-workflows/sep-tracker/nodes/store-report.ts`, remove `sep_reports` table from `CREATE_TABLES` SQL
- [x] 1.2 Remove `INSERT_REPORT` SQL constant
- [x] 1.3 Remove `reportId` from `StoreResult` interface
- [x] 1.4 Remove report insertion logic (lines 126-134)
- [x] 1.5 Return type becomes `{ stored: boolean; snapshotId: number; reportUrl: string }`
- [x] 1.6 In `example-workflows/sep-tracker/nodes/notify.ts`, remove `reportId` from `StoreResult` interface and update storage line
- [x] 1.7 Update `tests/fixtures/store-report.json` -- remove `"reportId": 0` from `expected`
- [x] 1.8 Update `tests/fixtures/notify.json` -- remove `"reportId": 1` from `store-report` input
- [x] 1.9 Verify: `tntc test example-workflows/sep-tracker` passes 5/5

## 2. WI-2: Environment Configuration

- [x] 2.1 In `pkg/cli/config.go`, add `EnvironmentConfig` struct with fields: Context, Namespace, Image, RuntimeClass, ConfigOverrides, SecretsSource
- [x] 2.2 Add `Environments map[string]EnvironmentConfig` field to `TentacularConfig`
- [x] 2.3 Add `LoadEnvironment(name string) (*EnvironmentConfig, error)` to config.go
- [x] 2.4 Update `mergeConfig` to handle environments map
- [x] 2.5 Create `pkg/cli/environment.go` with `ResolveEnvironment()` and `ApplyConfigOverrides()`
- [x] 2.6 Add tests in `pkg/cli/config_test.go`: load with environments, project overrides user, environment not found, config overrides merge

## 3. WI-3: Kind Cluster Detection

- [x] 3.1 Create `pkg/k8s/kind.go` with `ClusterInfo` struct: IsKind, ClusterName, Context
- [x] 3.2 Implement `DetectKindCluster() (*ClusterInfo, error)` -- reads kubeconfig, checks "kind-" prefix + localhost server
- [x] 3.3 Implement `LoadImageToKind(imageName, clusterName string) error` -- shells out to `kind load docker-image`
- [x] 3.4 In `pkg/builder/k8s.go`, add `ImagePullPolicy string` to `DeployOptions` and use in Deployment template
- [x] 3.5 In `pkg/cli/deploy.go`, after creating K8s client, call `DetectKindCluster()`. If kind: set runtimeClass="", imagePullPolicy="IfNotPresent"
- [x] 3.6 In `pkg/cli/build.go`, after building, detect kind and auto `kind load docker-image`
- [x] 3.7 Add `TestDetectKindCluster` and `TestDetectNonKindCluster` to `pkg/k8s/kind_test.go`

## 4. WI-4: Structured JSON Output

- [x] 4.1 Create `pkg/cli/output.go` with `CommandResult` struct (envelope: version, command, status, summary, hints, timing)
- [x] 4.2 Implement `EmitResult(cmd, result) error` -- checks `-o` flag, emits JSON or text
- [x] 4.3 In `engine/testing/runner.ts`, add `--json` flag that outputs `TestResult[]` as JSON to stdout
- [x] 4.4 In `pkg/cli/test.go`, pass `--json` to Deno runner when `-o json`, capture stdout, wrap in `CommandResult` envelope
- [x] 4.5 In `pkg/cli/deploy.go`, collect manifest actions into structured list, emit via `EmitResult`
- [x] 4.6 In `pkg/cli/run.go`, parse `ExecutionResult` from engine, wrap in `CommandResult` envelope with hints

## 5. WI-5: Live Workflow Testing (tntc test --live)

- [x] 5.1 In `pkg/cli/test.go`, add flags: `--live`, `--env` (default "dev"), `--keep`, `--timeout` (default 120s)
- [x] 5.2 When `--live`: call `runLiveTest` instead of Deno runner
- [x] 5.3 Create `pkg/cli/test_live.go` with `runLiveTest(cmd, args) error` implementing the full live test flow
- [x] 5.4 In `pkg/cli/deploy.go`, extract `deployWorkflow(dir, namespace, opts) (*DeployResult, error)` from `runDeploy`
- [x] 5.5 In `pkg/k8s/client.go`, add `WaitForReady(ctx, namespace, name, timeout) error` -- polls until ReadyReplicas == Replicas
- [x] 5.6 In `pkg/k8s/client.go`, add `NewClientWithContext(contextName string) (*Client, error)` -- uses explicit kubeconfig context
- [x] 5.7 In `pkg/builder/k8s.go`, accept config overrides in ConfigMap builder for env-specific config values
- [x] 5.8 Integrate: load env config -> switch context -> detect kind -> deploy -> wait ready -> trigger -> parse result -> cleanup -> emit

## 6. WI-6: Deploy Gate + Force Escape Hatch

- [ ] 6.1 In `pkg/cli/deploy.go`, add `--force` flag (alias `--skip-live-test`)
- [ ] 6.2 Before deploy: check if dev environment is configured. If configured and not --force: run live test first
- [ ] 6.3 If live test fails: abort with structured error including hints
- [ ] 6.4 Add `--verify` flag (default true with `-o json`)
- [ ] 6.5 After deploy: run workflow once, validate result
- [ ] 6.6 Emit structured output with phases: [preflight, live-test, deploy, verify]

## 7. WI-7: Skill & Documentation Update

- [ ] 7.1 In `tentacular-skill/SKILL.md`, add Deployment Flow section with 5-step agentic flow
- [ ] 7.2 Document structured output envelope and per-command schemas
- [ ] 7.3 Document environment config format
- [ ] 7.4 Document `--force` escape hatch
- [ ] 7.5 In `tentacular-skill/references/testing-guide.md`, add live testing documentation
- [ ] 7.6 In `tentacular-skill/references/deployment-guide.md`, add environment config, kind detection, deploy gate docs
