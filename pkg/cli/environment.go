package cli

import (
	"fmt"
	"os"
)

// EnvironmentConfig holds per-environment overrides.
type EnvironmentConfig struct {
	Context         string                 `yaml:"context,omitempty"`
	Namespace       string                 `yaml:"namespace,omitempty"`
	Image           string                 `yaml:"image,omitempty"`
	RuntimeClass    string                 `yaml:"runtime_class,omitempty"`
	ConfigOverrides map[string]interface{} `yaml:"config_overrides,omitempty"`
	SecretsSource   string                 `yaml:"secrets_source,omitempty"`
}

// ResolveEnvironment loads the merged config and returns the named environment.
// Resolution cascade: explicit envName > TENTACULAR_ENV env var > "" (top-level defaults).
// When envName is empty (and TENTACULAR_ENV is unset), returns top-level config
// promoted to an EnvironmentConfig.
func ResolveEnvironment(envName string) (*EnvironmentConfig, error) {
	if envName == "" {
		envName = os.Getenv("TENTACULAR_ENV")
	}
	cfg := LoadConfig()
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
	return &env, nil
}

// ApplyConfigOverrides merges environment overrides into workflow config.
// Override values replace existing keys; new keys are added.
func ApplyConfigOverrides(wfConfig map[string]interface{}, overrides map[string]interface{}) {
	for k, v := range overrides {
		wfConfig[k] = v
	}
}
