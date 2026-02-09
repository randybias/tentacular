## 1. Type Definitions

- [x] 1.1 Define Workflow, Trigger, NodeSpec, Edge, WorkflowConfig structs in `pkg/spec/types.go` with yaml struct tags
- [x] 1.2 Verify all struct fields match the v2 workflow.yaml format (name, version, description, triggers, nodes, edges, config)

## 2. Parser and Validation

- [x] 2.1 Implement `Parse(data []byte) (*Workflow, []string)` in `pkg/spec/parse.go` using `gopkg.in/yaml.v3`
- [x] 2.2 Validate required fields: name, version, triggers (non-empty), nodes (non-empty)
- [x] 2.3 Validate naming conventions: kebab-case for workflow name, identifier pattern for node names, semver for version
- [x] 2.4 Validate trigger types (manual, cron, webhook) and required sub-fields (cron requires schedule, webhook requires path)
- [x] 2.5 Validate edge reference integrity: from/to must reference defined nodes, no self-loops
- [x] 2.6 Implement DAG acyclicity check using DFS three-color algorithm
- [x] 2.7 Validate node path is required for every node

## 3. CLI Command

- [x] 3.1 Implement `pipedreamer validate [dir]` in `pkg/cli/validate.go` that reads workflow.yaml and calls `spec.Parse()`
- [x] 3.2 Print validation errors to stderr and success message to stdout
- [x] 3.3 Support optional directory argument (default to current directory)

## 4. Tests

- [x] 4.1 Test valid spec parsing: complete workflow returns populated struct and no errors
- [x] 4.2 Test missing name: returns "name is required" error
- [x] 4.3 Test invalid name: non-kebab-case name returns naming error
- [x] 4.4 Test cycle detection: cyclic edges return cycle error
- [x] 4.5 Test edge reference integrity: undefined node reference returns error
- [x] 4.6 Test trigger validation: cron without schedule returns error

## 5. Integration Verification

- [x] 5.1 Verify `go build ./cmd/pipedreamer/` compiles successfully
- [x] 5.2 Verify `go test ./pkg/spec/ -v` passes all tests
- [x] 5.3 Verify `pipedreamer validate` works end-to-end with a scaffolded workflow
