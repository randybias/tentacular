package spec

// Workflow is the top-level v2 workflow specification.
type Workflow struct {
	Name        string              `yaml:"name"`
	Version     string              `yaml:"version"`
	Description string              `yaml:"description"`
	Triggers    []Trigger           `yaml:"triggers"`
	Nodes       map[string]NodeSpec `yaml:"nodes"`
	Edges       []Edge              `yaml:"edges"`
	Config      WorkflowConfig      `yaml:"config"`
	Deployment  DeploymentConfig    `yaml:"deployment,omitempty"`
	Contract    *Contract           `yaml:"contract,omitempty"`
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
}

type NodeSpec struct {
	Path         string            `yaml:"path"`
	Capabilities map[string]string `yaml:"capabilities,omitempty"`
}

type Edge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type WorkflowConfig struct {
	Timeout string                 `yaml:"timeout,omitempty"`
	Retries int                    `yaml:"retries,omitempty"`
	Extras  map[string]interface{} `yaml:",inline"`
}

// ToMap returns a flat map merging typed fields and extras.
// Zero-valued typed fields are omitted.
func (c WorkflowConfig) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
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
	Version       string                 `yaml:"version"`
	Dependencies  map[string]Dependency  `yaml:"dependencies"`
	NetworkPolicy *NetworkPolicyConfig   `yaml:"networkPolicy,omitempty"`
	Extensions    map[string]interface{} `yaml:",inline"`
}

// Dependency declares a single external service dependency.
type Dependency struct {
	Protocol string          `yaml:"protocol"`
	Type     string          `yaml:"type,omitempty"`     // "dynamic-target" for wildcard deps
	Auth     *DependencyAuth `yaml:"auth,omitempty"`
	CIDR     string          `yaml:"cidr,omitempty"`     // required when type=dynamic-target
	DynPorts []string        `yaml:"dynPorts,omitempty"` // required when type=dynamic-target, e.g. ["443/TCP"]
	// Protocol-specific fields
	Host       string                 `yaml:"host,omitempty"`      // https, postgresql, nats, blob
	Port       int                    `yaml:"port,omitempty"`      // https, postgresql, nats
	Database   string                 `yaml:"database,omitempty"`  // postgresql
	User       string                 `yaml:"user,omitempty"`      // postgresql
	Subject    string                 `yaml:"subject,omitempty"`   // nats
	Container  string                 `yaml:"container,omitempty"` // blob
	Extensions map[string]interface{} `yaml:",inline"`
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
	Ports  []string `yaml:"ports,omitempty"`
	Reason string   `yaml:"reason,omitempty"` // human-readable justification
}
