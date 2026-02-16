package cli

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TentacularConfig holds default configuration values.
type TentacularConfig struct {
	Registry     string                          `yaml:"registry,omitempty"`
	Namespace    string                          `yaml:"namespace,omitempty"`
	RuntimeClass string                          `yaml:"runtime_class,omitempty"`
	Environments map[string]EnvironmentConfig    `yaml:"environments,omitempty"`
}

// LoadConfig returns merged config: project > user > defaults.
// Missing files are silently ignored.
func LoadConfig() TentacularConfig {
	cfg := TentacularConfig{}

	// 1. Load user-level (~/.tentacular/config.yaml)
	home, _ := os.UserHomeDir()
	if home != "" {
		userPath := filepath.Join(home, ".tentacular", "config.yaml")
		if data, err := os.ReadFile(userPath); err == nil {
			yaml.Unmarshal(data, &cfg)
		}
	}

	// 2. Load project-level (.tentacular/config.yaml) — overrides user
	projPath := filepath.Join(".tentacular", "config.yaml")
	if data, err := os.ReadFile(projPath); err == nil {
		var proj TentacularConfig
		yaml.Unmarshal(data, &proj)
		mergeConfig(&cfg, &proj)
	}

	return cfg
}

func mergeConfig(base, override *TentacularConfig) {
	if override.Registry != "" {
		base.Registry = override.Registry
	}
	if override.Namespace != "" {
		base.Namespace = override.Namespace
	}
	if override.RuntimeClass != "" {
		base.RuntimeClass = override.RuntimeClass
	}
	if len(override.Environments) > 0 {
		if base.Environments == nil {
			base.Environments = make(map[string]EnvironmentConfig)
		}
		for k, v := range override.Environments {
			base.Environments[k] = v
		}
	}
}
