## Why

Pipedreamer v2 needs a workflow specification format and validation engine before the DAG engine, dev command, or build/deploy pipeline can be implemented. The v1 workflow.yaml format is abandoned; v2 introduces a clean-break format with typed YAML parsing, strict validation (required fields, naming conventions, DAG acyclicity, reference integrity), and a `pipedreamer validate` CLI command. This is the second change in the dependency chain, building on the project foundation.

## What Changes

- **v2 workflow.yaml format** with top-level fields: `name`, `version`, `description`, `triggers`, `nodes`, `edges`, `config`
- **Go YAML parser** (`pkg/spec/parse.go`) using `gopkg.in/yaml.v3` with full type definitions (`pkg/spec/types.go`)
- **Validation engine** covering: required fields (`name`, `version`, `triggers`, `nodes`), kebab-case naming, semver version format, trigger type validation (manual/cron/webhook), node name validation, edge reference integrity, DAG acyclicity via DFS cycle detection
- **Error reporting** returning a slice of human-readable validation error strings
- **`pipedreamer validate` CLI command** (`pkg/cli/validate.go`) reads `workflow.yaml` from a directory and runs the parser/validator, reporting errors to stderr or success to stdout

## Capabilities

### New Capabilities
- `workflow-spec`: YAML parsing with typed Go structs, validation engine (required fields, naming conventions, trigger validation, edge reference integrity, DAG acyclicity check), error reporting, and `pipedreamer validate` CLI command

### Modified Capabilities
_(none)_

## Impact

- **New/modified files**: `pkg/spec/types.go`, `pkg/spec/parse.go`, `pkg/spec/parse_test.go`, `pkg/cli/validate.go`
- **Dependencies**: `gopkg.in/yaml.v3` (already in go.mod)
- **CLI**: `pipedreamer validate` command transitions from stub to working implementation
- **Downstream**: DAG engine (change 03), dev command (change 05), and build/deploy (change 06) all depend on a valid parsed `Workflow` struct
