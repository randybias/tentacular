## ADDED Requirements

### Requirement: Context interface
The Context SHALL be a plain object (not a class instance) with four members: `fetch`, `log`, `config`, and `secrets`.

#### Scenario: Context creation
- **WHEN** `createContext(opts)` is called with options containing nodeId, secrets, and config
- **THEN** it SHALL return a plain object satisfying the Context interface

#### Scenario: Context members
- **WHEN** a Context object is inspected
- **THEN** it SHALL have: `fetch` (function), `log` (Logger object), `config` (Record), and `secrets` (Record)

### Requirement: ctx.fetch resolves service names and injects auth
The `ctx.fetch(service, path, init?)` function SHALL resolve service names to URLs by convention and inject authentication headers from loaded secrets.

#### Scenario: Convention-based URL resolution
- **WHEN** `ctx.fetch("github", "/repos/owner/repo")` is called
- **THEN** the request SHALL be sent to `https://api.github.com/repos/owner/repo`

#### Scenario: Full URL passthrough
- **WHEN** `ctx.fetch("slack", "https://hooks.slack.com/services/T00/B00/xxx")` is called
- **THEN** the request SHALL be sent to `https://hooks.slack.com/services/T00/B00/xxx` as-is

#### Scenario: Bearer token injection
- **WHEN** `ctx.fetch("github", "/repos")` is called and `secrets["github"]["token"]` is `"ghp_abc123"`
- **THEN** the request SHALL include the header `Authorization: Bearer ghp_abc123`

#### Scenario: API key injection
- **WHEN** `ctx.fetch("openai", "/v1/chat")` is called and `secrets["openai"]["api_key"]` is `"sk-abc"`
- **THEN** the request SHALL include the header `X-API-Key: sk-abc`

#### Scenario: No secrets for service
- **WHEN** `ctx.fetch("example", "/data")` is called and no secrets exist for "example"
- **THEN** the request SHALL proceed without auth headers (not an error)

#### Scenario: Custom headers preserved
- **WHEN** `ctx.fetch("github", "/repos", { headers: { "Accept": "application/json" } })` is called
- **THEN** the request SHALL include both the injected auth header and the custom `Accept` header

### Requirement: ctx.log provides structured logging with node ID prefix
The `ctx.log` object SHALL provide `info`, `warn`, `error`, and `debug` methods that prefix output with the node ID.

#### Scenario: Info logging
- **WHEN** `ctx.log.info("processing", { count: 5 })` is called from node "transform"
- **THEN** the output SHALL be prefixed with `[transform] INFO`

#### Scenario: Warn logging
- **WHEN** `ctx.log.warn("rate limited")` is called from node "fetch-data"
- **THEN** the output SHALL be prefixed with `[fetch-data] WARN`

#### Scenario: Error logging
- **WHEN** `ctx.log.error("request failed", err)` is called from node "api-call"
- **THEN** the output SHALL be prefixed with `[api-call] ERROR`

#### Scenario: Debug logging
- **WHEN** `ctx.log.debug("payload", data)` is called from node "validate"
- **THEN** the output SHALL be prefixed with `[validate] DEBUG`

### Requirement: ctx.config provides node-specific configuration
The `ctx.config` property SHALL be a `Record<string, unknown>` containing node-specific configuration from `workflow.yaml`.

#### Scenario: Config access
- **WHEN** a node's config in `workflow.yaml` specifies `{ retries: 3, timeout: "30s" }`
- **THEN** `ctx.config` SHALL contain `{ retries: 3, timeout: "30s" }`

#### Scenario: Empty config
- **WHEN** a node has no config specified in `workflow.yaml`
- **THEN** `ctx.config` SHALL be an empty object `{}`

### Requirement: Secrets loading from YAML file
The `loadSecrets(path)` function SHALL load secrets from a `.secrets.yaml` file keyed by service name.

#### Scenario: Load from YAML file
- **WHEN** `loadSecrets(".secrets.yaml")` is called and the file contains `github: { token: "ghp_abc" }`
- **THEN** the returned object SHALL be `{ github: { token: "ghp_abc" } }`

#### Scenario: Missing secrets file
- **WHEN** `loadSecrets(".secrets.yaml")` is called and the file does not exist
- **THEN** the returned object SHALL be `{}` (empty, not an error)

#### Scenario: Invalid YAML
- **WHEN** `loadSecrets(".secrets.yaml")` is called and the file contains invalid YAML or non-object content
- **THEN** the returned object SHALL be `{}`

### Requirement: Secrets loading from K8s volume mount directory
The `loadSecrets(path)` function SHALL load secrets from a directory of files (K8s Secret volume mount pattern).

#### Scenario: Load from directory
- **WHEN** `loadSecrets("/secrets/")` is called and the directory contains a file `github` with JSON content `{ "token": "ghp_abc" }`
- **THEN** the returned object SHALL be `{ github: { token: "ghp_abc" } }`

#### Scenario: Plain text secret file
- **WHEN** `loadSecrets("/secrets/")` is called and the directory contains a file `api-key` with content `sk-abc123`
- **THEN** the returned object SHALL be `{ "api-key": { value: "sk-abc123" } }`

#### Scenario: Hidden files skipped
- **WHEN** the secrets directory contains a file `.hidden`
- **THEN** it SHALL be skipped and not included in the loaded secrets

#### Scenario: Auto-detect file vs directory
- **WHEN** `loadSecrets(path)` is called
- **THEN** it SHALL auto-detect whether `path` is a file or directory and use the appropriate loading strategy

### Requirement: Public module exports
The `engine/mod.ts` SHALL export `Context`, `Logger`, and `NodeFunction` types so that node authors can import them from `"tentacular"`.

#### Scenario: Context type import
- **WHEN** a node file contains `import type { Context } from "tentacular"`
- **THEN** the import SHALL resolve to the Context interface defined in `engine/types.ts`

#### Scenario: All node-author types available
- **WHEN** `engine/mod.ts` is imported
- **THEN** it SHALL export at minimum: `Context`, `Logger`, `NodeFunction`
