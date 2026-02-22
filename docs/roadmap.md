# Tentacular Roadmap

Prioritized around the new-developer critical path: **how fast can someone go from `git clone` to a running deployed workflow?**

---

## Repo Structure

Tentacular is intended to live across three repositories with distinct ownership and release cadences. This split is a prerequisite for the catalog and independent skill publishing described in the UX Refactor section.

### `randybias/tentacular` — Core CLI + Engine
The tool itself: Go CLI (`cmd/`, `pkg/`) and Deno engine (`engine/`). Versioned releases. `example-workflows/` moves out of this repo entirely once the catalog repo exists — it does not belong in the tool repo.

### `randybias/tentacular-skill` — Agent Skill
The OpenClaw skill that teaches agents how to drive the CLI. Already a self-contained directory (`tentacular-skill/`), making it pre-split. Published to ClawHub for agent discovery.

Skill versions track CLI versions — skill `v1.x` documents CLI `v1.x` features. The skill opens with a `requires: tntc >= vX.Y` declaration so agents can detect version mismatches before proceeding.

### `randybias/tentacular-catalog` — Community Workflows
The default source for `tntc catalog pull`. Seeded from `example-workflows/` in the current repo. Each workflow is a subdirectory; community contributions are PRs to this repo without touching the CLI.

`tntc catalog pull github.com/randybias/tentacular-catalog/pr-review@v1.0` is a git-based reference — a clone of a subdirectory at a tag. No OCI tooling required. OCI registry storage is a future option for enterprise access-control scenarios.

### Release Pipeline

A prerequisite for the repo split being useful. Without published binaries, users must clone and build manually — the catalog and skill repos have no install story.

**Components:**
- **`.goreleaser.yaml`** — builds `darwin-amd64`, `darwin-arm64`, `linux-amd64`, `linux-arm64` binaries; packages as `tntc_${OS}_${ARCH}.tar.gz`; publishes to GitHub Releases with `checksums.txt` (sha256)
- **`install.sh`** — curl-based installer at the repo root. Phase 1: downloads the matching binary from GitHub Releases and verifies the checksum. Phase 2 fallback: clones and builds from source (requires Go). Default install dir: `~/.local/bin` (no sudo). Override via `TNTC_INSTALL_DIR`. Force source build via `TNTC_BUILD_FROM_SOURCE=true`.
- **`pkg/version`** — version package with `Version`, `Commit`, `Date` variables injected by GoReleaser ldflags. Exposed via `tntc version`.
- **`make build-cli`** — local single-platform build with version injection (no GoReleaser required)
- **`make release`** — full GoReleaser release (requires `GITHUB_TOKEN` and a pushed version tag)
- **`make release-snapshot`** — dry-run build for local testing

**Install (one-liner):**
```bash
curl -fsSL https://raw.githubusercontent.com/randybias/tentacular/main/install.sh | bash
```

**Skill integration:** The skill prerequisite check runs `which tntc` first. If missing, it runs the install script automatically before proceeding with any CLI commands.

---

## Tier 2: First Hour (Secrets and Testing)

### Secrets Rotation and Sync (Phase C)

Phases A and B are resolved. Remaining:
- `tntc secrets sync <workflow>` -- pushes local secrets to K8s without full redeploy
- `tntc secrets diff <workflow>` -- shows what's local vs. what's in-cluster

---

## Tier 3: First Week (Production Confidence)

### 9. Immutable Versioned ConfigMaps

ConfigMap is always named `{name}-code`. Updates overwrite content, destroying the previous version. No rollback capability.

**Fix:** Name ConfigMaps as `{name}-code-{version}`, set `immutable: true` (K8s 1.21+), Deployment references the versioned name. Old ConfigMaps retained for rollback.

Trade-off: ConfigMaps accumulate (need cleanup policy or `--prune` flag). Version bumps in workflow.yaml become meaningful.

### 10. Rollback Command

