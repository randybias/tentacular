## Context

Pipedreamer v2 is a two-component workflow system: a Go CLI (`cmd/pipedreamer/`, `pkg/`) for management and a Deno/TypeScript engine (`engine/`) for execution. The codebase is fully implemented across 8 prior changes covering project foundation, workflow spec parsing, DAG compiler/executor, node contract and Context API, dev command with hot-reload, cluster readiness, testing framework, and build/deploy pipeline.

AI agents currently must read raw source files (Go and TypeScript) to understand how to author workflows. There is no centralized, structured documentation optimized for agent consumption. This change creates a `pipedreamer-skill/` directory containing a SKILL.md entry point and a `references/` subdirectory with detailed guides, all derived from the existing source code.

## Goals / Non-Goals

**Goals:**
- Create a single SKILL.md entry point that an AI agent reads first to understand the full Pipedreamer v2 system
- Provide a quick-reference table of all CLI commands with flags, usage, and examples
- Document the complete workflow.yaml specification with field types, validation rules, and annotated examples
- Document the node development contract (function signature, Context API, data passing, error handling)
- Document the testing workflow (fixture format, mock context, node and pipeline tests)
- Document the deployment pipeline (build, deploy, cluster check, security model)
- Organize references so agents can drill into specific topics without reading irrelevant content

**Non-Goals:**
- Generating API documentation automatically from source code (manual curation is intentional)
- Creating user-facing documentation or marketing materials
- Documenting internal engine implementation details (compiler algorithms, executor internals) -- only the public-facing contracts and CLI surface
- Creating interactive tutorials or runbooks

## Decisions

### Decision 1: SKILL.md + references/ directory structure
**Choice:** A top-level `pipedreamer-skill/SKILL.md` that provides an overview and quick reference, with `pipedreamer-skill/references/` containing four detailed guides.
**Rationale:** Agents work best with a clear entry point that gives them the 80% view, then lets them drill into specifics. The SKILL.md covers what an agent needs for most tasks (command syntax, node contract boilerplate, workflow.yaml skeleton). The references/ subdirectory is for when the agent needs deeper understanding of a specific area. This matches the pattern used by other agent-skill repositories.
**Alternative considered:** A single monolithic SKILL.md -- rejected because it would be too large for efficient agent context window usage. Agents should only load the reference they need.

### Decision 2: Content derived from source code, not generated
**Choice:** Manually curate documentation content by reading the existing source files and extracting the essential information.
**Rationale:** The source code is the single source of truth. The skill documents distill this into agent-optimized format -- concise, example-heavy, and structured for lookup rather than narrative reading. Auto-generation would produce too much noise and miss the agent-specific framing.

### Decision 3: Inline code examples in every reference
**Choice:** Every reference document includes complete, runnable code examples (not just API signatures).
**Rationale:** AI agents generate better code when they see complete examples rather than abstract descriptions. A full node file example is more useful than a list of Context methods. Workflow.yaml examples should be complete and valid, not fragments.

### Decision 4: Deployment guide includes security model
**Choice:** The deployment guide documents the Fortress security model (Deno permissions, distroless container, gVisor sandbox) as part of the deployment documentation rather than as a separate reference.
**Rationale:** Security is inseparable from deployment in Pipedreamer. The Deno permission flags, distroless base image, and gVisor RuntimeClass are all configured during the build/deploy process. Agents need this context when generating Dockerfiles or K8s manifests.

## Architecture

```
pipedreamer-skill/
  SKILL.md                           # Entry point: overview, CLI quick-ref, node contract, workflow skeleton
  references/
    workflow-spec.md                  # Complete workflow.yaml specification
    node-development.md               # Node authoring: Context API, patterns, data passing
    testing-guide.md                  # Testing: fixtures, mock context, CLI commands
    deployment-guide.md               # Build, deploy, cluster check, security model
```

### SKILL.md Structure
1. Project overview (architecture: Go CLI + Deno engine)
2. CLI command quick-reference table (all 9 commands with flags)
3. Node function contract (signature, default export, async pattern)
4. Workflow.yaml skeleton (minimal valid example)
5. Common workflows (init -> dev -> test -> build -> deploy)
6. Links to references/ for deep dives

### Reference Document Structure (each)
1. Purpose and scope
2. Complete API/format specification
3. Full annotated examples
4. Common patterns and pitfalls
5. Cross-references to other documents

## Risks / Trade-offs

- **Documentation drift** -- As the codebase evolves, SKILL.md and references may become stale. Mitigated by keeping the skill documents focused on stable contracts (Context interface, workflow.yaml format, CLI surface) rather than implementation details that change frequently.
- **Incomplete coverage** -- The skill may not cover every edge case in the codebase. Mitigated by focusing on the primary agent workflow (init, write nodes, test, deploy) and referencing source files for advanced scenarios.
- **Context window usage** -- Loading SKILL.md + all 4 references at once may use significant context. Mitigated by the layered design: agents load SKILL.md first (compact), then only the specific reference they need.
