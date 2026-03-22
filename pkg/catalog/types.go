package catalog

// CatalogIndex is the top-level catalog.yaml structure.
type CatalogIndex struct {
	Version   string          `yaml:"version"`
	Generated string          `yaml:"generated"`
	Templates []TemplateEntry `yaml:"templates"`
}

// TemplateEntry is one template in the catalog index.
// Deprecated: use pkg/scaffold.ScaffoldEntry instead.
type TemplateEntry struct {
	Name                 string   `yaml:"name"`
	DisplayName          string   `yaml:"displayName"`
	Description          string   `yaml:"description"`
	Category             string   `yaml:"category"`
	Tags                 []string `yaml:"tags"`
	Author               string   `yaml:"author"`
	MinTentacularVersion string   `yaml:"minTentacularVersion"`
	Complexity           string   `yaml:"complexity"`
	Path                 string   `yaml:"path"`
	Files                []string `yaml:"files"`
}

// ScaffoldEntry is an alias for TemplateEntry for transition compatibility.
// Deprecated: use pkg/scaffold.ScaffoldEntry directly.
type ScaffoldEntry = TemplateEntry
