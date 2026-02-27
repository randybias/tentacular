## 1. Spec Types

- [x] 1.1 Add `WorkflowMetadata` struct to `pkg/spec/types.go` with Owner, Team, Tags, Environment fields
- [x] 1.2 Add `Metadata *WorkflowMetadata` field to `Workflow` struct with `yaml:"metadata,omitempty"`

## 2. Parsing and Validation

- [x] 2.1 Verify metadata deserializes correctly via existing YAML unmarshaling (no custom parse logic needed)
- [x] 2.2 Add parse tests in `pkg/spec/parse_test.go` for workflow with full metadata
- [x] 2.3 Add parse tests for workflow without metadata (nil Metadata, no errors)
- [x] 2.4 Add parse tests for workflow with partial metadata (only some fields set)

## 3. Verification

- [x] 3.1 Run `go test ./pkg/spec/...` -- all pass
- [x] 3.2 Run `go test ./pkg/builder/...` -- all pass (no regressions)
- [x] 3.3 Run `go test ./pkg/...` -- all pass
