## Context

The current Dockerfile ENTRYPOINT includes `--allow-net` (unrestricted network). K8s NetworkPolicy already restricts egress at the cluster level based on contract dependencies. Adding Deno-level `--allow-net=host:port` flags provides defense-in-depth: even if NetworkPolicy is misconfigured, the Deno runtime itself blocks unauthorized connections.

The existing `DeriveEgressRules` function in `pkg/spec/derive.go` already extracts host:port pairs from contract dependencies. The new `DeriveDenoFlags` function follows the same pattern.

## Goals / Non-Goals

**Goals:**
- Derive `--allow-net=host1:port1,host2:port2` from contract dependencies
- Inject derived flags as container `command`/`args` in the Deployment, overriding the Dockerfile ENTRYPOINT defaults
- Workflows without a contract keep permissive `--allow-net` (no breaking change)

**Non-Goals:**
- Restricting `--allow-read`, `--allow-write`, or `--allow-env` dynamically (these are already scoped in the ENTRYPOINT)
- Changing the Dockerfile itself (flags are injected at the K8s Deployment level)

## Decisions

1. **Override ENTRYPOINT via K8s `command`/`args`** -- Rather than generating per-workflow Dockerfiles, we use a single base image and inject the `deno run` command with specific flags via the Deployment container spec. This keeps the image generic and cacheable. K8s `command` maps to Docker ENTRYPOINT; `args` maps to CMD. We set `command` to the full `deno run ...` invocation with derived flags.

2. **`DeriveDenoFlags` returns a string slice** -- Returns the full argument list for the `deno run` command (e.g., `["deno", "run", "--allow-net=host:port", "--allow-read=/app,/var/run/secrets", ...]`). If contract is nil or has no dependencies with hosts, returns nil to indicate "use ENTRYPOINT defaults."

3. **Include localhost:8080 in allow-net** -- The engine serves HTTP on port 8080 and must listen on it. This is always included in the derived flags.

4. **Stable flag ordering** -- Sort hosts alphabetically for deterministic output, matching the pattern used in `DeriveEgressRules`.

5. **Pass contract through `DeployOptions`** -- Add a `Contract *spec.Contract` field to `DeployOptions` to pass the contract into `GenerateK8sManifests`. When present and non-nil, the function calls `DeriveDenoFlags` and injects `command`/`args`.

## Risks / Trade-offs

- [Risk] Workflows that make network calls not declared in the contract will fail at the Deno level. --> Mitigation: This is the intended behavior -- undeclared deps should fail. Contract enforcement is already strict.
- [Risk] Dynamic-target dependencies (CIDR-based) cannot be expressed as Deno `--allow-net` hosts. --> Mitigation: If any dependency is dynamic-target type, fall back to permissive `--allow-net` for the entire workflow.
