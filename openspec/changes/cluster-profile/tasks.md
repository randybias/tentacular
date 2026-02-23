# cluster-profile — Implementation Tasks

## WI-1: `pkg/k8s/profile.go`

- [ ] Define `ClusterProfile`, `NodeInfo`, `RuntimeClass`, `CNIInfo`, `NetPolInfo`,
      `StorageClass`, `ExtensionSet`, `QuotaSummary`, `LimitRangeSummary` structs
- [ ] Implement `(*Client).Profile(ctx, namespace string) (*ClusterProfile, error)`
  - [ ] Collect K8s version via `Discovery().ServerVersion()`
  - [ ] List nodes; detect distribution from node labels + kubeconfig context name
  - [ ] List `NodeV1().RuntimeClasses()` + detect gVisor
  - [ ] Detect CNI via kube-system pod labels
  - [ ] List `StorageV1().StorageClasses()` + `StorageV1().CSIDrivers()`
  - [ ] Detect NetworkPolicy support + InUse (list cluster-wide, count > 0)
  - [ ] Detect ingress controllers (pod label scan, all namespaces)
  - [ ] List CRDs via dynamic client (`apiextensions.k8s.io/v1`) + map to ExtensionSet
  - [ ] Detect metrics server (kube-system pod label)
  - [ ] List `CoreV1().ResourceQuotas(namespace)` + `CoreV1().LimitRanges(namespace)`
  - [ ] Read namespace PSA labels
  - [ ] Derive `Guidance` strings from collected data
- [ ] Implement `(*ClusterProfile).Markdown() string` — renders the markdown format
- [ ] Implement `(*ClusterProfile).JSON() string` — renders indented JSON
- [ ] Write tests in `pkg/k8s/profile_test.go` covering guidance derivation and markdown output

## WI-2: `pkg/cli/profile.go`

- [ ] Implement `NewProfileCmd() *cobra.Command` with flags: `--env`, `--all`,
      `--output`, `--save`, `--force`
- [ ] `runProfile`: resolve environment via `ResolveEnvironment()`, build client,
      call `client.Profile()`, render output
- [ ] `runProfileAll`: iterate all environments in `LoadConfig().Environments`, call
      `runProfileForEnv` for each, collect errors, exit non-zero only if all fail
- [ ] `saveProfile`: write `.tentacular/envprofiles/<env>.md` and `<env>.json`
      (create dir if needed)
- [ ] Freshness check: if profile file exists and `--force` not set and age < 1h, skip
      with message `Profile for 'prod' is fresh (generated X min ago). Use --force to rebuild.`

## WI-3: `pkg/cli/cluster.go`

- [ ] Add `cluster.AddCommand(NewProfileCmd())` in `NewClusterCmd()`

## WI-4: `pkg/cli/configure.go`

- [ ] After writing config in `runConfigure`, iterate `cfg.Environments`
- [ ] For each environment: resolve, create client, call `Profile()` with 30s timeout
- [ ] On success: save to `.tentacular/envprofiles/<env>.{md,json}` and print confirmation
- [ ] On failure (unreachable): print warning, continue — do not fail `configure`

## WI-5: `docs/cli.md`

- [ ] Add `tntc cluster profile` section with: description, all flags, examples,
      output format description, profile storage path, drift-detection guidance

## WI-6: `openspec/specs/cluster-profile/spec.md`

- [ ] Write formal requirement spec covering all scenarios (modeled after `cluster-check/spec.md`)

## WI-7: k8s4agents — `skills/tentacular/SKILL.md`

- [ ] Create `skills/tentacular/SKILL.md` with:
  - Skill header (name, version, description)
  - Overview of `tntc` commands relevant to agents
  - Initial setup workflow: `tntc configure` → auto-profiles all environments
  - Pre-workflow-build step: load `.tentacular/envprofiles/<env>.md` as context
  - Drift detection: list of signals + `tntc cluster profile --env <name> --save`
  - Guidance interpretation: how to use each guidance string when designing tentacles
