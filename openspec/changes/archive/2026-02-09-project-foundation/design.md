## Context

Pipedreamer v2 is a greenfield rewrite of the v1 distributed workflow system. The v1 architecture (Rust WASM nodes, SpinKube, kro, WIT contracts) proved over-engineered for the primary use case: AI agents rapidly building glue-code workflows. V2 pivots to a two-component architecture: a Go CLI for management and a Deno/TypeScript engine for execution.

This change establishes the project scaffold that all subsequent changes build on. No existing code or specs exist.

## Goals / Non-Goals

**Goals:**
- Establish Go CLI binary with cobra routing and all command stubs
- Establish Deno engine directory with TypeScript strict mode and import map
- Implement `pipedreamer init <name>` to scaffold new workflow directories
- Set up Go module and Deno configuration for CI-ready builds and tests
- Create clear directory structure separating CLI (`cmd/`, `pkg/`) from engine (`engine/`)

**Non-Goals:**
- Implementing any CLI commands beyond `init` (validate, dev, test, build, deploy come in later changes)
- Implementing the DAG engine, compiler, executor, or context (changes 03-04)
- Kubernetes integration (change 06)
- Testing framework (change 07)

## Decisions

### Decision 1: Go + Cobra for CLI
**Choice:** Use Go with `spf13/cobra` for the CLI binary.
**Rationale:** Same approach as v1, proven pattern for CLI tools. Go compiles to a single binary, cobra provides subcommand routing, flag parsing, and help generation. Alternative considered: using Deno for CLI too — rejected because Go has better K8s client-go integration and single-binary distribution.

### Decision 2: Deno with std library for engine
**Choice:** Use Deno with standard library imports via import map in `deno.json`.
**Rationale:** Deno provides V8 sandboxing with permission flags, built-in TypeScript support, and `Deno.serve` for the HTTP trigger server. The import map (`std/yaml`, `std/path`, `std/flags`, `std/assert`) pins dependencies to a specific std version. Alternative considered: Node.js — rejected because Deno's security model (permission flags) is core to the "Fortress" deployment pattern.

### Decision 3: Directory structure
**Choice:** `cmd/pipedreamer/` for Go entrypoint, `pkg/` for Go packages, `engine/` for Deno code.
**Rationale:** Standard Go project layout. Engine directory is self-contained — everything in `engine/` gets packaged into the container image. This clean separation means the Go CLI is never included in production containers.

### Decision 4: CLI command stubs
**Choice:** Register all planned commands as cobra stubs from the start, even if unimplemented.
**Rationale:** Ensures `pipedreamer --help` shows the full command surface from day one. Stubs return "not yet implemented" errors. Each subsequent change fills in its command.

## Risks / Trade-offs

- **Two-language toolchain** → Developers need both Go and Deno installed. Mitigated by clear separation: Go for CLI/K8s, Deno for engine/nodes. CI installs both.
- **Import map version pinning** → Deno std library pinned to specific version may need updates. Mitigated by using a single `deno.json` import map that can be bumped in one place.
- **Stub commands may confuse users** → Running `pipedreamer dev` before Change 05 is implemented returns an error. Acceptable for incremental development.
