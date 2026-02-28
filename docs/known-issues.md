# Known Issues

---

## Module Proxy

### Deno `--allow-net` Scoping Broken for Module Proxy Deps

**Status:** Known issue — deferred fix  
**File:** `pkg/spec/derive.go` (`DeriveDenoFlags`, `moduleProxyHost`)

When a workflow has `jsr` or `npm` dependencies, the engine runs with a scoped
`--allow-net=esm-sh.tentacular-support.svc.cluster.local:8080,...` that includes the
module proxy host. This works for the current default `tentacular-support` namespace
but has two problems:

1. **Hardcoded namespace:** `moduleProxyHost` in `derive.go` is a constant. If the module
   proxy is installed in a non-default namespace, the `--allow-net` flag will still reference
   `tentacular-support` and block the connection.
2. **No config plumbing:** `DeriveDenoFlags` takes only a `*Contract` — it has no access to
   `ModuleProxyConfig`. Passing the proxy URL through requires a new function signature or
   a context struct.

**Fix (deferred):** Change `DeriveDenoFlags` to accept a `proxyURL string` parameter (or a
`DenoFlagOptions` struct). Update all callers. Pass `cfg.ModuleProxy` through the builder
pipeline so the correct proxy host is used in `--allow-net`.

---

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

**Status:** Resolved by MCP refactoring

`tntc undeploy` now routes through the MCP server's `wf_remove` tool, which uses label-based resource discovery to delete all resources associated with a deployment name, including ConfigMaps. Manual cleanup is no longer required.

---

## MCP Server

### Log Streaming Not Supported via MCP

**Status:** By design

`tntc logs --follow` is not supported when routing through the MCP server. The `wf_logs` MCP tool returns a snapshot of recent log lines. For real-time log streaming, use `kubectl logs -f` directly.

### cluster check --fix Removed

**Status:** By design

The `--fix` flag for `tntc cluster check` has been removed. Namespace creation and other remediation is now handled through dedicated MCP tools (`ns_create`). Run `tntc cluster install` to bootstrap the cluster environment.

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
- **Deployment Args Numeric YAML Quoting** -- Numeric values in Deployment `args:` (e.g., port `8080`) are now YAML-quoted (`"8080"`) to prevent K8s rejecting them as non-string types
- **kube-router NetworkPolicy ipset Sync Race** -- kube-router populates `podSelector` ipsets asynchronously. This was previously a problem for ephemeral trigger pods. With the elimination of curl-based trigger pods (replaced by K8s API service proxy in the MCP server), this race no longer affects `tntc run` or cron triggers. It may still affect other ephemeral pods (e.g., gVisor verification pods) on first connection.

---

### Fixed in Current Branch

#### Cron Schedule YAML Annotation Quoting
- Cron schedule values like `*/5 * * * *` must be quoted in YAML annotations because `*` is the YAML alias character
- Fixed: `pkg/builder/k8s.go` now wraps annotation value in double quotes

#### K8s Secret Values Must Be JSON-Structured
- The engine reads secrets from `/app/secrets/<service>` and parses each file as JSON
- Contract reference `secret: openai.api_key` means: K8s Secret key = `openai`, value = `{"api_key":"sk-..."}`
- Plain text values are silently ignored — `ctx.dependency().secret` returns null
- Fixed by documenting and validating in E2E tests

#### CNI Detection Missing kube-router
- Cluster profiling did not detect kube-router (k0s default CNI)
- Fixed: `pkg/k8s/profile.go` now detects `k8s-app: kube-router` pods with NetworkPolicy + egress support
- Also added k0s distribution detection via `node.k0sproject.io/role` node label

#### Module Proxy Import Maps Not Always Generated
- Import maps were only generated when the workflow had jsr/npm contract deps
- The engine itself has jsr: deps that need the proxy — import maps must always be generated
- Fixed: import map generation is now unconditional

#### deno.land/std URLs Not Routed Through Proxy
- Engine deno.land/std imports bypassed the module proxy
- Fixed: `rewriteDenoLandURL()` rewrites to `/gh/denoland/deno_std@version/path` proxy path
- `deno.land:443` still required in `--allow-import` for transitive cross-references within deno_std
