package scaffold

// ScaffoldIndex is the top-level scaffolds-index.yaml structure.
type ScaffoldIndex struct { //nolint:govet // field order is for YAML readability
	Scaffolds []ScaffoldEntry `yaml:"scaffolds"`
	Version   string          `yaml:"version"`
	Generated string          `yaml:"generated"`
}

// ScaffoldEntry is one scaffold in the index.
type ScaffoldEntry struct { //nolint:govet // field order matches scaffold.yaml key order for readability
	Tags                 []string `yaml:"tags"`
	Files                []string `yaml:"files"`
	Name                 string   `yaml:"name"`
	DisplayName          string   `yaml:"displayName"`
	Description          string   `yaml:"description"`
	Category             string   `yaml:"category"`
	Author               string   `yaml:"author"`
	MinTentacularVersion string   `yaml:"minTentacularVersion"`
	Complexity           string   `yaml:"complexity"`
	Version              string   `yaml:"version"`
	Path                 string   `yaml:"path"`
	Source               string   `yaml:"-"` // "private" or "public", not stored in YAML
}

// TentacleYAML is the tentacle.yaml identity/provenance file written in every tentacle directory.
type TentacleYAML struct {
	Scaffold *TentacleScaffold `yaml:"scaffold,omitempty"`
	Name     string            `yaml:"name"`
	Created  string            `yaml:"created"`
}

// TentacleScaffold records the scaffold provenance inside tentacle.yaml.
type TentacleScaffold struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	Source   string `yaml:"source"`   // "public" or "private"
	Modified bool   `yaml:"modified"` // informational
}
