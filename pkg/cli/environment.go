package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvironmentConfig holds per-environment overrides.
type EnvironmentConfig struct {
	Kubeconfig      string                 `yaml:"kubeconfig,omitempty"`
	Context         string                 `yaml:"context,omitempty"`
	Namespace       string                 `yaml:"namespace,omitempty"`
	Image           string                 `yaml:"image,omitempty"`
	RuntimeClass    string                 `yaml:"runtime_class,omitempty"`
	ConfigOverrides map[string]interface{} `yaml:"config_overrides,omitempty"`
	SecretsSource   string                 `yaml:"secrets_source,omitempty"`
	Enforcement     string                 `yaml:"enforcement,omitempty"` // "strict" (default) or "audit"
	MCPEndpoint     string                 `yaml:"mcp_endpoint,omitempty"`
	MCPTokenPath    string                 `yaml:"mcp_token_path,omitempty"`

	// OIDC fields (optional). When present, `tntc login` uses device authorization flow.
	OIDCIssuer       string `yaml:"oidc_issuer,omitempty"`
	OIDCClientID     string `yaml:"oidc_client_id,omitempty"`
	OIDCClientSecret string `yaml:"oidc_client_secret,omitempty"`
}

// ResolveEnvironment loads the merged config and returns the named environment.
// Resolution cascade: explicit envName > TENTACULAR_ENV env var > default_env config > "" (top-level defaults).
// When envName is empty (and TENTACULAR_ENV is unset and default_env is not set),
// returns top-level config promoted to an EnvironmentConfig.
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

// LoadEnvironment is a package-level convenience that loads config and looks up
// the named environment. Equivalent to ResolveEnvironment.
func LoadEnvironment(name string) (*EnvironmentConfig, error) {
	return ResolveEnvironment(name)
}

// LoadEnvironment looks up the named environment in the config.
// When name is empty, returns top-level config promoted to an EnvironmentConfig.
func (c *TentacularConfig) LoadEnvironment(name string) (*EnvironmentConfig, error) {
	if name == "" {
		return &EnvironmentConfig{
			Namespace:    c.Namespace,
			RuntimeClass: c.RuntimeClass,
		}, nil
	}
	env, ok := c.Environments[name]
	if !ok {
		return nil, fmt.Errorf("environment %q not found in config", name)
	}
	// Expand ~ in path fields
	if env.Kubeconfig != "" {
		env.Kubeconfig = expandHome(env.Kubeconfig)
	}
	if env.MCPTokenPath != "" {
		env.MCPTokenPath = expandHome(env.MCPTokenPath)
	}
	return &env, nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// ApplyConfigOverrides merges environment overrides into workflow config.
// Override values replace existing keys; new keys are added.
func ApplyConfigOverrides(wfConfig map[string]interface{}, overrides map[string]interface{}) {
	for k, v := range overrides {
		wfConfig[k] = v
	}
}
