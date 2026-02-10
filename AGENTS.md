# Pipedreamer v2 — Agent Instructions

## Architecture

Before making changes, read **[docs/architecture.md](docs/architecture.md)** to understand the system architecture in detail. It covers the two-component design (Go CLI + Deno engine), security model, deployment pipeline, secrets cascade, testing strategy, data flow, and extension points.

## Project Structure

- **Go CLI** (`cmd/pipedreamer/`, `pkg/`) — lifecycle management, K8s operations
- **Deno Engine** (`engine/`) — DAG compilation and execution
- **Example Workflows** (`example-workflows/`) — runnable workflow examples
- **Infrastructure** (`deploy/`) — gVisor setup scripts and K8s resources

## Key Commands

```bash
# Build
go build -o pipedreamer ./cmd/pipedreamer/

# Go tests (run from project root)
go test ./pkg/...

# Deno tests (run from engine/ directory)
cd engine && ~/.deno/bin/deno test --allow-read --allow-write=/tmp --allow-net --allow-env

# Type check engine
cd engine && ~/.deno/bin/deno check main.ts
```

## Temporary Files

Use `scratch/` for all temporary files, experiments, and throwaway work. This directory is gitignored and will not be committed. Never place temp files in the project root or alongside source code.

## Documentation

Project documentation lives in `docs/`. The primary reference is **[docs/architecture.md](docs/architecture.md)** — always read it before making architectural changes. When adding new docs, place them in `docs/` and keep them focused and concise.

## Conventions

- Go tests use `strings.Contains` on generated YAML output (not YAML parsing)
- Go test files for unexported functions use same-package tests (e.g., `package cli` not `package cli_test`)
- Deno tests use `Deno.makeTempDir()` for filesystem tests with manual cleanup
- Node contract: `export default async function run(ctx: Context, input: T): Promise<U>`
- Nodes import types via `import type { Context } from "pipedreamer"` (mapped in `engine/deno.json`)
- Workflow names must be kebab-case; versions must be semver (e.g., `1.0`)
- Secrets are never environment variables — always volume mounts or files

## Change Tracking

This project uses OpenSpec for change tracking. Archived changes live in `openspec/changes/archive/`. When making significant changes, create an OpenSpec change with proposal, design, and tasks artifacts.

## Documentation Guidelines

Documentation is split between README.md and docs/ by purpose:

### README.md — The Signpost + Quickstart
- Project identity (description, features, architecture overview)
- Prerequisites and installation
- Quick Start walkthrough (scaffold → validate → dev → build → deploy)
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

### Why This Split
Top open source projects (React, Kubernetes, Rust, Go, Next.js) keep READMEs
under 100 lines and delegate reference material to docs sites or docs/ directories.
The README should make someone want to try the project. The docs/ should help
them succeed once they do.
