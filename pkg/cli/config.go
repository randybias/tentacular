package cli

import (
	"os"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/catalog"
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
	Workspace    string                       `yaml:"workspace,omitempty"` // workspace root dir (default: ~/tentacles)
	Registry     string                       `yaml:"registry,omitempty"`
	Namespace    string                       `yaml:"namespace,omitempty"`
	RuntimeClass string                       `yaml:"runtime_class,omitempty"`
	DefaultEnv   string                       `yaml:"default_env,omitempty"`
	Environments map[string]EnvironmentConfig `yaml:"environments,omitempty"`
	ModuleProxy  ModuleProxyConfig            `yaml:"moduleProxy,omitempty"`
	MCP          MCPConfig                    `yaml:"mcp,omitempty"`
	Catalog      catalog.CatalogConfig        `yaml:"catalog,omitempty"`
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
			_ = yaml.Unmarshal(data, &cfg)
		}
	}

	// 2. Load project-level (.tentacular/config.yaml) — overrides user
	projPath := filepath.Join(".tentacular", "config.yaml")
	if data, err := os.ReadFile(projPath); err == nil {
		var proj TentacularConfig
		_ = yaml.Unmarshal(data, &proj)
		mergeConfig(&cfg, &proj)
	}

	return cfg
}

func mergeConfig(base, override *TentacularConfig) {
	if override.Workspace != "" {
		base.Workspace = override.Workspace
	}
	if override.Registry != "" {
		base.Registry = override.Registry
	}
	if override.Namespace != "" {
		base.Namespace = override.Namespace
	}
	if override.RuntimeClass != "" {
		base.RuntimeClass = override.RuntimeClass
	}
	if override.DefaultEnv != "" {
		base.DefaultEnv = override.DefaultEnv
	}
	if len(override.Environments) > 0 {
		if base.Environments == nil {
			base.Environments = make(map[string]EnvironmentConfig)
		}
		for k, v := range override.Environments {
			existing, ok := base.Environments[k]
			if !ok {
				base.Environments[k] = v
				continue
			}
			mergeEnvConfig(&existing, &v)
			base.Environments[k] = existing
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
	if override.Catalog.URL != "" {
		base.Catalog.URL = override.Catalog.URL
	}
	if override.Catalog.CacheTTL != "" {
		base.Catalog.CacheTTL = override.Catalog.CacheTTL
	}
}

// mergeEnvConfig merges individual fields of an EnvironmentConfig override
// into a base, preserving base fields that the override does not set.
func mergeEnvConfig(base, override *EnvironmentConfig) {
	if override.Kubeconfig != "" {
		base.Kubeconfig = override.Kubeconfig
	}
	if override.Context != "" {
		base.Context = override.Context
	}
	if override.Namespace != "" {
		base.Namespace = override.Namespace
	}
	if override.Image != "" {
		base.Image = override.Image
	}
	if override.RuntimeClass != "" {
		base.RuntimeClass = override.RuntimeClass
	}
	if len(override.ConfigOverrides) > 0 {
		if base.ConfigOverrides == nil {
			base.ConfigOverrides = make(map[string]interface{})
		}
		for k, v := range override.ConfigOverrides {
			base.ConfigOverrides[k] = v
		}
	}
	if override.SecretsSource != "" {
		base.SecretsSource = override.SecretsSource
	}
	if override.Enforcement != "" {
		base.Enforcement = override.Enforcement
	}
	if override.MCPEndpoint != "" {
		base.MCPEndpoint = override.MCPEndpoint
	}
	if override.MCPTokenPath != "" {
		base.MCPTokenPath = override.MCPTokenPath
	}
	if override.OIDCIssuer != "" {
		base.OIDCIssuer = override.OIDCIssuer
	}
	if override.OIDCClientID != "" {
		base.OIDCClientID = override.OIDCClientID
	}
	if override.OIDCClientSecret != "" {
		base.OIDCClientSecret = override.OIDCClientSecret
	}
}
