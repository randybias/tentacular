# Tasks

## Implementation

- [x] Remove CronJob manifest generation from `pkg/builder/k8s.go`
- [x] Remove proxy-prewarm init container generation from `pkg/builder/k8s.go`
- [x] Add `tentacular.dev/cron-schedule` annotation to Deployment in `pkg/builder/k8s.go`
- [x] Add control-plane ingress rule to NetworkPolicy (10.0.0.0/8:8080) in `pkg/builder/k8s.go`
- [x] Add `services/proxy` to MCP server ClusterRole in `mcp_deploy.go`
- [x] Remove dead `RunWorkflow` function from `pkg/k8s/client.go`
- [x] Remove `GenerateTriggerNetworkPolicy` from `pkg/k8s/netpol.go`
- [x] Remove trigger NetworkPolicy call from `pkg/cli/deploy.go`

## Testing

- [x] Update all tests to reflect new manifest structure (no CronJob, no init container, cron annotation present)
- [x] Verify control-plane ingress rule present in generated NetworkPolicy
- [x] Verify `tentacular.dev/cron-schedule` annotation present for cron triggers
- [x] Confirm go test -count=1 ./... passes
