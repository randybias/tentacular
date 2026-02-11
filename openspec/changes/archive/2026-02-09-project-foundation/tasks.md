## 1. Go CLI Setup

- [x] 1.1 Initialize Go module (`go.mod`) with cobra dependency
- [x] 1.2 Create `cmd/tntc/main.go` with cobra root command and global flags (namespace, registry, output, verbose)
- [x] 1.3 Register all command stubs: init, validate, dev, test, build, deploy, status, cluster check, visualize
- [x] 1.4 Implement `tntc init <name>` — scaffold workflow.yaml, nodes/hello.ts, .secrets.yaml.example, tests/fixtures/hello.json
- [x] 1.5 Add kebab-case name validation to init command
- [x] 1.6 Create `pkg/` directory structure: cli/, builder/, k8s/, spec/

## 2. Deno Engine Setup

- [x] 2.1 Create `engine/deno.json` with strict TS config, import map (std/yaml, std/path, std/flags, std/assert, tentacular), lint/fmt settings
- [x] 2.2 Create `engine/types.ts` with core type definitions (WorkflowSpec, Context, Logger, NodeFunction, CompiledDAG, ExecutionResult)
- [x] 2.3 Create `engine/mod.ts` exporting public API types (Context, Logger, NodeFunction, WorkflowSpec, ExecutionResult)
- [x] 2.4 Create `engine/main.ts` entrypoint accepting --workflow, --port, --watch flags

## 3. Project Structure

- [x] 3.1 Create directory structure: cmd/tntc/, pkg/cli/, pkg/builder/, pkg/k8s/, pkg/spec/, engine/compiler/, engine/executor/, engine/context/, engine/testing/, examples/, tests/
- [x] 3.2 Create `.gitignore` covering binaries, vendor, .deno, secrets, IDE files

## 4. Verification

- [x] 4.1 Verify `go build ./cmd/tntc/` compiles successfully
- [x] 4.2 Verify `tntc --help` shows all commands
- [x] 4.3 Verify `tntc init test-workflow` creates correct scaffold
- [x] 4.4 Verify `deno check engine/main.ts` passes with no type errors
- [x] 4.5 Verify `deno test` runs in engine directory (even if minimal tests)
