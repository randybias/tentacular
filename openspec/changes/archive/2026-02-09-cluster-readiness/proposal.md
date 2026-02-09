## Why

Before deploying workflows to Kubernetes, operators need confidence that the target cluster meets all prerequisites. Today, failures surface only at deploy time with opaque K8s API errors. A dedicated preflight check command (`pipedreamer cluster check`) provides clear, actionable pass/fail output before any deployment attempt, reducing debugging time and preventing partial deployments into misconfigured clusters.

## What Changes

- **New `pipedreamer cluster check` CLI command** — runs a suite of preflight validations against the target Kubernetes cluster and reports pass/fail with remediation guidance
- **K8s API reachability check** — verifies the cluster API server is reachable via kubeconfig or in-cluster config
- **gVisor RuntimeClass check** — confirms the `gvisor` RuntimeClass exists (required for Fortress deployment pattern sandbox isolation)
- **Namespace existence check** — verifies the target namespace exists in the cluster
- **RBAC permissions check** — validates the current service account has permissions to manage Deployments, Services, ConfigMaps, and Secrets in the target namespace
- **Secret references check** — confirms that secrets referenced in workflow specs resolve to existing K8s Secrets
- **`--fix` flag** — optional flag to auto-create the namespace if it does not exist
- **Structured output** — pass/fail results with remediation messages, supporting both text and JSON output formats via global `--output` flag

## Capabilities

### New Capabilities
- `cluster-check`: Preflight validation command that checks K8s API reachability, gVisor RuntimeClass, namespace existence, RBAC permissions, and secret references, with clear pass/fail output and remediation messages

### Modified Capabilities
- `cli-foundation`: Replaces the `cluster check` stub command with a fully functional implementation

## Impact

- **Modified files**: `pkg/cli/cluster.go` (replace stub with full implementation), `pkg/k8s/preflight.go` (expand preflight checks)
- **New files**: none expected (existing stubs cover the file structure)
- **Dependencies**: `k8s.io/client-go` (already present), `k8s.io/apimachinery` (already present)
- **No breaking changes**: This fills in an existing command stub; no API surface changes
- **Downstream enablement**: Build/deploy commands can call preflight checks before deploying to catch misconfigurations early
