## ADDED Requirements

### Requirement: Secrets check scans node source files
The `tntc secrets check <workflow-dir>` command SHALL scan node source files for `ctx.secrets` access patterns and compare against locally provisioned secrets.

#### Scenario: All secrets provisioned
- **WHEN** `tntc secrets check` is run on a workflow where all `ctx.secrets` references are covered by `.secrets.yaml`
- **THEN** it SHALL report all required secrets as provisioned and exit with code 0

#### Scenario: Missing secrets reported
- **WHEN** `tntc secrets check` is run on a workflow where some `ctx.secrets` references have no matching entry in `.secrets.yaml` or `.secrets/`
- **THEN** it SHALL list each missing secret with its service name
- **AND** it SHALL suggest running `tntc secrets init`

#### Scenario: No node files
- **WHEN** `tntc secrets check` is run on a workflow with no `nodes/*.ts` files
- **THEN** it SHALL report no required secrets found

#### Scenario: Regex extraction
- **WHEN** a node file contains `ctx.secrets?.slack?.webhook_url` or `ctx.secrets.postgres`
- **THEN** the check command SHALL extract `slack` and `postgres` as required service names

### Requirement: Secrets init scaffolds from example
The `tntc secrets init <workflow-dir>` command SHALL copy `.secrets.yaml.example` to `.secrets.yaml` with example comments uncommented.

#### Scenario: Successful init
- **WHEN** `tntc secrets init` is run and `.secrets.yaml.example` exists but `.secrets.yaml` does not
- **THEN** it SHALL create `.secrets.yaml` with uncommented content from the example file

#### Scenario: Refuses overwrite
- **WHEN** `tntc secrets init` is run and `.secrets.yaml` already exists
- **THEN** it SHALL return an error indicating the file exists (use `--force` to overwrite)

#### Scenario: Missing example file
- **WHEN** `tntc secrets init` is run and `.secrets.yaml.example` does not exist
- **THEN** it SHALL return an error indicating no example file was found

### Requirement: Shared secrets pool resolution
The `$shared.<name>` syntax in `.secrets.yaml` SHALL resolve to files in the repo-root `.secrets/` directory.

#### Scenario: Shared secret resolved
- **WHEN** `.secrets.yaml` contains `slack: $shared.slack` and `<repo-root>/.secrets/slack` exists with JSON content
- **THEN** the `slack` key SHALL be resolved to the parsed JSON content from the shared file

#### Scenario: Shared secret file missing
- **WHEN** `.secrets.yaml` references `$shared.nonexistent` and `<repo-root>/.secrets/nonexistent` does not exist
- **THEN** `buildSecretFromYAML()` SHALL return an error indicating the shared secret was not found

#### Scenario: No repo root found
- **WHEN** `$shared.` references are present but no `.git/` or `go.mod` is found walking up from the workflow directory
- **THEN** shared secret resolution SHALL be silently skipped (no error)

#### Scenario: Plain text shared secret
- **WHEN** a shared secret file contains plain text (not valid JSON)
- **THEN** the value SHALL be used as a trimmed string
