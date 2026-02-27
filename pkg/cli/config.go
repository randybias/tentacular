package cli

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ModuleProxyConfig configures the in-cluster esm.sh module proxy for jsr/npm deps.
type ModuleProxyConfig struct {
	Enabled   bool   `yaml:"enabled,omitempty"`
	Namespace string `yaml:"namespace,omitempty"` // default: tentacular-support
	Image     string `yaml:"image,omitempty"`     // default: ghcr.io/esm-dev/esm.sh:v135
	Storage   string `yaml:"storage,omitempty"`   // "emptydir" (default) or "pvc"
	PVCSize   string `yaml:"pvcSize,omitempty"`   // default: 5Gi (only when storage: pvc)
}

// MCPConfig holds MCP server connection settings.
type MCPConfig struct {
	Endpoint  string `yaml:"endpoint,omitempty"`   // e.g. http://tentacular-mcp.tentacular-system.svc.cluster.local:8080
	TokenPath string `yaml:"token_path,omitempty"` // path to bearer token file
}

// TentacularConfig holds default configuration values.
type TentacularConfig struct {
	Registry     string                       `yaml:"registry,omitempty"`
	Namespace    string                       `yaml:"namespace,omitempty"`
	RuntimeClass string                       `yaml:"runtime_class,omitempty"`
	Environments map[string]EnvironmentConfig `yaml:"environments,omitempty"`
	ModuleProxy  ModuleProxyConfig            `yaml:"moduleProxy,omitempty"`
	MCP          MCPConfig                    `yaml:"mcp,omitempty"`
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
	if override.ModuleProxy.Enabled {
		base.ModuleProxy.Enabled = true
	}
	if override.ModuleProxy.Namespace != "" {
		base.ModuleProxy.Namespace = override.ModuleProxy.Namespace
	}
	if override.ModuleProxy.Image != "" {
		base.ModuleProxy.Image = override.ModuleProxy.Image
	}
	if override.ModuleProxy.Storage != "" {
		base.ModuleProxy.Storage = override.ModuleProxy.Storage
	}
	if override.ModuleProxy.PVCSize != "" {
		base.ModuleProxy.PVCSize = override.ModuleProxy.PVCSize
	}
	if override.MCP.Endpoint != "" {
		base.MCP.Endpoint = override.MCP.Endpoint
	}
	if override.MCP.TokenPath != "" {
		base.MCP.TokenPath = override.MCP.TokenPath
	}
}
