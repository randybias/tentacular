## Why

Pipedreamer v2 needs a project scaffold that establishes the two-component architecture: a Go CLI binary for management operations and a Deno/TypeScript engine for workflow execution. Without this foundation, no other changes (workflow spec, DAG engine, dev command, build/deploy) can proceed. This is the first change in the dependency chain.

## What Changes

- **New Go CLI binary** (`cmd/pipedreamer/main.go`) with cobra command routing and global flags (`--namespace`, `--registry`, `--output`, `--verbose`)
- **New Deno engine directory** (`engine/`) with TypeScript strict mode, `deno.json` import map, and `main.ts` entrypoint
- **`pipedreamer init <name>` command** — scaffolds a new workflow directory with `workflow.yaml` template, `nodes/` directory, `.secrets.yaml.example`, and test fixtures
- **Go module setup** (`go.mod`) with cobra dependency
- **CLI command stubs** for all planned commands: validate, dev, test, build, deploy, status, cluster check, visualize
- **Project directory structure** establishing clear separation between Go CLI (`cmd/`, `pkg/`) and Deno engine (`engine/`)

## Capabilities

### New Capabilities
- `cli-foundation`: Go CLI binary with cobra routing, global flags, and `init` command that scaffolds workflow directories
- `engine-foundation`: Deno engine directory structure with TypeScript config, import map, `main.ts` entrypoint, and `mod.ts` public API

### Modified Capabilities
_(none — greenfield project)_

## Impact

- **New files**: `cmd/pipedreamer/main.go`, `pkg/cli/*.go`, `engine/deno.json`, `engine/main.ts`, `engine/mod.ts`, `go.mod`
- **Dependencies**: Go (cobra), Deno (std library for yaml, path, flags, assert)
- **Build toolchain**: Requires both `go` and `deno` installed
- **All subsequent changes depend on this**: workflow spec, DAG engine, context, dev command, build/deploy, testing framework
