## Why

Tentacular workflows are currently static in topology but not fully declarative as integration contracts. External dependencies, required secret keys, and network/security intent are inferred from node code, which weakens planning, review, and policy generation. We need workflow specs to be explicit contracts so agents can plan reliably and the platform can enforce security boundaries from declared intent.

## Key Insight

A Tentacular workflow is a sealed pod that can only reach declared network dependencies. This means **dependencies are the single primitive** — secrets, network policy, connection config, and drift validation are all derivable from the dependency list. The contract is not a collection of parallel sections; it is a typed dependency list from which everything else follows.

## What Changes

- Add a first-class `contract` section to `workflow.yaml` as a top-level peer of `nodes`, `edges`, and `config`. The contract is a typed dependency list.
- Each dependency declares protocol, host, port, connection metadata, and auth (secret key reference). Secrets are derived from dependency auth declarations — no separate secrets section.
- NetworkPolicy is derived from dependencies (egress allow per dep host/port) + triggers (webhook → ingress allow). Default-deny is implicit for a hardened pod. Optional overrides for edge cases only.
- Connection metadata is injected into node context via `ctx.dependency("name")` — nodes no longer manually assemble connections from config/secrets.
- Runtime-tracing drift detection during `tntc test`: mock context captures `ctx.dependency`, `ctx.fetch`, `ctx.secrets` calls and diffs against contract.
- Rich visualization includes dependency graph alongside DAG topology.
- Pre-build contract review loop with rich visualization artifacts required before build/deploy.
- All example workflows updated with contract blocks and node code migrated to `ctx.dependency()`.

## Capabilities

### New Capabilities
- `workflow-contract`: Declarative typed dependency list in `workflow.yaml` from which secrets, network policy, and connection config are derived.

### Modified Capabilities
- `workflow-spec`: Extend parser/validator to support `contract` schema, referential integrity, and enforcement modes.
- `k8s-deploy`: Synthesize NetworkPolicy from declared dependencies and trigger types.
- `cli-foundation`: Add contract-aware validation, runtime-tracing drift detection, and rich visualization outputs.
- `engine-context`: Add `ctx.dependency()` API for nodes to read connection metadata from contract declarations.

## Impact

- **Workflow schema**: New top-level `contract` block in `workflow.yaml`. Dependency connection info and secret references move from `config`/node code to `contract.dependencies`.
- **Node authoring**: Nodes use `ctx.dependency("name")` instead of manually assembling connections from config/secrets. Simpler, less error-prone.
- **Validation pipeline**: `tntc test` gains runtime-tracing drift detection. `tntc validate` checks contract structure.
- **Deployment pipeline**: `tntc deploy` auto-generates NetworkPolicy from dependencies + triggers.
- **Visualization**: `tntc visualize --rich` shows DAG + dependency graph + secret inventory + network intent.
- **Docs and skill guidance**: SKILL requires pre-build review with rich diagrams before build/deploy.
- **Breaking profile**: Not backward compatible. Workflows are disposable — validate the new version, discard the old.
