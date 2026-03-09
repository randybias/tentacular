# Tentacular — Agent Instructions

Tentacular is a security-first, agent-centric, DAG-based workflow builder and runner for Kubernetes. It consists of four repositories that work together as a platform.

## Related Repositories

| Repository | Purpose |
|------------|---------|
| [tentacular](https://github.com/randybias/tentacular) | Go CLI (`tntc`) + Deno workflow engine (this repo) |
| [tentacular-mcp](https://github.com/randybias/tentacular-mcp) | In-cluster MCP server (Go, Helm chart) |
| [tentacular-skill](https://github.com/randybias/tentacular-skill) | Agent skill definition (Markdown) |
| [tentacular-catalog](https://github.com/randybias/tentacular-catalog) | Workflow template catalog (TypeScript/Deno) |

## System Architecture

```
Developer / AI Agent
        |
    tntc CLI (Go)            <-- this repo
        |
   JSON-RPC 2.0 / HTTP
        |
    MCP Server (Go)          <-- tentacular-mcp (Helm-installed in-cluster)
        |
    Kubernetes API
        |
    Workflow Pods             <-- Deno engine from this repo's engine/
        (gVisor sandbox)         Templates from tentacular-catalog
```

The CLI has zero direct Kubernetes API access. All cluster operations route through the MCP server via authenticated HTTP.

## Architecture Documentation

Before making changes, read **[docs/architecture.md](docs/architecture.md)** to understand the system in detail. It covers the two-component design (Go CLI + Deno engine), security model, deployment pipeline, secrets cascade, testing strategy, data flow, and extension points.

## Project Structure

- **Go CLI** (`cmd/tntc/`, `pkg/`) — lifecycle management, K8s operations via MCP
- **Deno Engine** (`engine/`) — DAG compilation and execution
- **Template Catalog** — workflow templates available via `tntc catalog list` from [tentacular-catalog](https://github.com/randybias/tentacular-catalog)
- **Infrastructure** (`deploy/`) — gVisor setup scripts and K8s resources

## Key Commands

```bash
# Build
go build -o tntc ./cmd/tntc/

# Go tests (run from project root)
go test ./pkg/...

# Deno tests (run from engine/ directory)
cd engine && ~/.deno/bin/deno test --allow-read --allow-write=/tmp --allow-net --allow-env

# Type check engine
cd engine && ~/.deno/bin/deno check main.ts
```

## Go Module Path

All Go code uses `github.com/randybias` as the module path prefix.

## Conventions

- Go tests use `strings.Contains` on generated YAML output (not YAML parsing)
- Go test files for unexported functions use same-package tests (e.g., `package cli` not `package cli_test`)
- Deno tests use `Deno.makeTempDir()` for filesystem tests with manual cleanup
- Node contract: `export default async function run(ctx: Context, input: T): Promise<U>`
- Nodes import types via `import type { Context } from "tentacular"` (mapped in `engine/deno.json`)
- Workflow names must be kebab-case; versions must be semver (e.g., `1.0`)
- Secrets are never environment variables — always volume mounts or files
- Container images must always be built as multi-arch (linux/amd64,linux/arm64) using `docker buildx`. Never build single-platform images.

## Cross-Repo Changes

Changes in this repo often require updates in other repos:

- **New MCP tool:** handler in `tentacular-mcp/pkg/tools/` -> client method in `pkg/mcp/tools.go` -> CLI command in `pkg/cli/` -> skill docs in `tentacular-skill/`
- **Contract/spec changes:** parser in `pkg/spec/` -> engine types in `engine/types.ts` -> builder in `pkg/builder/` -> skill docs in `tentacular-skill/`
- **New workflow template:** template in `tentacular-catalog/templates/`
- **Security model changes:** may touch `pkg/builder/`, `tentacular-mcp/pkg/k8s/`, and `tentacular-skill/`

## Commit Messages

All repos use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```
feat: add workflow health endpoint
fix: handle nil pod status in wf_pods
test: add unit tests for discover handler
docs: update CLI reference for catalog commands
chore: bump Go to 1.25
```

GoReleaser uses these prefixes to auto-generate changelogs (grouping by `feat`/`fix`, excluding `docs`/`test`/`chore`).

## Versioning

All four repos use **lockstep versioning** — they are tagged with the same version number for every release, even if a repo has no changes. Tags use semantic versioning: `vMAJOR.MINOR.PATCH`.

## Temporary Files

Use `scratch/` for all temporary files, experiments, and throwaway work. This directory is gitignored. Never place temp files in the project root or alongside source code.

## Change Tracking

This project uses OpenSpec for change tracking. Archived changes live in `openspec/changes/archive/`. When making significant changes, create an OpenSpec change with proposal, design, and tasks artifacts.

## Documentation Guidelines

Documentation is split between README.md and docs/ by purpose:

### README.md — The Signpost + Quickstart
- Project identity (description, features, architecture overview)
- Prerequisites and installation
- Quick Start walkthrough (scaffold -> validate -> dev -> build -> deploy)
- Brief node contract teaser (just the code example)
- Examples table
- Links to docs/ for everything else

**README rules:**
- Keep under 200 lines
- No reference tables (CLI flags, API surfaces, config schemas)
- No operational procedures (secrets setup, gVisor install, testing commands)
- One code example per concept maximum
- Every section that needs detail should link to a docs/ file

### docs/ — Reference Material

| File | Content | When to update |
|------|---------|----------------|
| `architecture.md` | System design, data flow, extension points | Architectural changes |
| `cli.md` | CLI command reference, flags, examples | New/changed CLI commands |
| `workflow-spec.md` | workflow.yaml format and field reference | Spec format changes |
| `node-contract.md` | Context API, auth injection, fixture testing | Engine API changes |
| `secrets.md` | Local and production secrets management | Secrets workflow changes |
| `testing.md` | Go, Deno, and workflow test commands | Test infrastructure changes |
| `gvisor-setup.md` | gVisor installation and verification | Deploy infrastructure changes |
| `roadmap.md` | Project roadmap and future plans | Planning updates |

## License

Proprietary (Mirantis, Inc.)
