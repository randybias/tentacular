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
