package cli

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/catalog"
	"github.com/randybias/tentacular/pkg/scaffold"
)

// ModuleProxyConfig configures the in-cluster esm.sh module proxy for jsr/npm deps.
type ModuleProxyConfig struct {
	Namespace string `yaml:"namespace,omitempty"` // default: tentacular-support
	Image     string `yaml:"image,omitempty"`     // default: ghcr.io/esm-dev/esm.sh:v135
	Storage   string `yaml:"storage,omitempty"`   // "emptydir" (default) or "pvc"
	PVCSize   string `yaml:"pvcSize,omitempty"`   // default: 5Gi (only when storage: pvc)
	Enabled   bool   `yaml:"enabled,omitempty"`
}

// MCPConfig holds MCP server connection settings.
type MCPConfig struct {
	Endpoint string `yaml:"endpoint,omitempty"` // e.g. http://tentacular-mcp.tentacular-system.svc.cluster.local:8080
}

// GitStateConfig configures the git-backed state repository for tentacle source and secrets.
type GitStateConfig struct {
	RepoPath string `yaml:"repo_path,omitempty"` // path to the git-state repo root
	Enabled  bool   `yaml:"enabled,omitempty"`   // when true, enclave is required for scaffold init and deploy gates are enforced
}

// TentacularConfig holds default configuration values.
type TentacularConfig struct {
	Clusters       map[string]EnvironmentConfig `yaml:"clusters,omitempty"`
	MCP            MCPConfig                    `yaml:"mcp,omitempty"`
	Catalog        catalog.CatalogConfig        `yaml:"catalog,omitempty"`
	Scaffold       scaffold.ClientConfig        `yaml:"scaffold,omitempty"`
	GitState       GitStateConfig               `yaml:"git_state,omitempty"`
	Workspace      string                       `yaml:"workspace,omitempty"`
	Registry       string                       `yaml:"registry,omitempty"`
	Namespace      string                       `yaml:"namespace,omitempty"`
	RuntimeClass   string                       `yaml:"runtime_class,omitempty"`
	DefaultCluster string                       `yaml:"default_cluster,omitempty"`
	ModuleProxy    ModuleProxyConfig            `yaml:"moduleProxy,omitempty"`
}

// LoadConfig returns the user-level config from ~/.tentacular/config.yaml.
// Missing files are silently ignored.
func LoadConfig() TentacularConfig {
	cfg := TentacularConfig{}

	// Load user-level (~/.tentacular/config.yaml)
	home, _ := os.UserHomeDir()
	if home != "" {
		userPath := filepath.Join(home, ".tentacular", "config.yaml")
		if data, err := os.ReadFile(userPath); err == nil { //nolint:gosec // reading user config file
			_ = yaml.Unmarshal(data, &cfg)
		}
	}

	return cfg
}
