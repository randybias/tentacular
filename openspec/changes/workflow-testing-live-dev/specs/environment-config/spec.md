## ADDED Requirements

### Requirement: Named environment definitions in config files
The config cascade (`~/.tentacular/config.yaml`, `.tentacular/config.yaml`) SHALL support an `environments` map where each key is an environment name and each value is an `EnvironmentConfig` with fields: `context` (string), `namespace` (string), `image` (string), `runtime_class` (string), `config_overrides` (map[string]interface{}), `secrets_source` (string). All fields are optional.

#### Scenario: Load environment from user config
- **WHEN** `~/.tentacular/config.yaml` contains `environments.dev` with `context: kind-dev` and `namespace: dev-workflows`
- **THEN** `LoadEnvironment("dev")` SHALL return an `EnvironmentConfig` with `Context: "kind-dev"` and `Namespace: "dev-workflows"`

#### Scenario: Project config overrides user config environments
- **WHEN** user config defines `environments.dev.namespace: user-ns` and project config defines `environments.dev.namespace: proj-ns`
- **THEN** `LoadEnvironment("dev")` SHALL return `Namespace: "proj-ns"`

#### Scenario: Environment not found
- **WHEN** `LoadEnvironment("nonexistent")` is called and no environment with that name exists
- **THEN** the function SHALL return an error containing "not found"

### Requirement: Config overrides merge into workflow config
`ApplyConfigOverrides()` SHALL merge environment `config_overrides` into a workflow config map. Override values replace existing keys; new keys are added.

#### Scenario: Override existing config key
- **WHEN** workflow config has `timeout: 30s` and environment has `config_overrides.timeout: 60s`
- **THEN** after `ApplyConfigOverrides()`, workflow config SHALL have `timeout: 60s`

#### Scenario: Add new config key from overrides
- **WHEN** workflow config has `timeout: 30s` and environment has `config_overrides.debug: true`
- **THEN** after `ApplyConfigOverrides()`, workflow config SHALL have both `timeout: 30s` and `debug: true`

### Requirement: Environment config resolution order
CLI flags SHALL override environment config, which SHALL override workflow.yaml settings, which SHALL override project config, which SHALL override user config, which SHALL override defaults.

#### Scenario: CLI flag overrides environment namespace
- **WHEN** environment config specifies `namespace: dev-ns` and the CLI `-n production` flag is set
- **THEN** the deploy SHALL target the `production` namespace
