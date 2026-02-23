## Why

AI agents building tentacular workflows need to know what capabilities are available in
their target cluster before writing node contracts. Without a capability profile, an agent
may generate a workflow that assumes Istio mTLS, a specific CSI driver, or gVisor sandboxing
that doesn't exist in the target environment — causing silent runtime failures or incorrect
NetworkPolicy design.

`tntc cluster check` validates readiness (can I deploy?), but provides no capability data
(what can I build?). The profile fills that gap: a structured, agent-readable snapshot of
what the cluster supports.

Profiles are generated once on initial configuration and rebuilt when an agent detects
environment drift (cluster upgrade, new CRDs, changed storage classes, unexpected policy
behavior). They are stored as markdown in `.tentacular/envprofiles/<env>.md` and optionally
as JSON sidecars, committed alongside the tentacular config.

## What Changes

- **WI-1: `pkg/k8s/profile.go`** — `ClusterProfile` struct and `(*Client).Profile()` method.
  Queries: server version, distribution, nodes, RuntimeClasses, StorageClasses, CSI drivers,
  CNI (via kube-system pod labels), NetworkPolicy usage, ingress controllers, installed
  extensions (via CRD group discovery), resource quotas, limit ranges, pod security admission.
  Derives human-readable `Guidance` strings for the AI agent.

- **WI-2: `pkg/cli/profile.go`** — `tntc cluster profile` subcommand under `tntc cluster`.
  Flags: `--env <name>`, `--all` (all configured environments), `--output json|markdown`,
  `--save` (write to `.tentacular/envprofiles/<env>.md` + `.json`).
  Environment resolution via `ResolveEnvironment()` — consistent with `deploy` and `check`.

- **WI-3: `pkg/cli/cluster.go`** — wire `profile` subcommand into `NewClusterCmd()`.

- **WI-4: `pkg/cli/configure.go`** — after writing config, auto-run profile for each
  environment that has a reachable cluster. Skips gracefully if cluster is unreachable.

- **WI-5: `docs/cli.md`** — document the new command, flags, output formats, and
  drift-detection guidance.

- **WI-6: `openspec/specs/cluster-profile/spec.md`** — formal requirement spec.

- **WI-7: `k8s4agents` tentacular skill** — update/create
  `skills/tentacular/SKILL.md` to reference `tntc cluster profile` and document
  agent workflow for initial setup and drift-triggered re-profiling.

## Capabilities

### New Capabilities

- `cluster-profile`: Per-environment capability snapshot covering K8s version, distribution,
  node topology, RuntimeClasses, CNI, NetworkPolicy support, StorageClasses, CSI drivers,
  installed extensions (Istio, cert-manager, Prometheus Operator, etc.), namespace quotas,
  pod security admission, and derived agent guidance strings.

- `profile-on-configure`: Automatic profile generation when `tntc configure` writes
  environment config for a reachable cluster.

- `profile-drift-rebuild`: Agent-invocable `tntc cluster profile --env <name> --save`
  to rebuild a stale profile after drift is detected.

### Modified Capabilities

- `cluster-check`: `NewClusterCmd()` gains a `profile` subcommand sibling alongside `check`.
- `configure`: After writing config, attempts profile generation for all environments.

## Impact

- **New files**: `pkg/k8s/profile.go`, `pkg/cli/profile.go`,
  `openspec/specs/cluster-profile/spec.md`
- **Modified files**: `pkg/cli/cluster.go`, `pkg/cli/configure.go`, `docs/cli.md`
- **External repo**: `k8s4agents/skills/tentacular/SKILL.md` (new file)
- **Profile artifacts**: `.tentacular/envprofiles/<env>.md` + `<env>.json`
  (gitignored by default; teams may choose to commit them)
- **Dependencies**: No new Go dependencies — all discovery uses existing client-go APIs
  (`apiextensions.k8s.io/v1` CRD listing via the dynamic client already present on `Client`)
- **Breaking**: None. `tntc cluster check` behavior is unchanged. Profile generation on
  configure is best-effort and silently skips unreachable clusters.
