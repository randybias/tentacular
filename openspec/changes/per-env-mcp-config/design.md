# Design: Per-environment MCP configuration

## Config Schema Changes

### EnvironmentConfig additions

```go
type EnvironmentConfig struct {
    Kubeconfig      string                 `yaml:"kubeconfig,omitempty"`
    Context         string                 `yaml:"context,omitempty"`
    Namespace       string                 `yaml:"namespace,omitempty"`
    Image           string                 `yaml:"image,omitempty"`
    RuntimeClass    string                 `yaml:"runtime_class,omitempty"`
    ConfigOverrides map[string]interface{} `yaml:"config_overrides,omitempty"`
    SecretsSource   string                 `yaml:"secrets_source,omitempty"`
    Enforcement     string                 `yaml:"enforcement,omitempty"`
    MCPEndpoint     string                 `yaml:"mcp_endpoint,omitempty"`     // NEW
    MCPTokenPath    string                 `yaml:"mcp_token_path,omitempty"`   // NEW
}
```

### TentacularConfig additions

```go
type TentacularConfig struct {
    Registry     string                       `yaml:"registry,omitempty"`
    Namespace    string                       `yaml:"namespace,omitempty"`
    RuntimeClass string                       `yaml:"runtime_class,omitempty"`
    DefaultEnv   string                       `yaml:"default_env,omitempty"`  // NEW
    Environments map[string]EnvironmentConfig `yaml:"environments,omitempty"`
    ModuleProxy  ModuleProxyConfig            `yaml:"moduleProxy,omitempty"`
    MCP          MCPConfig                    `yaml:"mcp,omitempty"`
}
```

## MCP Client Resolution

The `requireMCPClient` helper (used by all MCP-routed commands) currently reads
from the global `cfg.MCP` block. The updated resolution cascade:

1. CLI flags (`--mcp-endpoint`, `--mcp-token` if we add them -- optional).
2. Active environment's `mcp_endpoint` and `mcp_token_path`.
3. Global `mcp.endpoint` and `mcp.token_path`.
4. Error: "no MCP endpoint configured".

The "active environment" is determined by:

1. `--env` flag (explicit).
2. `TENTACULAR_ENV` env var.
3. `default_env` config field.
4. Empty (falls back to global config).

## DefaultEnv Resolution

Update `ResolveEnvironment` in `pkg/cli/environment.go`:

```go
func ResolveEnvironment(envName string) (*EnvironmentConfig, error) {
    if envName == "" {
        envName = os.Getenv("TENTACULAR_ENV")
    }
    cfg := LoadConfig()
    if envName == "" {
        envName = cfg.DefaultEnv
    }
    return cfg.LoadEnvironment(envName)
}
```

## Backwards Compatibility

- If `default_env` is not set and `--env` is not passed, behavior is identical
  to today (global MCP config is used).
- The global `mcp` block remains functional and serves as the fallback.
- No existing config files need to be modified to continue working.
