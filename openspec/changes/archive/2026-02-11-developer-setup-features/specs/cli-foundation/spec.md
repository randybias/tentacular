## MODIFIED Requirements

### Requirement: CLI binary compiles and runs
The `tentacular` CLI SHALL compile from `cmd/tntc/main.go` into a single Go binary and display help when invoked with `--help`.

#### Scenario: Successful build
- **WHEN** `go build ./cmd/tntc/` is executed
- **THEN** a `tentacular` binary is produced with exit code 0

#### Scenario: Help output shows all commands
- **WHEN** `tntc --help` is executed
- **THEN** output SHALL list: init, validate, dev, test, build, deploy, status, run, logs, list, undeploy, cluster, visualize, configure

## ADDED Requirements

### Requirement: List command shows version column
The `tntc list` command SHALL display a VERSION column in its output.

#### Scenario: Version in text output
- **WHEN** `tntc list` is executed with default text output
- **THEN** the table header SHALL include a VERSION column between NAME and NAMESPACE
- **AND** each row SHALL show the workflow version from `app.kubernetes.io/version` label

#### Scenario: Version in JSON output
- **WHEN** `tntc list --output json` is executed
- **THEN** each workflow entry SHALL include a `version` field

#### Scenario: Missing version label
- **WHEN** a deployed workflow has no `app.kubernetes.io/version` label (pre-upgrade deployment)
- **THEN** the version column SHALL display an empty string

### Requirement: Build command reads config defaults
The `tntc build` command SHALL read registry default from config files when `--registry` is not explicitly set.

#### Scenario: Registry from config
- **WHEN** `tntc build` is executed without `--registry` and config file has `registry: gcr.io/proj`
- **THEN** the registry value SHALL be `gcr.io/proj`

#### Scenario: Flag overrides config
- **WHEN** `tntc build --registry override.io` is executed and config file has `registry: gcr.io/proj`
- **THEN** the registry value SHALL be `override.io`

### Requirement: Deploy command reads config defaults
The `tntc deploy` command SHALL read runtime-class default from config files when `--runtime-class` is not explicitly set.

#### Scenario: Runtime-class from config
- **WHEN** `tntc deploy` is executed without `--runtime-class` and config file has `runtime_class: gvisor`
- **THEN** the runtime class SHALL be `gvisor`

#### Scenario: Runtime-class flag overrides config
- **WHEN** `tntc deploy --runtime-class kata` is executed and config file has `runtime_class: gvisor`
- **THEN** the runtime class SHALL be `kata`
