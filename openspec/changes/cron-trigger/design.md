## Context

Cron triggers are spec'd and validated but the engine has no scheduler. Rather than building an in-engine cron scheduler, we generate K8s CronJob manifests that curl the existing HTTP endpoint. This keeps the engine stateless and leverages K8s native scheduling.

## Goals / Non-Goals

**Goals:**
- Generate CronJob manifests for each cron trigger during `tntc deploy`
- Support named triggers with POST body `{"trigger": "<name>"}` to differentiate behavior
- Clean up CronJobs on `tntc undeploy` via label selectors
- Pass POST body through to workflow executor as initial input for root nodes
- Add RBAC preflight checks for CronJob permissions

**Non-Goals:**
- In-engine cron scheduler (K8s handles scheduling)
- Webhook trigger implementation (Phase 3/4)
- CronJob monitoring dashboard

## Decisions

### CronJob curls existing HTTP endpoint
CronJobs use `curlimages/curl` to POST to the workflow's ClusterIP service. This avoids engine changes for scheduling and uses K8s native capabilities.

**Alternative**: In-engine cron library — adds complexity, requires persistent state, harder to debug.

### Named triggers with JSON payload
Each trigger can have a `name` field. CronJobs POST `{"trigger": "<name>"}` to `/run`. Root nodes receive this as input to branch behavior. Unnamed triggers POST `{}`.

### Label-based CronJob cleanup
CronJobs carry `app.kubernetes.io/name` and `app.kubernetes.io/managed-by: tentacular` labels. Undeploy lists by label selector and deletes all matches — no name guessing needed.

### Naming convention
Single cron trigger: `{wf}-cron`. Multiple: `{wf}-cron-0`, `{wf}-cron-1`, etc.

### Executor accepts initial input
The `execute()` method gains an optional `input` parameter. Root nodes (no dependencies) receive this instead of `{}`. This allows POST body to flow to the first node.

## Risks / Trade-offs

- **CronJob RBAC**: Clusters may not grant `batch/cronjobs` permissions. Preflight check catches this with remediation.
- **curl image availability**: `curlimages/curl` must be pullable in the cluster. Standard image, widely available.
- **concurrencyPolicy: Forbid**: Prevents overlapping runs. Users with long-running cron workflows may need to adjust schedules.
