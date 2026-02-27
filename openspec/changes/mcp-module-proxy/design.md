## Context

The tentacular platform uses a persistent esm.sh proxy Deployment for module resolution. Currently `tntc cluster install --module-proxy` generates and applies the proxy manifests (Deployment, Service, NetworkPolicy) directly via `k8s.GenerateModuleProxyManifests()`. The proxy runs in `tentacular-system` namespace, caches npm/jsr modules, and is accessed by workflow pods via import maps. Config options include storage mode (emptydir/pvc), PVC size, and image version, stored in `~/.tentacular/config.yaml` under `moduleProxy`.

## Goals / Non-Goals

**Goals:**
- MCP server owns the persistent esm.sh proxy lifecycle via reconciliation.
- `proxy_status` tool: report proxy Deployment readiness, pod status, storage config, image version.
- `proxy_reconcile` tool: ensure proxy Deployment, Service, and NetworkPolicy match desired config. Create if missing, update if drifted, idempotent.
- Accept proxy config (image, storage, pvcSize, namespace) as tool parameters, with defaults from MCP server config.
- CLI routes `tntc cluster install --module-proxy` through MCP after bootstrap phase.

**Non-Goals:**
- Ephemeral per-deploy proxies (the proxy is persistent and shared by all workflows).
- Module caching logic changes (esm.sh handles caching internally).
- Moving import map generation to MCP (that remains a build-time CLI operation in `pkg/k8s/importmap.go`).

## Decisions

### Reconciliation pattern (not ephemeral)
The MCP server reconciles the persistent esm.sh proxy Deployment to a desired state. This means comparing the live Deployment against the desired spec (image, replicas, storage, NetworkPolicy) and applying patches for any drift. This is a controller-style reconciliation, not an ephemeral start/stop cycle.

Alternative: Ephemeral per-deploy proxy -- wrong model. The proxy is a shared cluster resource that caches modules across all workflow deploys.

### proxy_reconcile accepts config overrides
The tool accepts optional parameters for image, storage mode, PVC size, and namespace. If not provided, it uses defaults from the MCP server's own configuration (which mirrors the CLI's `moduleProxy` config block). This lets the CLI pass user-specified values from flags/config.

### Manifest generation reuse
The proxy manifest generation logic (`GenerateModuleProxyManifests`) from the CLI's `pkg/k8s/` should be extracted or duplicated in the MCP server. Since the MCP server is the authority for cluster state, it should own the manifest templates. The CLI's copy becomes dead code in Phase 6.

### Background reconciler (deferred)
A background goroutine that periodically checks proxy health and auto-reconciles is a natural extension but can be deferred to a follow-up. The `proxy_reconcile` tool provides on-demand reconciliation for v1.

## Risks / Trade-offs

- **Config sync**: The MCP server and CLI need compatible proxy configuration. The CLI passes its config to MCP via tool parameters, so MCP is the source of truth for actual cluster state.
- **Manifest duplication**: During transition, both CLI (`pkg/k8s/`) and MCP server generate proxy manifests. Phase 6 removes the CLI copy.
- **NetworkPolicy complexity**: The proxy's NetworkPolicy allows egress to jsr.io, registry.npmjs.org, cdn.deno.land on 443. Changes to the allowed registries require updating the reconciliation spec.
- **Storage migration**: Switching from emptydir to PVC (or vice versa) on an existing proxy requires careful handling. The reconciler should detect and warn, not auto-migrate.
