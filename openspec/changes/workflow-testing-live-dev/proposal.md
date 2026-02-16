## Why

Workflow authors currently have no way to validate workflows against a real cluster before deploying to production. The mock test framework (`tntc test`) validates node contracts in isolation, but cannot catch environment-specific issues: missing secrets, broken network policies, incorrect RBAC, or cluster configuration problems. Developers also lack structured output for CI/CD integration, and the SEP tracker's `store-report` node carries dead report-insertion code that causes mock test failures.

## What Changes

- **WI-1: Store-Report Cleanup** -- Remove report-related code from `store-report` node. HTML reports belong in Azure Blob Storage only (not Postgres). Unblocks 5/5 mock tests.
- **WI-2: Environment Configuration** -- Extend the existing config cascade (`~/.tentacular/config.yaml`, `.tentacular/config.yaml`) with named environments (`dev`, `staging`, `prod`). Each environment specifies kubeconfig context, namespace, image, runtime class, config overrides, and secrets source.
- **WI-3: Kind Cluster Detection** -- Auto-detect kind clusters from kubeconfig context. Adjust deployment parameters: disable gVisor runtime class, set `imagePullPolicy=IfNotPresent`, auto-load images via `kind load docker-image`.
- **WI-4: Structured JSON Output** -- Consistent `--output json` / `-o json` envelope across `test`, `deploy`, and `run` commands. Agent-consumable format with version, status, summary, hints, and timing.
- **WI-5: Live Workflow Testing** -- `tntc test --live` deploys to a configured dev environment, triggers the workflow, validates execution results, and cleans up. The core integration testing feature.
- **WI-6: Deploy Gate** -- Default `tntc deploy` runs a live test first. `--force` skips it. Post-deploy verification with `--verify`. Structured output with phases: preflight, live-test, deploy, verify.
- **WI-7: Skill & Documentation Update** -- Update SKILL.md, testing guide, and deployment guide for the agentic deployment flow.

## Capabilities

### New Capabilities

- `environment-config`: Named environment definitions in config cascade with context, namespace, image, runtime class, config overrides, and secrets source.
- `kind-detection`: Auto-detection of kind clusters with deployment parameter adjustments.
- `structured-output`: JSON envelope for all commands with version, status, summary, hints, timing, and command-specific fields.
- `live-testing`: `tntc test --live` end-to-end workflow validation against real clusters.
- `deploy-gate`: Pre-deploy live test gate with force escape hatch.

### Modified Capabilities

- `testing-framework`: Mock tests unblocked by store-report cleanup. New `--live` flag for integration testing.
- `k8s-deploy`: Deploy gate integration, config override support, image pull policy configuration.
- `cli-foundation`: `-o json` flag on all commands, environment-aware deployment.

## Impact

- **Code**: `pkg/cli/test.go`, `pkg/cli/deploy.go`, `pkg/cli/build.go`, `pkg/cli/config.go`, `pkg/cli/run.go`, `pkg/k8s/client.go`, `pkg/builder/k8s.go`, `engine/testing/runner.ts`
- **New files**: `pkg/cli/environment.go`, `pkg/cli/output.go`, `pkg/cli/test_live.go`, `pkg/k8s/kind.go`
- **Example workflow**: `example-workflows/sep-tracker/nodes/store-report.ts`, `notify.ts`, test fixtures
- **Documentation**: `tentacular-skill/SKILL.md`, testing guide, deployment guide
- **Dependencies**: None beyond stdlib and existing deps.
- **Breaking**: None. All additions are optional flags and new capabilities.
