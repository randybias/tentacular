## Context

Tentacular's project foundation (Change 01) established the Go CLI binary and Deno engine scaffold. The validate command exists as a stub. The `pkg/spec/` package directory exists but needs the workflow specification types, YAML parser, and validation engine. The `gopkg.in/yaml.v3` dependency is already in `go.mod`. This change fills in the spec layer that all downstream features (DAG engine, dev command, build/deploy) depend on.

## Goals / Non-Goals

**Goals:**
- Define Go struct types for the v2 workflow.yaml format (Workflow, Trigger, NodeSpec, Edge, WorkflowConfig)
- Implement a YAML parser using `gopkg.in/yaml.v3` that unmarshals workflow.yaml into typed Go structs
- Implement a validation engine that checks: required fields, kebab-case naming, semver version format, trigger types and required sub-fields, node name format, edge reference integrity, DAG acyclicity
- Return human-readable error slices (not Go errors) for multi-error reporting
- Wire `tntc validate [dir]` CLI command to read workflow.yaml and invoke the parser

**Non-Goals:**
- Implementing the DAG compiler or executor (Change 03)
- Schema evolution or migration from v1 format
- JSON Schema generation or external validation tooling
- Workflow template inheritance or includes

## Decisions

### Decision 1: Single `Parse()` function with multi-error return
**Choice:** `func Parse(data []byte) (*Workflow, []string)` — returns parsed workflow and a slice of validation error strings.
**Rationale:** Callers get all validation errors at once rather than fail-fast on the first error. The `[]string` return type is simpler than a custom error type for this stage. If the spec is invalid, the returned workflow pointer is nil. Alternative considered: returning `(*Workflow, []error)` — rejected because the errors are pure validation messages, not recoverable errors.

### Decision 2: Regex-based naming validation
**Choice:** Compile regex patterns at package init time: `kebabRe` for workflow names (`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`), `identRe` for node names (`^[a-z][a-z0-9_-]*$`), `semverRe` for versions (`^[0-9]+\.[0-9]+$`).
**Rationale:** Simple, fast, and the patterns are well-understood. Node names allow underscores for compatibility with common naming conventions. Semver is simplified to major.minor (no patch) for workflow specs. Alternative considered: using a validation library — rejected as over-engineering for three regex patterns.

### Decision 3: DFS-based cycle detection
**Choice:** Three-color DFS (white/gray/black) to detect cycles in the edge graph.
**Rationale:** Standard textbook algorithm, O(V+E) time. Reports the specific edge that closes the cycle. Alternative considered: Kahn's algorithm (topological sort) — rejected because DFS gives better error messages (identifies the cycle edge rather than just "cycle exists").

### Decision 4: Validation in the parser, not separate
**Choice:** Validation runs inline during `Parse()` rather than as a separate `Validate(*Workflow)` function.
**Rationale:** Prevents callers from using an unvalidated workflow struct. The parse-and-validate pattern is a single entry point. Cycle detection is extracted to a helper `checkCycles()` for readability. Alternative considered: separate Parse + Validate — rejected because it allows invalid intermediate states.

### Decision 5: CLI reads file, delegates to spec.Parse
**Choice:** `pkg/cli/validate.go` reads `workflow.yaml` from disk and passes bytes to `spec.Parse()`. The spec package has no file I/O.
**Rationale:** Clean separation of concerns. The spec package is pure parsing/validation with no side effects, making it easy to test. The CLI handles file I/O and user-facing output.

## Risks / Trade-offs

- **Simplified semver** (major.minor only) → May need to add patch version later. Low risk, easy to extend the regex.
- **No streaming parser** → Entire workflow.yaml loaded into memory. Acceptable because workflow files are small (< 100 KB).
- **Error messages are strings** → Cannot be programmatically categorized (e.g., "is this a naming error or a reference error?"). Acceptable for CLI output. Could add error codes later if needed for machine consumption.
- **Map iteration order** → Go map iteration is non-deterministic, so validation error order for nodes may vary between runs. Acceptable for human consumption.
