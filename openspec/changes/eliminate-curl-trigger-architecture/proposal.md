# Eliminate curl-based trigger architecture

## Why

The previous trigger architecture relied on ephemeral `curlimages/curl` pods to
POST to workflow `/run` endpoints and K8s CronJob resources to run those pods on
a schedule. This approach had several compounding problems:

**curlimages/curl dependency is brittle.**
The `curlimages/curl` image has no locked versioning in the CLI-generated
manifests. Any upstream change to the image breaks trigger pods. Additionally,
the `curlimages/curl` image runs as a numeric UID (`nobody`, 65534), which
satisfies `runAsNonRoot` but fails Pod Security Admission `restricted` profile
validation because PSA requires a numeric non-zero user in the image metadata
rather than just at runtime. Working around this requires an explicit
`securityContext.runAsUser` on the trigger pod, adding more complexity.

**ipset sync race with kube-router.**
kube-router v2.6.2 populates `podSelector` ipsets via pod IP. Ephemeral trigger
pods may attempt to connect before kube-router has added their IP to the ipset,
causing "Connection refused" on the first attempt. The workaround (`--retry 5
--retry-connrefused --retry-delay 1`) adds latency and masks a structural timing
problem.

**CronJob+Pod+curl is over-engineered for a simple HTTP call.**
Triggering a workflow amounts to a single authenticated HTTP POST. Running a
full CronJob lifecycle (schedule, Job creation, Pod scheduling, image pull,
container start, curl, cleanup) for this single call is disproportionately
complex. The MCP server is already in-cluster with full K8s API access; it can
make this call directly via the API service proxy.

**Proxy-prewarm init container adds unnecessary complexity.**
The `proxy-prewarm` init container (also using `curlimages/curl`) ran before the
main engine container to warm the esm.sh module cache. This was necessary because
a cold cache caused module resolution timeouts during engine startup. The init
container approach blocked pod readiness, slowed rollouts, and added another
`curlimages/curl` dependency.

## What Changed

- **Removed CronJob manifest generation** from `pkg/builder/k8s.go`. The CLI no
  longer generates CronJob resources for `type: cron` triggers. Instead, the cron
  schedule is recorded as an annotation on the Deployment.
- **Removed proxy-prewarm init container** generation from `pkg/builder/k8s.go`.
  The init container block for `curlimages/curl` pre-warming is gone.
- **Added `tentacular.dev/cron-schedule` annotation** to generated Deployments
  for workflows with cron triggers. The MCP server's internal cron scheduler reads
  this annotation and fires the workflow on schedule.
- **Added control-plane ingress rule** to generated NetworkPolicy. A new ingress
  rule allows TCP 8080 traffic from `10.0.0.0/8` (the API server / control plane
  CIDR). This is required for the K8s API service proxy pattern used by `wf_run`
  and module pre-warm.
- **Removed dead `RunWorkflow` function** from `pkg/k8s/client.go`. This function
  created ephemeral trigger pods and was the client-side implementation of the old
  approach. It is now entirely replaced by `wf_run` on the MCP server.
- **Removed `GenerateTriggerNetworkPolicy`** from `pkg/k8s/netpol.go`. The
  trigger egress NetworkPolicy (which allowed trigger pods to reach the engine) is
  no longer needed since no trigger pods are created.
- **Removed trigger NetworkPolicy call** from `pkg/cli/deploy.go`. The deploy
  flow no longer generates or applies a trigger NetworkPolicy.
- **Updated all tests** to reflect the new manifest structure.

## Impact

- Net change: -810 lines, +261 lines.
- No more `curlimages/curl` in CLI-generated manifests.
- MCP server now owns all triggering: `wf_run` via API service proxy, cron via
  internal scheduler, module pre-warm via background goroutine in `wf_apply`.
- Existing CronJobs in clusters are redundant but harmless -- they will continue
  to fire until manually deleted or the workflow is redeployed.
- The `tentacular.dev/cron-schedule` annotation is the new canonical source of
  truth for cron trigger configuration.
