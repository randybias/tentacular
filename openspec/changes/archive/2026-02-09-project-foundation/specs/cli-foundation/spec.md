## ADDED Requirements

### Requirement: CLI binary compiles and runs
The `tentacular` CLI SHALL compile from `cmd/tntc/main.go` into a single Go binary and display help when invoked with `--help`.

#### Scenario: Successful build
- **WHEN** `go build ./cmd/tntc/` is executed
- **THEN** a `tentacular` binary is produced with exit code 0

#### Scenario: Help output shows all commands
- **WHEN** `tntc --help` is executed
- **THEN** output SHALL list: init, validate, dev, test, build, deploy, status, cluster, visualize

### Requirement: Global flags
The CLI SHALL support global flags: `--namespace` (default "default"), `--registry`, `--output` (text|json), `--verbose`.

#### Scenario: Namespace flag
- **WHEN** any command is invoked with `--namespace prod`
- **THEN** the namespace value "prod" SHALL be available to the command handler

#### Scenario: Default namespace
- **WHEN** any command is invoked without `--namespace`
- **THEN** the namespace value SHALL default to "default"

### Requirement: Init command scaffolds workflow
The `tntc init <name>` command SHALL create a new workflow directory with the required scaffold files.

#### Scenario: Successful init
- **WHEN** `tntc init my-workflow` is executed
- **THEN** a directory `my-workflow/` SHALL be created containing:
  - `workflow.yaml` with the workflow name set to "my-workflow"
  - `nodes/hello.ts` with a default node implementation
  - `.secrets.yaml.example` with documentation
  - `tests/fixtures/hello.json` with test fixture

#### Scenario: Invalid name rejected
- **WHEN** `tntc init NotKebab` is executed
- **THEN** the command SHALL fail with an error indicating kebab-case is required

#### Scenario: Name validation
- **WHEN** the name argument is checked
- **THEN** it SHALL match the pattern `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`
