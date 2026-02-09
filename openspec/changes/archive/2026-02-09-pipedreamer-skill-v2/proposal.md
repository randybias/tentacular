## Why

AI agents building Pipedreamer workflows need a structured skill document (SKILL.md) with detailed reference materials so they can scaffold, develop, test, and deploy workflows without searching through scattered source files. The current codebase has no centralized documentation optimized for agent consumption -- agents must read raw Go and TypeScript source to understand CLI commands, node contracts, the Context API, workflow.yaml format, testing patterns, and deployment procedures. A well-structured skill with reference subdirectory eliminates this friction and enables faster, more accurate workflow authoring.

## What Changes

- **SKILL.md** (`pipedreamer-skill/SKILL.md`): Top-level skill document providing a quick-reference for all CLI commands (`init`, `validate`, `dev`, `test`, `build`, `deploy`, `status`, `cluster check`, `visualize`), the node function contract, workflow.yaml structure, and common development workflows. Organized for rapid agent lookup.
- **Workflow specification reference** (`pipedreamer-skill/references/workflow-spec.md`): Detailed documentation of the v2 workflow.yaml format -- all fields, types, validation rules, trigger types (manual/cron/webhook), node spec, edge definitions, config options. Includes annotated examples.
- **Node development guide** (`pipedreamer-skill/references/node-development.md`): TypeScript patterns for writing nodes -- the `export default async function run(ctx: Context, input: T): Promise<U>` contract, Context API usage (fetch with auth injection, structured logging, config access, secrets), error handling, and multi-node data passing via edges.
- **Testing guide** (`pipedreamer-skill/references/testing-guide.md`): How to write and run tests -- fixture format (`tests/fixtures/<node>.json`), node-level testing with mock context, pipeline-level testing, CLI test command usage, mock fetch responses, and log capture assertions.
- **Deployment guide** (`pipedreamer-skill/references/deployment-guide.md`): Build and deploy workflow -- `pipedreamer build` (Dockerfile generation, container image), `pipedreamer deploy` (K8s manifests, gVisor runtime, secrets volume mounts), cluster readiness checks, and the Fortress security model (Deno permission flags, distroless container, gVisor sandbox).

## Capabilities

### New Capabilities
- `agent-skill`: SKILL.md and references/ directory providing comprehensive, agent-optimized documentation for Pipedreamer v2 workflow development, covering CLI commands, node authoring, workflow specification, testing, and deployment

### Modified Capabilities
_(none)_

## Impact

- **New files**: `pipedreamer-skill/SKILL.md`, `pipedreamer-skill/references/workflow-spec.md`, `pipedreamer-skill/references/node-development.md`, `pipedreamer-skill/references/testing-guide.md`, `pipedreamer-skill/references/deployment-guide.md`
- **Dependencies**: None -- documentation only, derived from existing source code
- **APIs**: No API changes -- this change documents existing APIs and contracts
- **Downstream**: Enables AI agents to autonomously create, test, and deploy Pipedreamer workflows without needing to read source code directly
