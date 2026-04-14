package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvironmentConfig holds per-environment overrides.
type EnvironmentConfig struct {
	Context         string         `yaml:"context,omitempty"`
	Namespace       string         `yaml:"namespace,omitempty"`
	Image           string         `yaml:"image,omitempty"`
	RuntimeClass    string         `yaml:"runtime_class,omitempty"`
	ConfigOverrides map[string]any `yaml:"config_overrides,omitempty"`
	SecretsSource   string         `yaml:"secrets_source,omitempty"`
	Enforcement     string         `yaml:"enforcement,omitempty"` // "strict" (default) or "audit"
	MCPEndpoint     string         `yaml:"mcp_endpoint,omitempty"`

	// OIDC fields (optional). When present, `tntc login` uses device authorization flow.
	OIDCIssuer       string `yaml:"oidc_issuer,omitempty"`
	OIDCClientID     string `yaml:"oidc_client_id,omitempty"`
	OIDCClientSecret string `yaml:"oidc_client_secret,omitempty"`
}

// ResolveEnvironment loads the merged config and returns the named environment.
// Resolution cascade: explicit clusterName > TENTACULAR_CLUSTER env var > default_cluster config > "" (top-level defaults).
// When clusterName is empty (and TENTACULAR_CLUSTER is unset and default_cluster is not set),
// returns top-level config promoted to an EnvironmentConfig.
func ResolveEnvironment(clusterName string) (*EnvironmentConfig, error) {
	if clusterName == "" {
		clusterName = os.Getenv("TENTACULAR_CLUSTER")
	}
	cfg := LoadConfig()
	if clusterName == "" {
		clusterName = cfg.DefaultCluster
	}
	return cfg.LoadEnvironment(clusterName)
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
	env, ok := c.Clusters[name]
	if !ok {
		return nil, fmt.Errorf("environment %q not found in config", name)
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
func ApplyConfigOverrides(wfConfig, overrides map[string]any) {
	for k, v := range overrides {
		wfConfig[k] = v
	}
}
