# Tentacular Roadmap

Last updated: 2026-03-10

## Active

### P1 — Next Up

Items planned for the near term.

| Item | Description | Dependencies | Target |
|------|-------------|--------------|--------|
| Secrets sync/diff (Phase C) | `tntc secrets sync <workflow>` pushes local secrets to K8s without full redeploy. `tntc secrets diff <workflow>` shows local vs. in-cluster differences. | None | TBD |
| UX-B: First-run workspace setup | `tntc configure --init-workspace` creates workspace directory, writes sane config, confirms K8s connection. Default workspace path becomes first-class config key. | None | TBD |
| Remove deprecated Kubeconfig/Context fields | Remove `Kubeconfig` and `Context` from `EnvironmentConfig` struct and associated dead code paths. | CLI/MCP separation (done) | TBD |
| Replace Ping healthz with MCP tool call | CLI connectivity checks should use an MCP protocol-level tool call instead of `/healthz` endpoint. | CLI/MCP separation (done) | TBD |
| External secrets backend | `secrets_source` config option for Vault, AWS Secrets Manager, etc. in production environments. | None | TBD |

### P2 — Planned

Items planned but not yet scheduled.

| Item | Description | Dependencies |
|------|-------------|--------------|
| Immutable versioned ConfigMaps | Name ConfigMaps as `{name}-code-{version}`, set `immutable: true`. Old ConfigMaps retained for rollback. | None |
| Rollback command | `tntc rollback <name> --version <version>` finds versioned ConfigMap, patches Deployment, runs rollout restart. | Immutable versioned ConfigMaps |
| Version history command | `tntc versions <name>` lists all ConfigMaps matching `{name}-code-*`. Optional `--diff v1 v2`. | Immutable versioned ConfigMaps |
| RBAC scaffolding | `deployment.serviceAccount` and `deployment.rbac` in workflow.yaml. `tntc deploy` creates SA/ClusterRole/Binding. | None |
| OpenTelemetry integration | Instrument Deno engine with OTel SDK: per-workflow trace, per-node spans, error recording, metrics export. | None |
| Token usage reporting | `ctx.usage()` API for nodes to record LLM token consumption. Surface in `tntc status` / `tntc logs`. | OTel integration (optional) |
| Environment configuration file | `environments.yaml` defining deployment environments with cluster context, namespace, registry, etc. `tntc deploy --env staging`. | None |
| ConfigMap-mounted runtime config overrides | Mount K8s ConfigMap at `/app/config` to override workflow config at runtime without rebuilding. | None |
| NATS JetStream durable subscriptions | Upgrade from core NATS to JetStream for queue triggers: durable subscriptions, ack with redelivery, replay. | None |
| Rate limiting / concurrency control | Max concurrent executions, token bucket rate limiting, backpressure for NATS subscriptions. | JetStream durable subscriptions |
| Dead letter queue | Failed NATS-triggered executions publish to `{subject}.dlq`. Retry, alerting, forensic analysis. | JetStream durable subscriptions |
| Multi-cluster deployment | Deploy workflows across multiple K8s clusters from kubeconfig contexts. | None |
| Canary deploys / traffic splitting | Run multiple workflow versions simultaneously with percentage-based traffic routing. | None |
| Webhook triggers via NATS bridge | Centralized webhook gateway publishing events to NATS subjects. Single ingress, per-registration routing. | JetStream durable subscriptions |
| Message payload passthrough | Binary payloads, content-type negotiation, schema validation for NATS trigger messages. | None |
| Multi-workflow namespace coordination | `tntc deploy-group` or project-level manifest for coordinating related workflows. | None |
| JSR import migration | Migrate engine from `deno.land/std` URL imports to JSR imports (`jsr:@std/*`). | None |

## Speculative Proposals

Items not yet on the committed roadmap. Design sketches and open questions for future discussion.

### SP-1: Webhook Gateway — Per-Registration Routing via NATS

