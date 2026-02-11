## ADDED Requirements

### Requirement: Valid workflow spec parsing
The parser SHALL accept a valid v2 workflow.yaml and return a populated Workflow struct with nil error slice.

#### Scenario: Complete valid spec
- **WHEN** a workflow.yaml with valid `name` (kebab-case), `version` (semver major.minor), at least one trigger, at least one node with path, and valid edges is parsed
- **THEN** `spec.Parse()` SHALL return a non-nil `*Workflow` and an empty error slice

#### Scenario: Workflow fields populated
- **WHEN** a valid workflow.yaml with name "test-workflow", version "1.0", 2 nodes, and 1 edge is parsed
- **THEN** the returned Workflow SHALL have `Name == "test-workflow"`, `Version == "1.0"`, `len(Nodes) == 2`, `len(Edges) == 1`

### Requirement: Invalid spec rejection with errors
The parser SHALL reject invalid workflow specs and return a slice of human-readable error strings describing all validation failures.

#### Scenario: Missing required name
- **WHEN** a workflow.yaml without a `name` field is parsed
- **THEN** the error slice SHALL contain "name is required"

#### Scenario: Invalid name format
- **WHEN** a workflow.yaml with `name: NotKebab` is parsed
- **THEN** the error slice SHALL contain an error indicating the name must be kebab-case

#### Scenario: Missing version
- **WHEN** a workflow.yaml without a `version` field is parsed
- **THEN** the error slice SHALL contain "version is required"

#### Scenario: Invalid version format
- **WHEN** a workflow.yaml with `version: "abc"` is parsed
- **THEN** the error slice SHALL contain an error indicating the version must be semver

#### Scenario: Missing triggers
- **WHEN** a workflow.yaml with no triggers is parsed
- **THEN** the error slice SHALL contain "at least one trigger is required"

#### Scenario: Missing nodes
- **WHEN** a workflow.yaml with no nodes is parsed
- **THEN** the error slice SHALL contain "at least one node is required"

#### Scenario: Node missing path
- **WHEN** a node is defined without a `path` field
- **THEN** the error slice SHALL contain an error indicating path is required for that node

#### Scenario: Multiple errors reported
- **WHEN** a workflow.yaml has multiple validation failures
- **THEN** the error slice SHALL contain all errors, not just the first one

### Requirement: Trigger validation
The parser SHALL validate trigger types and their required sub-fields.

#### Scenario: Invalid trigger type
- **WHEN** a trigger has a type other than "manual", "cron", or "webhook"
- **THEN** the error slice SHALL contain an error indicating the invalid trigger type

#### Scenario: Cron trigger missing schedule
- **WHEN** a trigger of type "cron" has no `schedule` field
- **THEN** the error slice SHALL contain an error indicating cron trigger requires schedule

#### Scenario: Webhook trigger missing path
- **WHEN** a trigger of type "webhook" has no `path` field
- **THEN** the error slice SHALL contain an error indicating webhook trigger requires path

#### Scenario: Valid manual trigger
- **WHEN** a trigger of type "manual" is provided
- **THEN** it SHALL be accepted with no errors (manual triggers have no required sub-fields)

### Requirement: Edge reference integrity
The parser SHALL verify that all edge `from` and `to` fields reference defined nodes.

#### Scenario: Edge references undefined node
- **WHEN** an edge has `to: nonexistent` and no node named "nonexistent" is defined
- **THEN** the error slice SHALL contain an error indicating the referenced node is not defined

#### Scenario: Self-loop detection
- **WHEN** an edge has `from: a` and `to: a`
- **THEN** the error slice SHALL contain an error indicating a self-loop

### Requirement: DAG acyclicity check
The parser SHALL detect cycles in the workflow graph and report them as validation errors.

#### Scenario: Simple cycle detection
- **WHEN** edges form a cycle (e.g., a->b->c->a)
- **THEN** the error slice SHALL contain an error with the word "cycle"

#### Scenario: Acyclic graph accepted
- **WHEN** edges form a valid DAG with no cycles
- **THEN** no cycle-related errors SHALL be present in the error slice

### Requirement: Validate CLI command
The `tntc validate [dir]` command SHALL read workflow.yaml from the specified directory (or current directory) and run the parser/validator.

#### Scenario: Valid workflow
- **WHEN** `tntc validate` is run in a directory containing a valid workflow.yaml
- **THEN** the command SHALL print a success message and exit with code 0

#### Scenario: Invalid workflow
- **WHEN** `tntc validate` is run in a directory containing an invalid workflow.yaml
- **THEN** the command SHALL print validation errors to stderr and exit with a non-zero exit code

#### Scenario: Missing workflow file
- **WHEN** `tntc validate` is run in a directory without workflow.yaml
- **THEN** the command SHALL exit with an error indicating the file could not be read

#### Scenario: Custom directory
- **WHEN** `tntc validate ./my-workflow` is run
- **THEN** the command SHALL look for `my-workflow/workflow.yaml`
