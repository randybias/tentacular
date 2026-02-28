# Design: In-Cluster Module Proxy via esm.sh

**Status:** Draft  
**Branch:** `feature/esm-module-proxy`

---

## Problem

Workflow nodes are deployed as TypeScript files via ConfigMap at runtime. When Deno loads them, it resolves `jsr:` and `npm:` specifiers by contacting `jsr.io` and `registry.npmjs.org`. These external fetches are blocked by Tentacular's default-deny NetworkPolicy, causing workflow node imports to fail unless bootstrap egress is explicitly permitted.

---

## Solution

Deploy a single in-cluster **esm.sh** instance as a cluster-level service during `tntc cluster install`. All workflow pods resolve module imports through this internal proxy. External egress to `jsr.io`/`registry.npmjs.org` is confined to the proxy — workflow pods need no external egress for dependencies at all.

---

## Architecture

```
workflow pod
  └─ Deno import("jsr:@db/postgres")
       └─► import_map.json (ConfigMap-mounted)
             └─► http://esm-sh.tentacular-support.svc.cluster.local
                   └─► jsr.io / registry.npmjs.org  (first fetch only)
                         └─► cached in emptyDir / PVC
```

---

## Components

### 1. esm.sh Deployment (`tntc cluster install`)

A cluster-level Deployment in the `tentacular-support` namespace:

```
Deployment: esm-sh
Namespace:  tentacular-support
Image:      ghcr.io/esm-dev/esm.sh:latest (pinned version)
Port:       8080
Storage:    emptyDir (default) or PVC (opt-in via config)
NetworkPolicy:
  ingress:  from any pod in any namespace on port 8080
  egress:   jsr.io:443, registry.npmjs.org:443, cdn.deno.land:443, raw.githubusercontent.com:443
```

Config flags (passed via env or config file):
- `ESM_ORIGIN`: set to the internal service URL so self-referential redirects resolve correctly
- `NPMRC`: optional, for private npm registries

### 2. Import Map ConfigMap (per workflow, at `tntc deploy`)

`tntc deploy` generates an `import_map.json` that rewrites all `jsr:` and `npm:` specifiers to
the internal esm.sh URL, then stores it as a ConfigMap:

```json
{
  "imports": {
    "jsr:@db/postgres": "http://esm-sh.tentacular-support.svc.cluster.local/jsr/@db/postgres@^0.4",
    "npm:zod": "http://esm-sh.tentacular-support.svc.cluster.local/zod@^3"
  }
}
```

Import maps are **always generated**, not conditional on a contract or jsr/npm deps. The engine
itself has `jsr:` dependencies (e.g., `@nats-io/transport-deno`) that must route through the
module proxy, so every workflow pod receives an import map. Contract-declared dependencies are
merged in on top of the engine's baseline entries. Version is taken from the contract; if
omitted, `*` (latest) is used.

`deno.land/std` URLs are rewritten through the esm.sh proxy using the GitHub proxy feature:
`/gh/denoland/deno_std@<version>/<path>`. This means no direct `deno.land` egress is needed
from workflow pods; the module proxy fetches from `raw.githubusercontent.com` on their behalf.

ConfigMap name: `<workflow-name>-import-map`
Mounted at: `/app/engine/deno.json`

### 3. Deno Engine Flags

The engine `ENTRYPOINT` uses:

```
deno run \
  --allow-net \
  --allow-env=DENO_DIR,HOME,TELEMETRY_SINK \
  --allow-import=deno.land:443,<proxy-host> \
  engine/main.ts
```

- No `--import-map` flag is needed. Deno auto-discovers `deno.json` from `/app/engine/`.
- `--allow-env` includes `TELEMETRY_SINK` for the telemetry sink endpoint.
- `--allow-import` includes `deno.land:443` because it is still needed for transitive
  cross-references within `deno_std`, even though the actual module bytes route through
  the proxy. `<proxy-host>` is the cluster-local esm.sh service hostname.

### 4. Workflow Pod NetworkPolicy

All workflow pods get this egress rule automatically:

```yaml
- to:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: tentacular-support
    podSelector:
      matchLabels:
        app.kubernetes.io/name: esm-sh
  ports:
  - protocol: TCP
    port: 8080
```

No external `jsr.io` egress needed on workflow pods.

---

## `tntc cluster install` Changes

Adds esm.sh to the cluster component manifest set:

```
tntc cluster install
  ├─ (existing) engine image pull / RBAC / etc.
  └─ (new) esm-sh Deployment + Service + NetworkPolicy
```

New config option in `~/.tentacular/config.yaml`:

```yaml
moduleProxy:
  enabled: true          # default true when cluster install runs
  storage: emptydir      # or "pvc" for persistence across restarts
  pvcSize: 5Gi           # only used when storage: pvc
  image: ghcr.io/esm-dev/esm.sh:v1.x.x
```

---

## NetworkPolicy Behaviour

Once the module proxy is installed, the generated NetworkPolicy for every workflow pod contains
**no egress to `jsr.io`, `registry.npmjs.org`, or `deno.land`**. The only dep-related egress
is to `esm-sh.tentacular-support:8080`.

External package fetches are isolated to the module proxy pod, which has its own
NetworkPolicy allowing outbound 443 to the public internet.

---

## Storage Options

| Mode       | PVC | Behaviour on restart              | Recommended for   |
|------------|-----|-----------------------------------|-------------------|
| `emptydir` | No  | Cold cache — re-fetches from jsr.io | Dev / staging   |
| `pvc`      | Yes (one, cluster-wide) | Warm cache survives restarts | Production |

---

## Open Questions

1. **Version pinning:** Should the import map use exact versions from a lockfile equivalent,
   or semver ranges from the contract? Exact versions are safer; ranges risk drift.
2. **Private registries:** esm.sh supports npm auth via `.npmrc`. Design for passing private
   registry tokens is TBD.
3. **esm.sh upgrade path:** Pinning the image version in config is required; auto-updates are
   risky since module resolution behaviour can change.
4. **Fallback behaviour:** If the esm.sh pod is unavailable, Deno fails to resolve imports.
   Should the engine retry or surface a clear error? Currently it would fail with a network error.

---

## Implementation Plan

1. `tntc cluster install` — add esm.sh Deployment + Service + NetworkPolicy
2. `pkg/spec` — add `jsr` and `npm` protocol types to `Dependency`
3. `pkg/k8s` — add `GenerateImportMap(wf, proxyURL)` → ConfigMap manifest
4. `pkg/k8s` — update `GenerateNetworkPolicy` to add esm.sh egress rule when module proxy is enabled
5. `pkg/cli/deploy.go` — generate and apply the import map ConfigMap at deploy time
6. Engine Dockerfile — add `--allow-env`, `--allow-import` flags to ENTRYPOINT
7. `tntc contract status` — surface module proxy status
