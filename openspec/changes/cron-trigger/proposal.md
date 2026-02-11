## Why

Cron triggers are defined in the spec and validated by the parser, but never executed. The engine only handles HTTP (`POST /run`). We need `tntc deploy` to generate K8s CronJob manifests for cron triggers, and `tntc undeploy` to clean them up. This also supports named triggers with different schedules and behavior in one workflow.

## What Changes

- Add `Name` field to `Trigger` struct for named triggers
- Validate trigger names are unique and match `identRe`
- Generate CronJob manifests during `tntc deploy` for each cron trigger
- Add CronJob resource support to K8s client (findResource, label-based deletion)
- Add RBAC preflight checks for `batch/cronjobs` and `batch/jobs`
- Accept POST body on `/run` endpoint and pass as input to executor (for trigger payloads)
- Auto-run preflight checks before deploy
- Add `--allow-env` to Dockerfile ENTRYPOINT

## Capabilities

### New Capabilities
- `cron-trigger`: Generate K8s CronJob manifests for cron-type triggers, with named trigger support and POST body passthrough

### Modified Capabilities
<!-- None -->

## Impact

- `pkg/spec/types.go`: Trigger gains Name field
- `pkg/spec/parse.go`: Trigger name validation
- `pkg/builder/k8s.go`: CronJob manifest generation
- `pkg/builder/k8s_test.go`: CronJob tests
- `pkg/k8s/client.go`: CronJob resource mapping and deletion
- `pkg/k8s/preflight.go`: RBAC checks for batch resources
- `engine/server.ts`: POST body passthrough to executor
- `engine/executor/simple.ts`: Accept initial input for root nodes
- `engine/executor/types.ts`: Updated execute signature
- `pkg/cli/deploy.go`: Auto-preflight before apply
- `pkg/builder/dockerfile.go`: --allow-env permission
