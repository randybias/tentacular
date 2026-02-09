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
}

type Trigger struct {
	Type     string `yaml:"type"`
	Schedule string `yaml:"schedule,omitempty"`
	Path     string `yaml:"path,omitempty"`
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
	Timeout string `yaml:"timeout,omitempty"`
	Retries int    `yaml:"retries,omitempty"`
}