A dedicated webhook gateway replacing the simple per-workflow webhook trigger with a centralized, multi-tenant-safe ingress model. Secure UUID paths (`/wh/{uuid}`), CRD/ConfigMap registration, HMAC validation per-registration, single ingress forever. Supersedes the NATS bridge item above with a more complete design. Depends on JetStream and DLQ items. See prior roadmap version for full design sketch and open questions.

## Archive

Completed items, most recent first.

### 2026-03-10 — Exoskeleton Phase 1 (CLI)

| Item | Completed | Notes |
|------|-----------|-------|
| tntc login/logout/whoami | 2026-03-10 | OAuth 2.0 Device Authorization Grant via Keycloak, token storage and refresh |
| ExoStatus/ExoRegistration MCP client methods | 2026-03-10 | CLI methods for exo_status and exo_registration MCP tools |
| OIDC token management | 2026-03-10 | Device auth flow, auto-refresh, expiry detection |
| Undeploy confirmation with cleanup warning | 2026-03-10 | Warns about data loss when cleanup is enabled, `--force` to skip |
| tentacular-* dep validation skip + s3 protocol | 2026-03-10 | CLI skips local validation for exoskeleton deps, s3 protocol support |

### 2026-03 — CLI/MCP Separation

| Item | Completed | Notes |
|------|-----------|-------|
| CLI/MCP separation and per-environment config | 2026-03 | All cluster operations route through MCP server. `cluster install` removed. Per-environment `mcp_endpoint` and `mcp_token_path`. |

### 2026-02 — tntc init and Catalog

| Item | Completed | Notes |
|------|-----------|-------|
| tntc init + scaffold init | 2026-02 | Scaffold command and scaffold-based quickstart initialization |
| Workflow scaffold commands | 2026-02 | `tntc scaffold list`, `tntc scaffold init <scaffold>` |
| Release pipeline | 2026-02 | GoReleaser, install.sh, stable.txt, version package |

### 2026-02 — Core Fixes and Features

| Item | Completed | Notes |
|------|-----------|-------|
| Nested secrets YAML support | 2026-02 | `buildSecretFromYAML()` handles nested maps via JSON serialization |
| ImagePullPolicy in generated Deployments | 2026-02 | `imagePullPolicy: Always` in all generated manifests |
| Dockerfile `--no-lock` on `deno cache` | 2026-02 | Matches `deno run` ENTRYPOINT, no lockfile issues |
| Initial configuration (`tntc configure`) | 2026-02 | Registry, namespace, runtime class defaults with layered config |
| Per-workflow namespace in workflow.yaml | 2026-02 | `deployment.namespace` with CLI > workflow.yaml > config > default resolution |
| Version tracking in K8s metadata | 2026-02 | `app.kubernetes.io/version` label on all generated resources |
| Secrets management (Phase A + B) | 2026-02 | `tntc secrets check`, `tntc secrets init`, shared secrets pool, `$shared` references |
| Fixture config/secrets support | 2026-02 | Test fixtures with `config` and `secrets` fields |
| Pre-built base image with dynamic loading | 2026-02 | ConfigMap-mounted workflow code, ~5-10s deploys without Docker rebuilds |
| Preflight secret provisioning ordering | 2026-02 | Deploy no longer checks K8s secret existence when local secrets will be auto-provisioned |
| K8s Secret volume symlink bug | 2026-02 | `loadSecretsFromDir()` handles symlink-based volume mounts |
| Read-only filesystem lockfile error | 2026-02 | `--no-lock` in ENTRYPOINT prevents write failures on distroless |
| `--allow-read` path widened | 2026-02 | Includes `/var/run/secrets` for SA tokens and CA certs |
| `--unstable-net` flag added | 2026-02 | Enables `Deno.createHttpClient()` for custom TLS |
| Version label YAML quoting | 2026-02 | Prevents semver strings from being parsed as floats |
| Example workflows migration | 2026-02 | Consolidated to `tentacular-scaffolds` repo |
