package params

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Schema is the top-level params.schema.yaml structure.
type Schema struct {
	Parameters  map[string]ParamDef `yaml:"parameters"`
	Version     string              `yaml:"version"`
	Description string              `yaml:"description,omitempty"`
}

// ParamDef is one parameter definition in params.schema.yaml.
type ParamDef struct {
	Default     any    `yaml:"default,omitempty"`
	Example     any    `yaml:"example,omitempty"`
	Path        string `yaml:"path"`
	Type        string `yaml:"type"`
	Items       string `yaml:"items,omitempty"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// LoadSchema reads and parses a params.schema.yaml file.
func LoadSchema(path string) (*Schema, error) {
	data, err := os.ReadFile(path) //nolint:gosec // reading user-provided schema file path
	if err != nil {
		return nil, fmt.Errorf("reading params schema: %w", err)
	}
	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing params schema: %w", err)
	}
	return &s, nil
}

// LoadParamsFile reads a YAML file containing parameter name-value pairs.
func LoadParamsFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) //nolint:gosec // reading user-provided params file path
	if err != nil {
		return nil, fmt.Errorf("reading params file: %w", err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("parsing params file: %w", err)
	}
	return values, nil
}

// Validate checks that all required parameters are present in values and that
// types match. Returns a list of validation errors (empty means valid).
func Validate(schema *Schema, values map[string]any) []string {
	var errs []string
	for name, def := range schema.Parameters {
		val, ok := values[name]
		if !ok {
			if def.Required {
				errs = append(errs, fmt.Sprintf("required parameter '%s' is missing", name))
			}
			continue
		}
		if err := checkType(name, def.Type, val); err != nil {
			errs = append(errs, err.Error())
		}
	}
	return errs
}

func checkType(name, typeName string, val any) error {
	switch typeName {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("parameter '%s': expected string, got %T", name, val)
		}
	case "number":
		switch val.(type) {
		case int, int64, float64, float32:
			// ok
		default:
			return fmt.Errorf("parameter '%s': expected number, got %T", name, val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("parameter '%s': expected boolean, got %T", name, val)
		}
	case "list":
		// yaml.v3 unmarshals sequences as []any or []interface{}
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("parameter '%s': expected list, got %T", name, val)
		}
	case "map":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("parameter '%s': expected map, got %T", name, val)
		}
	default:
		return fmt.Errorf("parameter '%s': unknown type '%s'", name, typeName)
	}
	return nil
}
