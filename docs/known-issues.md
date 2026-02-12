# Known Issues

Detailed investigation notes for open bugs and gaps discovered during workflow development (Feb 2026). The [roadmap](roadmap.md) references these items by number for the fixes.

---

## CLI / Builder Bugs

### JSR Import Map Resolution (Whack-a-Mole)

**Roadmap:** Tier 4, item #23
**File:** `engine/deno.json`

Third-party Deno libraries on `deno.land/x` internally use JSR-style bare specifiers (e.g., `@std/fmt/colors`). The engine's `deno.json` import map needs explicit mappings for each. Current approach: add mappings as failures surface.

Better long-term fix: migrate engine to JSR imports entirely.

### ServiceAccount Not Wired to Deployment

**Roadmap:** Tier 3, item #12
**File:** `pkg/builder/k8s.go`

Even if a ServiceAccount exists in the namespace, the generated Deployment has no `serviceAccountName`, defaulting to the `default` SA with no RBAC privileges.

**Current workaround:**

```bash
kubectl patch deployment <name> -n <namespace> \
  --type=json -p='[{"op":"add","path":"/spec/template/spec/serviceAccountName","value":"<sa-name>"}]'
```

### ConfigMap Not Cleaned Up by Undeploy

`tntc undeploy` deletes Service, Deployment, Secret, and CronJobs, but does NOT delete the ConfigMap (`<name>-code`). Manual cleanup required: `kubectl delete configmap <name>-code -n <namespace>`.

---

## Testing Gaps

### No Way to Mock In-Cluster APIs Locally

Nodes that read the K8s service account token from `/var/run/secrets/kubernetes.io/serviceaccount/token` can't be tested locally. `createMockContext()` provides mock `fetch` but can't mock `Deno.readTextFile()` calls outside the context API.

**Possible approaches:**
1. Environment variable override: if `KUBERNETES_SERVICE_HOST` is set, use SA token; otherwise look for kubeconfig in `ctx.config`/`ctx.secrets`
2. Filesystem mock in test context: extend `createMockContext()` with `_setFileContent(path, content)`
3. K8s helper in Context API: `ctx.k8s.get(path)` handling auth, TLS, and local/cluster detection

### Pipeline Tests Use Mock Fetch

`tntc test --pipeline` runs the full DAG with `createMockContext()`, so it never validates real HTTP behavior, Slack delivery, or database writes. Only validates that nodes can pass data through edges without errors.

**Consideration:** An integration test mode (`--live` flag or `tntc integration-test`) could run with real context.

---

## Documentation Gaps

### No Documentation on In-Cluster Patterns

No docs or examples for:
- Accessing the K8s API from within a workflow (SA token, CA cert, `Deno.createHttpClient`)
- Using Postgres or other databases from workflow nodes
- Setting up RBAC for cluster-access workflows
- Sharing resources between related workflows in the same namespace

The `cluster-health-*` workflows serve as reference implementations but patterns should be documented.

### Skill SKILL.md Missing In-Cluster and Database Patterns

**File:** `tentacular-skill/SKILL.md`

Covers HTTP-based workflows well but missing: K8s API access, database patterns, custom TLS, RBAC/ServiceAccount config, and multi-workflow architectures.

---

## Resolved (Feb 2026)

The following issues have been fixed. See [roadmap.md](roadmap.md) Archive section for details.

- **Nested Secrets YAML Support** -- `buildSecretFromYAML()` now uses `map[string]interface{}` and JSON-serializes nested maps
- **ImagePullPolicy** -- Generated Deployments include `imagePullPolicy: Always`
- **Dockerfile `--no-lock`** -- Both `deno cache` and `deno run` include `--no-lock`
- **Fixture Config/Secrets** -- Test fixtures support optional `config` and `secrets` fields
- **Deployment Guide Dockerfile** -- Updated to match actual generated Dockerfile
- **Preflight Secret Check First-Deploy Failure** -- `tntc deploy` no longer checks for K8s secret existence when local secrets will be auto-provisioned during the same deploy
- **Version Label YAML Float Parsing** -- `app.kubernetes.io/version` label is now YAML-quoted (`"1.0"`) to prevent float interpretation
