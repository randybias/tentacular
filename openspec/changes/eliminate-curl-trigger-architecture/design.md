# Design: Eliminate curl-based trigger architecture

## Architecture Decision: MCP Server as Control Plane

The MCP server (`tentacular-mcp`) is the control plane for all workflow
operations. It runs in-cluster with a scoped ClusterRole that grants it
read/write access to the Kubernetes API. This makes it the natural owner of
all trigger behavior:

- **Manual triggers (`wf_run`)**: MCP server POSTs directly to the workflow's
  ClusterIP service via HTTP.
- **Cron triggers**: MCP server's internal scheduler reads `tentacular.dev/cron-schedule`
  annotations and fires workflows on schedule.
- **Module pre-warm**: MCP server extracts JSR/npm dependencies from the workflow
  ConfigMap and warms the esm.sh module proxy cache in a background goroutine
  after `wf_apply` returns.

No ephemeral pods are created for any of these operations.

## Direct HTTP Pattern for wf_run

The MCP server triggers workflows via direct HTTP to the workflow's ClusterIP
service:

```
POST http://{svc}.{ns}.svc.cluster.local:8080/run
```

The MCP server (running in tentacular-system) connects directly to the workflow
Service's port 8080. The workflow engine receives a standard HTTP POST to `/run`
and returns a JSON response. No K8s API service proxy is involved.

Advantages over ephemeral trigger pods:
- No pod scheduling, image pull, or container lifecycle overhead.
- No `curlimages/curl` dependency.
- No ipset sync race with kube-router -- direct pod-to-service networking is
  stable and does not depend on API server proxy ipsets.
- Auth is handled by the MCP server's in-cluster ServiceAccount, not a
  hard-coded curl command.
- No `services/proxy` RBAC permission required (removed from ClusterRole).

The NetworkPolicy must allow ingress from the tentacular-system namespace
(via namespaceSelector) on TCP 8080. This rule is added to all generated
NetworkPolicies.

## Internal Cron Scheduler

The MCP server uses `robfig/cron/v3` for internal scheduling. On startup, the
scheduler scans all Deployments in tentacular-managed namespaces that have the
`tentacular.dev/cron-schedule` annotation and registers a cron entry for each.

The cron entry fires `wf_run` internally (same code path as the MCP tool).

Cron schedule format: standard 5-field cron expression (e.g., `"0 9 * * *"`).
Multiple schedules for one workflow are encoded as a JSON array in the annotation:
`tentacular.dev/cron-schedule: '["0 9 * * *","0 * * * *"]'`.

Named cron triggers are encoded as a JSON array of objects:
`tentacular.dev/cron-schedule: '[{"schedule":"0 9 * * *","name":"daily"}]'`.

The cron scheduler re-syncs when `wf_apply` or `wf_remove` is called, so
new deployments and undeployments are reflected without restarting the MCP server.

## Module Pre-Warm: Background Goroutine in wf_apply

When `wf_apply` is called, the MCP server inspects the ConfigMap manifest for
`workflow.yaml` content. It extracts any `jsr:` or `npm:` import specifiers
from the workflow's node files and triggers warming of those modules in the
esm.sh module proxy.

Warming is performed in a background goroutine -- `wf_apply` returns immediately
without waiting for warming to complete.

### Pre-Warm Race Condition

There is a known race condition between module warming and pod startup:

1. `wf_apply` applies manifests and returns (warming starts in background).
2. K8s schedules and starts the workflow pod.
3. The Deno engine loads node modules at startup by fetching from esm.sh proxy.
4. If the proxy has not yet cached the modules, the fetch may time out.
5. The pod fails to start (CrashLoopBackOff or timeout error).

**Recovery**: K8s restart policy (`Always` or `OnFailure`) handles this
automatically. By the time the pod restarts (backoff ~10s), warming will
typically have completed. The second pod start succeeds.

**Mitigation**: Pre-warm is best-effort. For workflows with many or large
module dependencies, the window is longer. Users can check `proxy_status` to
confirm the proxy is healthy before deploying.

**Not a regression**: The previous `proxy-prewarm` init container had the same
race in a different form -- if the init container's curl timed out before the
proxy was warm, the pod would fail to start anyway. The new approach trades a
blocking init container (which delayed pod readiness) for an async background
goroutine (which is faster overall but has a small first-start race).

## Control-Plane Ingress in NetworkPolicy

All generated NetworkPolicies include an ingress rule:

```yaml
- from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: tentacular-system
  ports:
    - protocol: TCP
      port: 8080
```

This allows the MCP server (running in tentacular-system) to reach the
workflow engine's HTTP server via direct HTTP. It is required for:
1. `wf_run` -- direct HTTP from the MCP server to the workflow Service.
2. Cron triggers -- same direct HTTP path as `wf_run`.

Using namespaceSelector instead of `ipBlock: 10.0.0.0/8` is more precise
and works correctly regardless of cluster CIDR configuration.

## MCP Server ClusterRole: services/proxy Removed

The MCP server ClusterRole (`tentacular-mcp`) no longer requires
`services/proxy` in the `""` (core) API group. The previous API service
proxy pattern required this RBAC permission, but with direct HTTP the MCP
server connects to workflow services as regular in-cluster pod-to-service
traffic. The `services/proxy` rule has been removed from `mcp_deploy.go`.

## CronJob Manifests from CLI

CronJob manifests are no longer generated by the CLI during `tntc deploy`.
Existing CronJobs in clusters from previous deployments are redundant but
harmless -- they will continue to fire until:
- The workflow is redeployed (new manifests do not include the CronJob, but
  `wf_apply` garbage-collects resources no longer in the manifest set).
- The workflow is undeployed (`wf_remove` removes all resources by label).
- The CronJob is manually deleted.

There is no automated migration of existing CronJobs. The annotation-based
approach is the new standard going forward.
