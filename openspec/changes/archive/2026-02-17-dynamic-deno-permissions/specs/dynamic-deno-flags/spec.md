## ADDED Requirements

### Requirement: DeriveDenoFlags produces Deno command from contract
The `DeriveDenoFlags` function SHALL accept a `*Contract` and return a `[]string` representing the full `deno run` command with derived permission flags. If the contract is nil or has no dependencies, it SHALL return nil.

#### Scenario: Contract with HTTPS dependencies
- **WHEN** `DeriveDenoFlags` is called with a contract containing HTTPS dependencies to `api.github.com:443` and `hooks.slack.com:443`
- **THEN** the result SHALL contain `--allow-net=0.0.0.0:8080,api.github.com:443,hooks.slack.com:443` (sorted alphabetically after the listen address)

#### Scenario: Contract with PostgreSQL dependency
- **WHEN** `DeriveDenoFlags` is called with a contract containing a PostgreSQL dependency to `db.example.com:5432`
- **THEN** the result SHALL contain `--allow-net=0.0.0.0:8080,db.example.com:5432`

#### Scenario: Nil contract returns nil
- **WHEN** `DeriveDenoFlags` is called with a nil contract
- **THEN** the result SHALL be nil

#### Scenario: Contract with no host-based dependencies returns nil
- **WHEN** `DeriveDenoFlags` is called with a contract whose dependencies have no host fields
- **THEN** the result SHALL be nil

#### Scenario: Dynamic-target dependency triggers permissive fallback
- **WHEN** `DeriveDenoFlags` is called with a contract containing any `type: dynamic-target` dependency
- **THEN** the result SHALL be nil (fall back to permissive ENTRYPOINT `--allow-net`)

#### Scenario: Default ports applied
- **WHEN** `DeriveDenoFlags` is called with a contract where a dependency has `protocol: https` but no explicit port
- **THEN** the default port for the protocol SHALL be used (443 for HTTPS, 5432 for PostgreSQL, etc.)

### Requirement: Deployment injects command/args when contract provides flags
When `DeployOptions.Contract` is non-nil and `DeriveDenoFlags` returns a non-nil result, the generated Deployment container spec SHALL include `command` and `args` fields overriding the Dockerfile ENTRYPOINT.

#### Scenario: Deployment with contract-derived flags
- **WHEN** `GenerateK8sManifests` is called with a `DeployOptions` containing a non-nil contract with HTTPS dependencies
- **THEN** the Deployment manifest SHALL contain a `command:` field with `["deno"]`
- **AND** the manifest SHALL contain an `args:` field with the derived flags including `--allow-net=` with specific hosts

#### Scenario: Deployment without contract uses ENTRYPOINT defaults
- **WHEN** `GenerateK8sManifests` is called with nil contract in `DeployOptions`
- **THEN** the Deployment manifest SHALL NOT contain `command:` or `args:` fields

#### Scenario: Deployment with empty contract uses ENTRYPOINT defaults
- **WHEN** `GenerateK8sManifests` is called with a contract that has no dependencies
- **THEN** the Deployment manifest SHALL NOT contain `command:` or `args:` fields

### Requirement: Derived flags include all static permission flags
The derived command SHALL include all the same static flags as the Dockerfile ENTRYPOINT: `--no-lock`, `--unstable-net`, `--allow-read=/app,/var/run/secrets`, `--allow-write=/tmp`, `--allow-env`, and the engine entry point and workflow arguments.

#### Scenario: All static flags preserved
- **WHEN** `DeriveDenoFlags` returns a non-nil result
- **THEN** the result SHALL contain `--allow-read=/app,/var/run/secrets`, `--allow-write=/tmp`, `--allow-env`, `--no-lock`, `--unstable-net`, `engine/main.ts`, `--workflow`, `/app/workflow/workflow.yaml`, `--port`, `8080`
