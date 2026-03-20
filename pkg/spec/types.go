package spec

// Workflow is the top-level v2 workflow specification.
type Workflow struct {
	Metadata    *WorkflowMetadata   `yaml:"metadata,omitempty"`
	Contract    *Contract           `yaml:"contract,omitempty"`
	Nodes       map[string]NodeSpec `yaml:"nodes"`
	Name        string              `yaml:"name"`
	Version     string              `yaml:"version"`
	Description string              `yaml:"description"`
	Deployment  DeploymentConfig    `yaml:"deployment,omitempty"`
	Config      WorkflowConfig      `yaml:"config"`
	Triggers    []Trigger           `yaml:"triggers"`
	Edges       []Edge              `yaml:"edges"`
}

// WorkflowMetadata provides optional descriptive metadata for MCP reporting.
type WorkflowMetadata struct {
	Group       string   `yaml:"group,omitempty"`
	Environment string   `yaml:"environment,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// DeploymentConfig holds deployment-specific settings embedded in workflow.yaml.
type DeploymentConfig struct {
	Namespace string `yaml:"namespace,omitempty"`
}

type Trigger struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name,omitempty"`
	Schedule string `yaml:"schedule,omitempty"`
	Path     string `yaml:"path,omitempty"`
	Subject  string `yaml:"subject,omitempty"`
	// webhook-specific fields
	Provider string   `yaml:"provider,omitempty"` // e.g. "github"
	Event    string   `yaml:"event,omitempty"`    // e.g. "pull_request"
	Actions  []string `yaml:"actions,omitempty"`  // e.g. ["opened", "synchronize"]
}

type NodeSpec struct {
	Capabilities map[string]string `yaml:"capabilities,omitempty"`
	Path         string            `yaml:"path"`
}

type Edge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type WorkflowConfig struct {
	Extras  map[string]any `yaml:",inline"`
	Timeout string         `yaml:"timeout,omitempty"`
	Retries int            `yaml:"retries,omitempty"`
}

// ToMap returns a flat map merging typed fields and extras.
// Zero-valued typed fields are omitted.
func (c WorkflowConfig) ToMap() map[string]any {
	m := make(map[string]any)
	for k, v := range c.Extras {
		m[k] = v
	}
	if c.Timeout != "" {
		m["timeout"] = c.Timeout
	}
	if c.Retries != 0 {
		m["retries"] = c.Retries
	}
	return m
}

// Contract defines external dependencies and their network requirements.
type Contract struct {
	Dependencies  map[string]Dependency `yaml:"dependencies"`
	NetworkPolicy *NetworkPolicyConfig  `yaml:"networkPolicy,omitempty"`
	Extensions    map[string]any        `yaml:",inline"`
	Version       string                `yaml:"version"`
}

// Dependency declares a single external service dependency.
type Dependency struct {
	Auth       *DependencyAuth `yaml:"auth,omitempty"`
	Extensions map[string]any  `yaml:",inline"`
	Host       string          `yaml:"host,omitempty"`
	Protocol   string          `yaml:"protocol"`
	Type       string          `yaml:"type,omitempty"`
	CIDR       string          `yaml:"cidr,omitempty"`
	Version    string          `yaml:"version,omitempty"`
	Database   string          `yaml:"database,omitempty"`
	User       string          `yaml:"user,omitempty"`
	Subject    string          `yaml:"subject,omitempty"`
	Container  string          `yaml:"container,omitempty"`
	DynPorts   []string        `yaml:"dynPorts,omitempty"`
	Port       int             `yaml:"port,omitempty"`
}

// DependencyAuth specifies authentication for a dependency.
type DependencyAuth struct {
	Type   string `yaml:"type"`   // any string identifying the auth mechanism
	Secret string `yaml:"secret"` // Must be in "service.key" format
}

// NetworkPolicyConfig allows manual egress CIDR configuration.
type NetworkPolicyConfig struct {
	AdditionalEgress []EgressOverride `yaml:"additionalEgress,omitempty"`
}

// EgressOverride adds a CIDR-based egress rule.
type EgressOverride struct {
	ToCIDR string   `yaml:"toCIDR"`
	Reason string   `yaml:"reason,omitempty"`
	Ports  []string `yaml:"ports,omitempty"`
}
