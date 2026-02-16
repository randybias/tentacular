## Context

The tentacular CLI has a working mock test framework (`tntc test`) that validates node contracts in isolation using a mock context. However, there is no way to validate workflows against real clusters. This change adds environment configuration, kind cluster detection, structured JSON output, and live workflow testing -- enabling a complete development-to-deployment validation pipeline.

Existing infrastructure:
- Config cascade in `pkg/cli/config.go`: `TentacularConfig` struct, `LoadConfig()`, `mergeConfig()` with user-level and project-level YAML.
- K8s client in `pkg/k8s/client.go`: `Client` struct, `ListWorkflows()`, kubeconfig integration.
- Deploy pipeline in `pkg/cli/deploy.go`: `runDeploy()` with build, manifest generation, and kubectl apply.
- Test runner in `pkg/cli/test.go`: invokes Deno test engine via `engine/testing/runner.ts`.
- SEP tracker example workflow with 5 nodes including `store-report` (carries dead report-insertion code).

## Goals / Non-Goals

**Goals:**

- Remove dead report code from store-report to unblock mock tests
- Add named environments to config cascade (dev, staging, prod)
- Auto-detect kind clusters and adjust deployment parameters
- Provide consistent `--output json` envelope across all commands
- Implement `tntc test --live` for end-to-end workflow validation
- Gate default deploys behind live test pass (with `--force` escape hatch)
- Update skill documentation for the agentic deployment flow

**Non-Goals:**

- Cluster provisioning (kind, minikube, etc.) -- "bring your own cluster"
- Remote cluster management or multi-cluster orchestration
- Test parallelism or test sharding
- Workflow rollback or canary deployment strategies
- Secret rotation or secret management beyond config reference

## Decisions

### D1: Environment config extends existing cascade

Named environments are added as an `environments` map in the existing config YAML files. Each environment specifies: `context` (kubeconfig context name), `namespace`, `image`, `runtime_class`, `config_overrides` (map merged into workflow config), and `secrets_source`. Resolution: `LoadEnvironment(name)` calls `LoadConfig()` then looks up `environments[name]`. Environment-level fields override top-level defaults. This extends the D1 decision from the developer-setup-features change.

**Why map, not separate files:** Environments are lightweight (5-6 fields each). Separate files add directory management complexity without benefit. The map keeps all environments visible in one file.

### D2: Kind detection via kubeconfig heuristics

`DetectKindCluster()` reads the current kubeconfig context. Detection criteria: context name has `kind-` prefix AND server address is `localhost` or `127.0.0.1`. When detected: `runtimeClass` is forced to empty string (no gVisor in kind), `imagePullPolicy` is set to `IfNotPresent`, and `kind load docker-image` is called after build.

**Why not `kind get clusters`:** Shelling out to `kind` CLI requires kind to be installed. Kubeconfig heuristics work without the kind binary (except for image loading, which does require it).

### D3: Structured output envelope

All commands emit a `CommandResult` envelope when `-o json` is set:
```
{version, command, status, summary, hints[], timing{startedAt, durationMs}}
```
Plus command-specific fields. `EmitResult(cmd, result)` checks the `-o` flag and emits JSON or falls through to existing text output. The Deno test runner adds a `--json` flag that outputs `TestResult[]` to stdout, which the Go CLI wraps in the envelope.

**Why envelope, not raw results:** Agents need consistent metadata (timing, status, hints) regardless of command. The envelope provides this without each command implementing its own JSON format.

### D4: Live test flow

`tntc test --live` executes this pipeline:
1. Load environment config (default: `dev`)
2. Switch kubeconfig context to environment's context
3. Detect kind cluster (adjust params if needed)
4. Deploy workflow to environment namespace
5. Wait for deployment Ready (poll until ReadyReplicas == Replicas, with timeout)
6. Trigger workflow execution
7. Parse ExecutionResult from engine
8. Cleanup deployed resources (unless `--keep`)
9. Emit structured result

The deploy step reuses an extracted `deployWorkflow()` function from `runDeploy`. This avoids duplicating the build/manifest/apply pipeline.

### D5: Deploy gate integration

Default `tntc deploy` behavior changes: if a dev environment is configured and `--force` is not set, a live test runs before the production deploy. If the live test fails, deploy aborts with a structured error including hints. `--force` (alias `--skip-live-test`) bypasses the gate. Post-deploy `--verify` triggers a single workflow execution to validate the deployment. Structured output includes phases: `[preflight, live-test, deploy, verify]`.

**Why default-on:** The gate prevents deploying broken workflows. Developers who know what they're doing use `--force`. This follows the "safe by default" principle.

## Risks / Trade-offs

**Kind detection false positives** -- A non-kind cluster using localhost could be misdetected. Mitigation: the `kind-` context name prefix is a strong signal. Users can override via environment config.

**Deploy gate adds latency** -- Every deploy now includes a live test cycle. Mitigation: `--force` skips it. The live test targets a lightweight dev environment, not production.

**Context switching side effects** -- `NewClientWithContext()` changes the active kubeconfig context. Mitigation: the function creates a new client with an explicit context override, not modifying the shared kubeconfig state.

**Cleanup failure on live test** -- If cleanup fails after live test, resources remain in the dev namespace. Mitigation: `--keep` makes this intentional. Failed cleanups emit warnings, not errors.
