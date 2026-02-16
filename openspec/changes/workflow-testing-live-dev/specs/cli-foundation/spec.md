## ADDED Requirements

### Requirement: Named environment configuration
The config cascade SHALL support named environments with deployment parameters.

#### Scenario: Environment config structure
- **GIVEN** a config YAML with an `environments` map
- **WHEN** `LoadEnvironment("dev")` is called
- **THEN** it SHALL return an EnvironmentConfig with: Context, Namespace, Image, RuntimeClass, ConfigOverrides, SecretsSource

#### Scenario: Environment not found
- **WHEN** `LoadEnvironment("nonexistent")` is called and no such environment is configured
- **THEN** it SHALL return an error indicating the environment was not found

#### Scenario: Environment inherits top-level defaults
- **GIVEN** a config with top-level `namespace: default-ns` and environment `dev` with no namespace
- **WHEN** `LoadEnvironment("dev")` is called
- **THEN** the returned config SHALL inherit the top-level namespace

#### Scenario: Environment overrides top-level
- **GIVEN** a config with top-level `namespace: default-ns` and environment `dev` with `namespace: dev-ns`
- **WHEN** `LoadEnvironment("dev")` is called
- **THEN** the returned config SHALL use `dev-ns`

### Requirement: Structured JSON output envelope
All commands SHALL support `-o json` / `--output json` for agent-consumable output.

#### Scenario: JSON envelope structure
- **WHEN** any command emits output with `-o json`
- **THEN** the output SHALL be a JSON object with fields: version ("1"), command, status ("pass"|"fail"), summary, hints (array), timing (startedAt ISO8601, durationMs)

#### Scenario: Text output default
- **WHEN** any command emits output without `-o json`
- **THEN** existing text output behavior SHALL be preserved

#### Scenario: Command-specific fields
- **WHEN** `tntc test -o json` emits output
- **THEN** the envelope SHALL include a `results` field with test result details
- **WHEN** `tntc deploy -o json` emits output
- **THEN** the envelope SHALL include a `phases` field with deploy phase details
- **WHEN** `tntc run -o json` emits output
- **THEN** the envelope SHALL include an `execution` field with execution result details
