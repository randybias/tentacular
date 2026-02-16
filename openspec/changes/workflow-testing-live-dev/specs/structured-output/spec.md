## ADDED Requirements

### Requirement: Common JSON envelope for all commands
When `-o json` is set, all CLI commands SHALL emit a `CommandResult` JSON envelope with fields: `version` (string, always "1"), `command` (string), `status` ("pass" or "fail"), `summary` (string), `hints` (string array), `timing` (object with `startedAt` ISO8601 string and `durationMs` integer).

#### Scenario: Successful command JSON output
- **WHEN** `tntc validate -o json` succeeds
- **THEN** the output SHALL be a JSON object with `version: "1"`, `command: "validate"`, `status: "pass"`, a non-empty `summary`, and `timing` with `startedAt` and `durationMs`

#### Scenario: Failed command JSON output with hints
- **WHEN** `tntc test -o json` has test failures
- **THEN** the output SHALL have `status: "fail"` and `hints` SHALL contain actionable suggestions

### Requirement: Command-specific result fields
The `CommandResult` envelope SHALL support optional command-specific fields: `results` (for test results), `phases` (for phased operations like deploy), `execution` (for run results), `manifests` (for deploy manifest actions). Fields not relevant to a command SHALL be omitted from JSON output.

#### Scenario: Test command includes results
- **WHEN** `tntc test -o json` completes
- **THEN** the JSON SHALL include a `results` field with per-node test results

#### Scenario: Deploy command includes phases and manifests
- **WHEN** `tntc deploy -o json` completes
- **THEN** the JSON SHALL include `phases` and `manifests` fields

### Requirement: EmitResult dispatches on output flag
`EmitResult()` SHALL check the `-o` / `--output` flag. When set to `json`, it SHALL marshal the `CommandResult` as compact JSON to the provided writer. Otherwise, it SHALL emit a human-readable text summary with status icon, summary, hints, and timing.

#### Scenario: Text output format
- **WHEN** `EmitResult` is called without `-o json`
- **THEN** output SHALL be `[PASS] <summary>` or `[FAIL] <summary>` with hints on subsequent lines

### Requirement: Deno test runner JSON mode
The Deno test runner (`engine/testing/runner.ts`) SHALL accept a `--json` flag. When set, it SHALL output `TestResult[]` as JSON to stdout instead of the human-readable test report.

#### Scenario: Deno runner JSON output
- **WHEN** the test runner is invoked with `--json`
- **THEN** stdout SHALL contain a JSON array of test result objects
