## 1. Directory Structure

- [x] 1.1 Create `tentacular-skill/` directory and `tentacular-skill/references/` subdirectory

## 2. SKILL.md Entry Point

- [x] 2.1 Create `tentacular-skill/SKILL.md` with project overview (Go CLI + Deno engine architecture), CLI command quick-reference table (init, validate, dev, test, build, deploy, status, cluster check, visualize with flags and usage), node function contract summary, minimal workflow.yaml skeleton, common development workflow (init -> dev -> test -> build -> deploy), and links to all reference documents

## 3. Reference Documents

- [x] 3.1 Create `tentacular-skill/references/workflow-spec.md` documenting the complete v2 workflow.yaml format: all top-level fields with types and validation rules, trigger types (manual/cron/webhook), node spec format (path, capabilities), edge format with validation (reference integrity, no self-loops, DAG acyclicity), config options (timeout, retries), and a complete annotated multi-node example
- [x] 3.2 Create `tentacular-skill/references/node-development.md` documenting TypeScript node patterns: default export async function signature, Context.fetch with URL construction and auth injection (Bearer token, API key), Context.log with node-prefixed methods (info/warn/error/debug), Context.config and Context.secrets access, data passing between nodes (single vs multiple dependencies), error handling, and a complete annotated node example
- [x] 3.3 Create `tentacular-skill/references/testing-guide.md` documenting the testing workflow: JSON fixture format (input/expected at tests/fixtures/<node>.json), node-level testing with tntc test, pipeline testing with --pipeline flag, createMockContext() usage (mock fetch, log capture, _setFetchResponse), mockFetchResponse helper, test result output format, and a complete fixture example
- [x] 3.4 Create `tentacular-skill/references/deployment-guide.md` documenting build and deploy: tntc build (Dockerfile generation, distroless base, --tag flag), tntc deploy (K8s manifest generation, Deployment with gVisor RuntimeClass, Service, --namespace/--registry flags), cluster check with --fix, secrets management (local .secrets.yaml, K8s Secret volume at /app/secrets, .secrets.yaml.example), and the Fortress security model (Deno permissions, distroless, gVisor)

## 4. Verification

- [x] 4.1 Verify all files exist: SKILL.md, references/workflow-spec.md, references/node-development.md, references/testing-guide.md, references/deployment-guide.md
- [x] 4.2 Verify SKILL.md contains CLI quick-reference, node contract, workflow skeleton, and links to all four reference documents
- [x] 4.3 Verify each reference document contains complete code/config examples and covers all scenarios defined in the spec
