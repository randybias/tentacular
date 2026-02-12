## ADDED Requirements

### Requirement: Configure command writes config file
The `tntc configure` command SHALL write configuration defaults to a YAML config file.

#### Scenario: User-level config
- **WHEN** `tntc configure --registry nats.ospo-dev.miralabs.dev:30500 --namespace default`
- **THEN** the values SHALL be written to `~/.tentacular/config.yaml`
- **AND** the directory `~/.tentacular/` SHALL be created if it does not exist

#### Scenario: Project-level config
- **WHEN** `tntc configure --registry nats.ospo-dev.miralabs.dev:30500 --project`
- **THEN** the values SHALL be written to `.tentacular/config.yaml` in the current working directory
- **AND** the directory `.tentacular/` SHALL be created if it does not exist

#### Scenario: Show current config after writing
- **WHEN** `tntc configure` writes a config file
- **THEN** it SHALL print the resulting configuration values to stdout

#### Scenario: Partial flag update
- **WHEN** `tntc configure --namespace prod` is run and the config file already contains `registry: gcr.io/proj`
- **THEN** the config file SHALL contain both `namespace: prod` and `registry: gcr.io/proj`

### Requirement: Configure command flags
The `tntc configure` command SHALL accept flags for each configurable field.

#### Scenario: Registry flag
- **WHEN** `tntc configure --registry gcr.io/proj` is run
- **THEN** the config file SHALL contain `registry: gcr.io/proj`

#### Scenario: Namespace flag
- **WHEN** `tntc configure --namespace prod` is run
- **THEN** the config file SHALL contain `namespace: prod`

#### Scenario: Runtime-class flag
- **WHEN** `tntc configure --runtime-class gvisor` is run
- **THEN** the config file SHALL contain `runtime_class: gvisor`

#### Scenario: Project flag
- **WHEN** `tntc configure --project` is used
- **THEN** the output SHALL be written to `.tentacular/config.yaml` instead of `~/.tentacular/config.yaml`

### Requirement: Config loading with two-tier merge
The `LoadConfig()` function SHALL load and merge user-level and project-level config files.

#### Scenario: User-level only
- **WHEN** `~/.tentacular/config.yaml` exists with `registry: gcr.io/proj` and no project-level config exists
- **THEN** `LoadConfig()` SHALL return `TentacularConfig{Registry: "gcr.io/proj"}`

#### Scenario: Project overrides user
- **WHEN** user-level config has `namespace: staging` and project-level config has `namespace: prod`
- **THEN** `LoadConfig()` SHALL return config with `Namespace: "prod"`

#### Scenario: No config files
- **WHEN** neither `~/.tentacular/config.yaml` nor `.tentacular/config.yaml` exists
- **THEN** `LoadConfig()` SHALL return a zero-value `TentacularConfig` (all empty strings)

#### Scenario: Partial merge
- **WHEN** user-level has `registry: gcr.io/proj` and project-level has `namespace: prod` (no registry)
- **THEN** `LoadConfig()` SHALL return config with both `Registry: "gcr.io/proj"` and `Namespace: "prod"`
