## Context

Three developer ergonomics features are needed to reduce repetitive flag usage and provide deployment metadata:

1. **Config system** -- Currently every `tntc build` and `tntc deploy` invocation requires explicit `--registry`, `--namespace`, and `--runtime-class` flags. There is no persistence mechanism. The global namespace flag is defined in `cmd/tntc/main.go:19` as a persistent flag with default `"default"`. The registry flag is at line 21. These cobra defaults are the only fallback.

2. **Per-workflow namespace** -- The `spec.Workflow` struct in `pkg/spec/types.go:4-12` has no deployment metadata. Namespace is resolved solely from the CLI flag. Workflows that always deploy to the same namespace (like `sep-tracker` to `pd-sep-tracker`) require the flag every time.

3. **Version tracking** -- The labels in `pkg/builder/k8s.go:82-83` and `119-120` include `app.kubernetes.io/name` and `app.kubernetes.io/managed-by` but not `app.kubernetes.io/version`. The `WorkflowInfo` struct in `pkg/k8s/client.go:193-201` has no `Version` field. `tntc list` output in `pkg/cli/list.go:49` shows NAME, NAMESPACE, STATUS, REPLICAS, AGE -- no version column.

All three are additive changes. T1-6 touches the same file as T0-2 (k8s.go) but in different areas (labels vs. container spec).

## Goals / Non-Goals

**Goals:**

- Implement `tntc configure` with flags-only interface (no interactive wizard)
- Two-tier config: user-level `~/.tentacular/config.yaml` and project-level `.tentacular/config.yaml` with project overriding user
- Integrate config defaults into `deploy` and `build` commands using `cmd.Flags().Changed()`
- Add `deployment.namespace` optional field to workflow.yaml schema
- Implement four-level namespace cascade: CLI flag > workflow.yaml > config file > "default"
- Add `app.kubernetes.io/version` label to all generated K8s resources
- Surface version in `tntc list` and `WorkflowInfo`

**Non-Goals:**

- Interactive configuration wizard (`tntc configure` is flags-only per plan decision)
- Config validation or type checking beyond YAML parse
- Version comparison or upgrade detection between deployed and local versions
- Config inheritance across workflows (each workflow reads its own config cascade independently)

## Decisions

### D1: Config file format and merge semantics

Config uses YAML with three fields: `registry`, `namespace`, `runtime_class`. Merge is field-level: non-empty fields in a higher-priority source override lower-priority. Zero values (empty strings) mean "not set, use next level."

Load order: user-level (`~/.tentacular/config.yaml`) then project-level (`.tentacular/config.yaml`). Project overrides user.

**Why YAML:** Consistent with `workflow.yaml` format. The `gopkg.in/yaml.v3` dependency already exists.

**Why not env vars:** Env vars don't distinguish between user-level and project-level. Config files support both and are version-controllable (project-level can be committed to git).

### D2: Cobra flag integration with Changed()

Use `cmd.Flags().Changed("flag-name")` to detect if a flag was explicitly set by the user. If Changed() returns false, apply config file default. If the config file also has no value, fall back to cobra's built-in default.

This approach avoids the problem of cobra defaults masking config file values. The cobra default "default" for namespace would otherwise always win over the config file.

**Implementation pattern in deploy.go:**
```
namespace from flag
if not Changed("namespace"):
    if wf.Deployment.Namespace != "":
        namespace = wf.Deployment.Namespace
    else if cfg.Namespace != "":
        namespace = cfg.Namespace
```

### D3: Namespace resolution cascade

Four levels, highest priority first:
1. CLI `-n` / `--namespace` flag (explicit user intent)
2. `workflow.yaml` `deployment.namespace` (workflow-level default)
3. Config file `namespace` (user/project-level default)
4. Cobra default `"default"` (hardcoded fallback)

The cascade is evaluated in `runDeploy()` after parsing the workflow spec. This is the only place namespace resolution needs the full cascade -- `build` only needs registry from config, not namespace.

### D4: Version label from spec.Workflow.Version

Use `wf.Version` (already parsed and validated as semver `X.Y` format by `parse.go:43`) as the value for `app.kubernetes.io/version`. This label gets added to both label string locations in k8s.go (lines 82-83 for ConfigMap, lines 119-120 for Deployment/Service/CronJob).

The `GenerateK8sManifests()` signature and `GenerateCodeConfigMap()` signature already receive the full `*spec.Workflow` pointer, so `wf.Version` is already available without API changes.

### D5: list.go version column insertion

Insert VERSION column between NAME and NAMESPACE in the header. Extract version from `dep.Labels["app.kubernetes.io/version"]` in `ListWorkflows()`. Add `Version string` field to `WorkflowInfo` struct.

For backwards compatibility with pre-version deployments (no label present), default to empty string when the label is absent.

## Risks / Trade-offs

**Config file discovery uses current working directory for project-level** -- `LoadConfig()` reads `.tentacular/config.yaml` relative to CWD. If the user runs `tntc deploy` from a different directory than the workflow root, the project-level config may not be found. This is acceptable because `tntc deploy` already takes a `[dir]` argument and resolves paths from there. The config system uses CWD intentionally to match standard tooling conventions (`.npmrc`, `.gitconfig`, etc.).

**No config file validation** -- Invalid YAML in config files will be silently ignored (the `yaml.Unmarshal` error is swallowed to allow partial/missing configs). This is intentional: a missing or malformed config file should not break CLI operation. The fallback behavior (cobra defaults) is always available.

**Version label on CronJob curl containers** -- The version label appears on CronJob metadata but the CronJob's curl container is a generic curl image, not the workflow engine version. This is semantically correct though -- the label reflects the workflow version that the CronJob triggers, not the container it runs.
