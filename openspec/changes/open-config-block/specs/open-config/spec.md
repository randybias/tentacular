## ADDED Requirements

### Requirement: Arbitrary config keys survive Go parsing
The Go `WorkflowConfig` struct SHALL accept arbitrary YAML keys via an inline extras map (`map[string]interface{}`). Unknown keys MUST NOT be silently dropped during YAML unmarshaling.

#### Scenario: Custom config key is preserved
- **WHEN** a workflow YAML contains `config: { timeout: "30s", nats_url: "nats://localhost:4222" }`
- **THEN** parsing produces a `WorkflowConfig` with `Timeout="30s"` and `Extras["nats_url"]="nats://localhost:4222"`

#### Scenario: Multiple custom keys
- **WHEN** a workflow YAML contains `config: { nats_url: "nats://host", custom_key: "value", retries: 3 }`
- **THEN** `Extras` contains both `nats_url` and `custom_key`, and `Retries` equals 3

### Requirement: ToMap produces flat merged output
The `WorkflowConfig` SHALL provide a `ToMap()` method that returns a `map[string]interface{}` merging typed fields and extras into a single flat map.

#### Scenario: ToMap includes typed and extra fields
- **WHEN** `WorkflowConfig` has `Timeout="30s"`, `Retries=2`, and `Extras={"nats_url": "test"}`
- **THEN** `ToMap()` returns `{"timeout": "30s", "retries": 2, "nats_url": "test"}`

#### Scenario: ToMap omits zero-valued typed fields
- **WHEN** `WorkflowConfig` has `Timeout=""`, `Retries=0`, and `Extras={"nats_url": "test"}`
- **THEN** `ToMap()` returns `{"nats_url": "test"}` (no `timeout` or `retries` keys)

#### Scenario: Typed fields take precedence over extras
- **WHEN** YAML contains `config: { timeout: "30s" }` and the inline map also captures `timeout`
- **THEN** the typed `Timeout` field value is used (Go yaml.v3 inline behavior gives typed fields precedence)

### Requirement: TypeScript WorkflowConfig accepts arbitrary keys
The `WorkflowConfig` interface in `engine/types.ts` SHALL include an index signature `[key: string]: unknown` so custom config keys are type-safe.

#### Scenario: Custom key access in TypeScript
- **WHEN** TypeScript code accesses `config.nats_url` on a `WorkflowConfig` object
- **THEN** the type system allows the access with type `unknown` (no cast needed)