`tntc rollback <name> --version <version>` — finds ConfigMap `{name}-code-{version}`, patches Deployment to reference it, runs rollout restart. Depends on immutable versioned ConfigMaps (#9).

### 11. Version History Command

`tntc versions <name>` — lists all ConfigMaps matching `{name}-code-*`, shows version, creation timestamp, size. Optional `--diff v1 v2` to show code changes between versions.

### 12. RBAC Scaffolding

Workflows needing K8s API access (e.g., cluster-health-collector) require manually creating ServiceAccount, ClusterRole, and ClusterRoleBinding. The generated Deployment doesn't set `serviceAccountName` even when the SA exists.

**Fix:** Add `deployment.serviceAccount` and optional `deployment.rbac` to workflow.yaml:

```yaml
deployment:
  serviceAccount: cluster-health-collector
  rbac:
    clusterRole:
      rules:
        - apiGroups: [""]
          resources: ["nodes", "pods", "namespaces"]
          verbs: ["get", "list"]
```

`tntc deploy` creates the SA/ClusterRole/Binding if specified and wires `serviceAccountName` into the Deployment.

### 13. Deployment Metadata Enrichment

Moved to the **Refactoring the UX** section where it anchors the broader principle that Kubernetes is the authoritative source of truth for deployed workflow state.

### 14. OpenTelemetry Integration

Instrument the Deno engine with OTel SDK: per-workflow trace, per-node spans, error recording, and optional metrics export. Configure via workflow.yaml `telemetry` section or environment variables. Enables integration with Grafana, Jaeger, Datadog, etc.

### 15. Token Usage Reporting

Workflows calling LLM APIs have no mechanism to report back token consumption. Add a `ctx.usage()` API (or similar) that nodes can call to record token counts, and surface totals in `tntc status` / `tntc logs` output. May emit through OTel metrics if available (#14).

---

## Tier 4: First Month (Scaling)

Items listed in approximate priority order within this tier.

### 16. Environment Configuration File

A configuration file (e.g., `environments.yaml` or `tentacular-envs.yaml`) that defines deployment environments and their associated Kubernetes configs — cluster context, namespace, registry, runtime class, secrets profile, etc. Enables `tntc deploy --env staging` instead of juggling CLI flags per environment. Resolution order: CLI flags > environment config > project config > user config.

### 17. ConfigMap-Mounted Runtime Config Overrides

Mount a K8s ConfigMap at `/app/config` to override workflow config values at runtime without rebuilding the container. The engine merges ConfigMap values on top of workflow.yaml config.

### 18. NATS JetStream Durable Subscriptions

Upgrade from core NATS (at-most-once) to JetStream (at-least-once delivery) for queue triggers: durable subscriptions, acknowledgment with timeout-based redelivery, and replay for debugging or reprocessing.

### 19. Rate Limiting / Concurrency Control for Queue Triggers

Max concurrent executions, token bucket / sliding window rate limiting, and backpressure to slow down NATS subscription when at capacity.

### 20. Dead Letter Queue for Failed Executions

Failed NATS-triggered executions publish the original message to `{subject}.dlq`. Enables retry from DLQ, alerting on DLQ depth, and forensic analysis.

### 21. Multi-Cluster Deployment

Deploy workflows across multiple K8s clusters with a single command. CLI discovers available clusters from kubeconfig contexts and generates manifests for each.

### 22. Canary Deploys / Traffic Splitting

Run multiple versions of a workflow simultaneously. CronJobs and NATS subscriptions route to the active version. Canary sends a percentage of traffic to the new version.

### 23. Webhook Triggers via NATS Bridge

A single gateway workflow subscribes to HTTP webhooks and publishes events to NATS subjects. Downstream workflows subscribe via queue triggers. Avoids per-workflow Ingress, centralizes webhook handling and TLS termination.

### 24. Message Payload Passthrough

Support binary payloads, content-type negotiation, and schema validation for incoming NATS trigger messages (currently JSON-only).

### 25. Multi-Workflow Namespace Coordination

`tntc deploy` treats each workflow independently, but related workflows (e.g., collector + reporter) often share a namespace and secrets. A `tntc deploy-group` command or project-level manifest could coordinate multiple related workflows. The per-workflow namespace feature (#5) solves the single-workflow case; this addresses the multi-workflow orchestration case.

### 26. JSR Import Migration

The engine uses `deno.land/std` URL-based imports, but third-party JSR libraries import `@std/*` bare specifiers that the engine's import map doesn't cover. Current fix is whack-a-mole (adding mappings as failures surface). Long-term fix: migrate engine to JSR imports (`jsr:@std/yaml`, `jsr:@std/path`, etc.) so JSR resolution works naturally.

---

## Refactoring the UX

A cross-cutting initiative to give Tentacular a coherent user-facing story independent of the skill documentation. Today, the skill compensates for gaps in the CLI and tooling — canonical workflow location is documented in prose, config layering isn't enforced, and the cluster has almost no queryable metadata. This section resolves the underlying problems.

**Core principle: Kubernetes is the source of truth for runtime state. Local disk is source code. No third state.**

### 13. Deployment Metadata Enrichment (K8s as the Registry)

A local deployment state store (SQLite, JSON file, etc.) creates a second source of truth that drifts from the cluster. If you have kubectl access, you have the state. If you don't, a local file doesn't help — you can't deploy either.

**Approach:**
- Annotate all generated K8s resources at deploy time with `tentacular.io/*` labels:
  - `tentacular.io/deployed-by` — username or agent identity
  - `tentacular.io/git-sha` — source commit at deploy time
  - `tentacular.io/deployed-at` — RFC3339 timestamp
  - `tentacular.io/source-path` — path on disk the workflow was deployed from
  - `tentacular.io/workflow-name`, `tentacular.io/workflow-version`
- `tntc list --all-namespaces` queries the cluster directly for all tentacular-managed resources via label selector (`app.kubernetes.io/managed-by=tentacular`)
- `tntc status <name>` surfaces these annotations — no local cache needed
- Local `.tentacular/last-deploy.json` (git-ignored) is a *convenience cache only* — "what did I last do from this machine" — never a registry

**Result:** `tntc list --all-namespaces` becomes the single global view of what's deployed, where, and from what source. Agents and humans get the same answer.

### UX-A. `tntc init` — Scaffold Anywhere

There is no `tntc init` command. New users copy from `example-workflows/` by hand. The skill documents this workaround. It should not exist.

**Fix:**
- `tntc init <name>` scaffolds a new workflow in the current directory (or `--dir <path>`)
- Generates `workflow.yaml` (minimal valid spec), `nodes/hello.ts`, `.secrets.yaml.example`, `tests/`
- Optionally: `tntc init <name> --from catalog://pr-review` pulls a catalog template and scaffolds from it
- Replaces the "copy from `example-workflows/`" instruction in the skill entirely

### UX-B. First-Run Workspace Setup

`tntc configure` currently sets registry, namespace, and runtime class. It does not establish where workflows live or verify anything about the local environment.

**Fix:**
- `tntc configure --init-workspace` creates `~/tentacles/` (or user-specified path), writes a sane `~/.tentacular/config.yaml` with real defaults, and confirms the K8s connection
- Default workspace path becomes a first-class config key: `workspace: ~/tentacles`
- `tntc init` without `--dir` scaffolds into the configured workspace
- Project config (`.tentacular/config.yaml`) should only hold project-level overrides — not duplicate the full user config. The current state (identical files) is a bug.

### UX-C. Workflow Catalog

Discovery and sharing for Tentacular workflows. Workflows are source artifacts (TypeScript + YAML) — they should be human-readable before running. A git-based reference model is the natural fit; no binary packaging or OCI tooling required.

**Commands:**
- `tntc catalog search <query>` — search the configured catalog index
- `tntc catalog pull <ref>` — fetch a workflow into the local workspace. Refs are git-based: `github.com/randybias/tentacular-catalog/pr-review@v1.0` clones the subdirectory at that tag
- `tntc catalog push <name>` — packages the workflow and opens a PR against the configured catalog repo (or pushes to a private catalog)
- `tntc catalog list` — list workflows available in the configured catalog

**Storage:** Git repos. The default catalog is `randybias/tentacular-catalog` (see Repo Structure). Private catalogs are any git repo with the same directory convention. OCI registry storage is a future option for environments that need access-control independent of git auth.

**Index:** A static JSON index file (`catalog.json`) at the root of the catalog repo, served via GitHub raw or Pages. Fields per entry: name, description, version, path, required secrets, tags. `tntc catalog search` queries this index.

**Relationship to the skill:** `example-workflows/` moves to `tentacular-catalog` as the seed. The skill stops documenting the "copy from example-workflows" workaround; `tntc catalog pull` replaces it.

---

## Speculative Proposals

Items not yet on the committed roadmap. These are design sketches and open questions for future discussion. No implementation commitment.

### SP-1: Webhook Gateway — Per-Registration Routing via NATS

A dedicated webhook gateway component that replaces the simple per-workflow webhook trigger with a centralized, multi-tenant-safe ingress model. Designed for production use where multiple workflows consume events from multiple upstream sources (GitHub, GitLab, etc.).

**Core design:**

- **Secure paths:** Each registration gets a UUID path (`/wh/{uuid}`) — no enumeration, no shared endpoints
- **No code push:** Registrations stored in a CRD or ConfigMap; `tntc webhook register --event pull_request --workflow pr-review` creates one
- **NATS mapping:** Registration record maps `{uuid}` + `event_type` → NATS subject (e.g., `webhooks.github.pr.opened`)
- **Auth:** HMAC validation per-registration (each consumer gets its own secret); optional IP allowlist for known GitHub/GitLab CIDR ranges
- **Single ingress:** One HTTPRoute, one TLS cert, one NetworkPolicy — forever

**Relationship to existing roadmap:**
- Supersedes #23 (Webhook Triggers via NATS Bridge) with a more complete design
- Depends on #18 (NATS JetStream Durable Subscriptions) for reliable delivery
- Integrates with #20 (Dead Letter Queue) for failed event handling

**Open questions:**

1. **Registration lifecycle:** What happens on CRD deletion? Drain in-flight NATS messages before teardown, or hard cut?
2. **Fan-out:** Can multiple workflows subscribe to the same `webhooks.github.pr.opened` subject? If yes, JetStream durable consumer per-workflow (not per-registration).
3. **Replay / DLQ:** Design jointly with #18 and #20 — worth speccing together rather than independently.
4. **`tntc webhook register` UX:** Should print the webhook URL + secret at registration time only (like GitHub deploy keys). Secret must never be retrievable after creation.

---

## Archive (Resolved)

Items verified as fixed in the current codebase. Kept for historical context.

### Nested Secrets YAML Support

**Resolved Feb 2026.** `buildSecretFromYAML()` now unmarshals into `map[string]interface{}` and JSON-serializes nested maps into K8s Secret `stringData` entries. Matches the engine's `loadSecretsFromDir` JSON parsing.

### ImagePullPolicy in Generated Deployments

**Resolved Feb 2026.** Generated Deployment manifests now include `imagePullPolicy: Always`, ensuring Kubernetes always pulls the latest image on redeploy.

### Dockerfile `--no-lock` on `deno cache`

**Resolved Feb 2026.** The `deno cache` command in the generated Dockerfile now includes `--no-lock`, matching the `deno run` ENTRYPOINT. Engine dependencies are cached at build time to the distroless default `/deno-dir/` and served from the read-only image layer at runtime — no `ENV DENO_DIR` override is needed. Workflow node dependencies (e.g., JSR imports like `@db/postgres`) are not pre-cached and require network access at runtime.

### Initial Configuration (`tntc configure`)

**Resolved Feb 2026.** `tntc configure` sets default registry, namespace, and runtime class. Supports user-level (`~/.tentacular/config.yaml`) and project-level (`.tentacular/config.yaml`) config files. All commands read these defaults; CLI flags override. Resolution: CLI flag > project config > user config.

### Per-Workflow Namespace in workflow.yaml

**Resolved Feb 2026.** Workflows can declare `deployment.namespace` in workflow.yaml. Namespace resolution: CLI `-n` flag > `workflow.yaml deployment.namespace` > config file default > `default`.

### Version Tracking in K8s Metadata

**Resolved Feb 2026.** All generated K8s resources (ConfigMap, Deployment, Service, CronJob) include `app.kubernetes.io/version` label from workflow.yaml `version` field. `tntc list` displays a VERSION column.

### Secrets Management Overhaul (Phase A + B)

**Resolved Feb 2026.** Phase A: `tntc secrets check` scans node source for `ctx.secrets` references and reports gaps against provisioned secrets. `tntc secrets init` creates `.secrets.yaml` from `.secrets.yaml.example`. Phase B: Shared secrets pool at repo root (`.secrets/`) with `$shared.<name>` references in workflow `.secrets.yaml`, resolved during `tntc deploy`. Phase C (sync/diff) remains on the roadmap.

### Fixture Config/Secrets Support

**Resolved Feb 2026.** Test fixture format extended with optional `config` and `secrets` fields. The test runner passes these to `createMockContext()`, enabling meaningful testing of nodes that read `ctx.config` or `ctx.secrets`.

### Pre-Built Base Image with Dynamic Workflow Loading

**Resolved Feb 2026.** `tntc build` generates an engine-only Docker image with no workflow code baked in. `tntc deploy` creates a ConfigMap with `workflow.yaml` and `nodes/*.ts`, mounts at `/app/workflow/`, and triggers a rollout restart. Code changes deploy in ~5-10 seconds without Docker rebuilds.

### Preflight Secret Provisioning Ordering

**Resolved Feb 2026.** Deploy command preflight checks no longer check for K8s secret existence when local secrets (`.secrets/` or `.secrets.yaml`) will be auto-provisioned during the same deploy. When no local secrets exist, the check is also skipped. Fixes first-deploy failures where the secret didn't exist yet.

### K8s Secret Volume Symlink Bug

**Resolved Feb 2026.** `loadSecretsFromDir()` now checks `!entry.isFile && !entry.isSymlink` instead of `!entry.isFile`, correctly handling Kubernetes symlink-based Secret volume mounts.

### Read-Only Filesystem Lockfile Error (Runtime)

**Resolved Feb 2026.** `deno run` in the generated Dockerfile ENTRYPOINT includes `--no-lock`, preventing lockfile write failures on the read-only distroless filesystem. (Note: `deno cache` at build time still needs `--no-lock` — tracked in Tier 0 item #3.)

### `--allow-read` Path Widened

**Resolved Feb 2026.** ENTRYPOINT includes `--allow-read=/app,/var/run/secrets`, enabling workflows to read K8s service account tokens and CA certificates.

### `--unstable-net` Flag Added

**Resolved Feb 2026.** ENTRYPOINT includes `--unstable-net`, enabling `Deno.createHttpClient()` for custom TLS (e.g., in-cluster K8s API calls with cluster CA).

### Version Label YAML Quoting

**Resolved Feb 2026.** The `app.kubernetes.io/version` label value is now quoted in generated YAML (e.g., `"1.0"`) to prevent YAML parsers from interpreting semver strings like `1.0` as floating-point numbers.

### AGENTS.md Example Workflows Convention

**Resolved Feb 2026.** All workflows consolidated into `example-workflows/`. AGENTS.md, README.md, and architecture.md updated. Multi-arch build convention (`docker buildx`) added.
