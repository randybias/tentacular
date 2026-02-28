# cluster-profile — Implementation Tasks

## WI-1: `pkg/k8s/profile.go`

- [x] Define `ClusterProfile`, `NodeInfo`, `RuntimeClass`, `CNIInfo`, `NetPolInfo`,
      `StorageClass`, `ExtensionSet`, `QuotaSummary`, `LimitRangeSummary` structs
- [x] Implement `(*Client).Profile(ctx, namespace string) (*ClusterProfile, error)`
  - [x] Collect K8s version via `Discovery().ServerVersion()`
  - [x] List nodes; detect distribution from node labels + kubeconfig context name
  - [x] List `NodeV1().RuntimeClasses()` + detect gVisor
  - [x] Detect CNI via kube-system pod labels (calico, cilium, kube-router, weave, flannel, kindnet)
  - [x] List `StorageV1().StorageClasses()` + `StorageV1().CSIDrivers()`
  - [x] Detect NetworkPolicy support + InUse (list cluster-wide, count > 0)
  - [x] Detect ingress controllers (pod label scan, all namespaces)
  - [x] List CRDs via dynamic client (`apiextensions.k8s.io/v1`) + map to ExtensionSet
  - [x] Detect metrics server (kube-system pod label)
  - [x] List `CoreV1().ResourceQuotas(namespace)` + `CoreV1().LimitRanges(namespace)`
  - [x] Read namespace PSA labels
  - [x] Derive `Guidance` strings from collected data
- [x] Implement `(*ClusterProfile).Markdown() string` — renders the markdown format
- [x] Implement `(*ClusterProfile).JSON() string` — renders indented JSON
- [x] Write tests in `pkg/k8s/profile_test.go` covering guidance derivation and markdown output
- [x] Add kube-router CNI detection (k8s-app label, NetworkPolicy + egress supported)
- [x] Add k0s distribution detection (node.k0sproject.io/role label)

## WI-2: `pkg/cli/profile.go`

- [x] Implement `NewProfileCmd() *cobra.Command` with flags: `--env`, `--all`,
      `--output`, `--save`, `--force`
- [x] `runProfile`: resolve environment via `ResolveEnvironment()`, build client,
      call `client.Profile()`, render output
- [x] `runProfileAll`: iterate all environments in `LoadConfig().Environments`, call
      `runProfileForEnv` for each, collect errors, exit non-zero only if all fail
- [x] `saveProfile`: write `.tentacular/envprofiles/<env>.md` and `<env>.json`
      (create dir if needed)
- [x] Freshness check: if profile file exists and `--force` not set and age < 1h, skip
      with message `Profile for 'prod' is fresh (generated X min ago). Use --force to rebuild.`

## WI-3: `pkg/cli/cluster.go`

- [x] Add `cluster.AddCommand(NewProfileCmd())` in `NewClusterCmd()`

## WI-4: `pkg/cli/configure.go`

- [x] After writing config in `runConfigure`, iterate `cfg.Environments`
- [x] For each environment: resolve, create client, call `Profile()` with 30s timeout
- [x] On success: save to `.tentacular/envprofiles/<env>.{md,json}` and print confirmation
- [x] On failure (unreachable): print warning, continue — do not fail `configure`

## WI-5: `docs/cli.md`

- [x] Add `tntc cluster profile` section with: description, all flags, examples,
      output format description, profile storage path, drift-detection guidance

## WI-6: `openspec/specs/cluster-profile/spec.md`

- [ ] Write formal requirement spec covering all scenarios (modeled after `cluster-check/spec.md`)

## WI-7: k8s4agents — `skills/tentacular/SKILL.md`

- [x] Skill header, overview of `tntc` commands
- [x] Initial setup workflow: `tntc configure` -> auto-profiles all environments
- [x] Pre-workflow-build step: load `.tentacular/envprofiles/<env>.md` as context
- [x] Drift detection: list of signals + `tntc cluster profile --env <name> --save`
- [x] Guidance interpretation: how to use each guidance string when designing tentacles
