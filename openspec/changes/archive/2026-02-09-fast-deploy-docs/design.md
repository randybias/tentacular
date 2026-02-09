## Context

The Fast Deploy feature splits the monolithic Docker build into two layers:
1. **Base engine image** (Phase 1): Engine-only Dockerfile, `pipedreamer-engine:latest`, no workflow code
2. **ConfigMap code delivery** (Phase 2+3): Workflow code in ConfigMap, mounted at `/app/workflow/`, rollout restart

Two documentation files need updating:
- `docs/architecture.md` — Technical architecture reference (528 lines, covers system overview through extension points)
- `pipedreamer-skill/references/deployment-guide.md` — Practical deployment guide (428 lines, covers build through security model)

Both docs currently describe the old monolithic flow: build bakes everything into one image, deploy uses `--cluster-registry` for image tags.

## Goals / Non-Goals

**Goals:**
- Update all pipeline descriptions to reflect two-layer architecture (engine image + ConfigMap)
- Add ConfigMap to resource tables and manifest examples
- Update Dockerfile example to show engine-only output
- Replace `--cluster-registry` references with `--image`
- Add Fast Iteration section to deployment guide
- Update Full Lifecycle example to show new workflow

**Non-Goals:**
- Rewrite entire documentation structure
- Add new architecture sections unrelated to Fast Deploy
- Update test count tables (may have changed independently)
- Documentation for features not yet implemented (queue triggers, webhooks)

## Decisions

### 1. Preserve existing document structure

**Decision:** Update content within existing sections rather than reorganizing. Add the "Fast Iteration" section as a new subsection in the deployment guide.

**Rationale:** Minimal disruption to existing readers. The docs are well-structured — the changes fit naturally into existing sections.

### 2. Show both build-once and deploy-often workflows

**Decision:** Document two workflows: (a) initial setup: `build` → `deploy`, and (b) fast iteration: edit code → `deploy` (no build needed).

**Rationale:** This is the core value proposition of Fast Deploy. Making both workflows explicit prevents confusion.

### 3. Keep old Dockerfile example as reference

**Decision:** Replace the Dockerfile example with the new engine-only version. Do not keep the old version as a "legacy" reference.

**Rationale:** The old Dockerfile is no longer generated. Keeping it would be confusing. Git history preserves the old version if needed.

## Risks / Trade-offs

**[Risk] Documentation goes stale if code changes again** → Acceptable. Documentation is updated per-change via OpenSpec.

**[Risk] Deployment guide references may be used by AI skill** → The `pipedreamer-skill/references/` path suggests this doc feeds an AI skill. Updates must be accurate and complete since they may be used for automated guidance.
