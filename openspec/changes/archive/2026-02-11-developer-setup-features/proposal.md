## Why

New developers need to repeatedly specify the same CLI flags (registry, namespace, runtime class) on every `tntc build` and `tntc deploy` invocation. Workflows lack a way to declare their intended deployment namespace, and deployed workflows have no version tracking in K8s metadata, making it impossible to tell what version is running via `tntc list`. These Tier 1 items remove friction from the developer setup and deployment workflow.

## What Changes

- **T1-4: `tntc configure` command** -- New `configure` command writes defaults to `~/.tentacular/config.yaml` (user-level) or `.tentacular/config.yaml` (project-level). Existing commands (`build`, `deploy`) read these defaults when flags are not explicitly set, using `cmd.Flags().Changed()` to detect explicit flag usage. New files: `pkg/cli/config.go`, `pkg/cli/configure.go`. Modified: `cmd/tntc/main.go`, `pkg/cli/deploy.go`, `pkg/cli/build.go`.
- **T1-5: Per-workflow namespace in workflow.yaml** -- Add `deployment.namespace` field to workflow spec. Namespace resolution cascade: CLI `-n` flag > `workflow.yaml` deployment.namespace > config file default > "default". Modified: `pkg/spec/types.go`, `pkg/cli/deploy.go`.
- **T1-6: Version tracking in K8s metadata** -- Add `app.kubernetes.io/version` label to all generated K8s resources (ConfigMap, Deployment, Service, CronJob). Surface version in `tntc list` output. Modified: `pkg/builder/k8s.go`, `pkg/k8s/client.go`, `pkg/cli/list.go`.

## Capabilities

### New Capabilities

- `cli-configure`: The `tntc configure` command and two-tier config loading system (user-level + project-level YAML config files with merge semantics).

### Modified Capabilities

- `workflow-spec`: Add optional `deployment` section with `namespace` field to `workflow.yaml` schema.
- `k8s-deploy`: Generated K8s resources include `app.kubernetes.io/version` label from workflow version. Namespace resolution cascade incorporates workflow.yaml and config file defaults.
- `cli-foundation`: `tntc list` output includes version column. `tntc build` and `tntc deploy` read config file defaults for registry, namespace, and runtime-class when flags are not explicitly set.

## Impact

- **Code**: `pkg/cli/deploy.go`, `pkg/cli/build.go`, `pkg/builder/k8s.go`, `pkg/k8s/client.go`, `pkg/cli/list.go`, `pkg/spec/types.go`, `cmd/tntc/main.go`
- **New files**: `pkg/cli/config.go`, `pkg/cli/config_test.go`, `pkg/cli/configure.go`
- **Tests**: `pkg/cli/config_test.go` (4 new tests), `pkg/builder/k8s_test.go` (update 2 existing tests), `pkg/spec/parse_test.go` (2 new tests)
- **APIs**: New `deployment` section in workflow.yaml schema (optional, non-breaking). New `tntc configure` command.
- **Dependencies**: None beyond stdlib.
- **Breaking**: None. All additions are optional fields and new commands.
