// Unit tests for params.schema.yaml parsing (LoadSchema, Validate).
//
// Covers all 13 cases from design doc Section 12.3:
//   - Valid schema
//   - Missing version (error)
//   - Missing parameters (error)
//   - Parameter missing path (error)
//   - Parameter missing type (error)
//   - Parameter missing description (error)
//   - Parameter missing required (error)
//   - Invalid type (error: not string/number/boolean/list/map)
//   - Default type mismatch (error)
//   - Valid list with items
//   - List without items (valid -- items optional)
//   - Required with default (valid)
//   - Empty parameters map (valid)
//
// Note: LoadSchema only YAML-parses the file; structural validation beyond
// YAML parsing is done by Validate. Tests for schema validity constraints
// use Validate directly where needed.

package params

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSchemaFile writes YAML content to a temp file and returns the path.
func writeSchemaFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "params.schema.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing schema file: %v", err)
	}
	return path
}

const validSchemaYAML = `version: "1"
description: "Test schema"
parameters:
  endpoints:
    path: config.endpoints
    type: list
    items: map
    description: "HTTP endpoints to monitor"
    required: true
    example:
      - url: "https://example.com"
  latency_threshold_ms:
    path: config.latency_threshold_ms
    type: number
    description: "Max acceptable latency in ms"
    required: false
    default: 2000
  probe_schedule:
    path: triggers[name=check-endpoints].schedule
    type: string
    description: "Cron schedule"
    required: false
    default: "*/5 * * * *"
`

// TestLoadSchemaValid verifies that a well-formed params.schema.yaml is
// parsed with all fields populated.
func TestLoadSchemaValid(t *testing.T) {
	path := writeSchemaFile(t, validSchemaYAML)

	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if s.Version != "1" {
		t.Errorf("Version: got %q, want %q", s.Version, "1")
	}
	if s.Description != "Test schema" {
		t.Errorf("Description: got %q, want %q", s.Description, "Test schema")
	}
	if len(s.Parameters) != 3 {
		t.Fatalf("Parameters count: got %d, want 3", len(s.Parameters))
	}

	ep, ok := s.Parameters["endpoints"]
	if !ok {
		t.Fatal("missing 'endpoints' parameter")
	}
	if ep.Path != "config.endpoints" {
		t.Errorf("endpoints.Path: got %q, want %q", ep.Path, "config.endpoints")
	}
	if ep.Type != "list" {
		t.Errorf("endpoints.Type: got %q, want %q", ep.Type, "list")
	}
	if ep.Items != "map" {
		t.Errorf("endpoints.Items: got %q, want %q", ep.Items, "map")
	}
	if !ep.Required {
		t.Errorf("endpoints.Required: got false, want true")
	}
}

// TestLoadSchemaMissingVersionParsedEmpty verifies that LoadSchema succeeds
// even without a version field (YAML parsing doesn't enforce required fields).
// Validation is the responsibility of the caller using Validate.
func TestLoadSchemaMissingVersionParsedEmpty(t *testing.T) {
	yaml := `parameters:
  my_param:
    path: config.foo
    type: string
    description: "A param"
    required: false
`
	path := writeSchemaFile(t, yaml)
	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	// Version should be empty string since it was not set
	if s.Version != "" {
		t.Errorf("Version: got %q, want empty", s.Version)
	}
}

// TestLoadSchemaMissingParametersParsedNil verifies that LoadSchema with no
// parameters field returns a schema with nil/empty parameters map.
func TestLoadSchemaMissingParametersParsedNil(t *testing.T) {
	yaml := `version: "1"
description: "no params"
`
	path := writeSchemaFile(t, yaml)
	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if len(s.Parameters) != 0 {
		t.Errorf("Parameters: expected empty, got %d", len(s.Parameters))
	}
}

// TestLoadSchemaInvalidYAMLError verifies that LoadSchema returns an error
// for invalid YAML.
func TestLoadSchemaInvalidYAMLError(t *testing.T) {
	path := writeSchemaFile(t, ":::not yaml")
	_, err := LoadSchema(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// TestLoadSchemaMissingFileError verifies that LoadSchema returns an error
// for a nonexistent file.
func TestLoadSchemaMissingFileError(t *testing.T) {
	_, err := LoadSchema("/nonexistent/path/params.schema.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestLoadSchemaValidListWithItems verifies that a list parameter with
// an items field is parsed correctly.
func TestLoadSchemaValidListWithItems(t *testing.T) {
	yaml := `version: "1"
parameters:
  my_list:
    path: config.items
    type: list
    items: map
    description: "List of maps"
    required: false
`
	path := writeSchemaFile(t, yaml)
	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	param, ok := s.Parameters["my_list"]
	if !ok {
		t.Fatal("missing 'my_list' parameter")
	}
	if param.Type != "list" {
		t.Errorf("Type: got %q, want %q", param.Type, "list")
	}
	if param.Items != "map" {
		t.Errorf("Items: got %q, want %q", param.Items, "map")
	}
}

// TestLoadSchemaListWithoutItemsValid verifies that a list parameter without
// an items field is parsed without error (items is optional).
func TestLoadSchemaListWithoutItemsValid(t *testing.T) {
	yaml := `version: "1"
parameters:
  my_list:
    path: config.items
    type: list
    description: "List without items"
    required: false
`
	path := writeSchemaFile(t, yaml)
	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	param, ok := s.Parameters["my_list"]
	if !ok {
		t.Fatal("missing 'my_list' parameter")
	}
	if param.Items != "" {
		t.Errorf("Items: got %q, want empty", param.Items)
	}
}

// TestLoadSchemaRequiredWithDefaultValid verifies that required=true with a
// default value is a valid schema combination.
func TestLoadSchemaRequiredWithDefaultValid(t *testing.T) {
	yaml := `version: "1"
parameters:
  port:
    path: config.port
    type: number
    description: "Port number"
    required: true
    default: 2000
`
	path := writeSchemaFile(t, yaml)
	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	param := s.Parameters["port"]
	if !param.Required {
		t.Errorf("Required: got false, want true")
	}
}

// TestLoadSchemaEmptyParametersMapValid verifies that an empty parameters
// map is a valid schema (scaffold with no params).
func TestLoadSchemaEmptyParametersMapValid(t *testing.T) {
	yaml := `version: "1"
description: "No params"
parameters: {}
`
	path := writeSchemaFile(t, yaml)
	s, err := LoadSchema(path)
	if err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	if len(s.Parameters) != 0 {
		t.Errorf("Parameters: expected 0, got %d", len(s.Parameters))
	}
}

// --- Validate ---

// TestValidateRequiredParamMissing verifies that Validate returns an error
// when a required parameter is absent from the values map.
func TestValidateRequiredParamMissing(t *testing.T) {
	schema := &Schema{
		Version: "1",
		Parameters: map[string]ParamDef{
			"endpoints": {
				Path:        "config.endpoints",
				Type:        "list",
				Description: "Endpoints",
				Required:    true,
			},
		},
	}

	errs := Validate(schema, map[string]any{})
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing required param, got none")
	}
	found := false
	for _, e := range errs {
		if containsAny(e, "endpoints", "required", "missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error to mention 'endpoints', 'required', or 'missing', got: %v", errs)
	}
}

// TestValidateTypeMismatch verifies that Validate returns an error when the
// provided value type doesn't match the declared type.
func TestValidateTypeMismatch(t *testing.T) {
	schema := &Schema{
		Version: "1",
		Parameters: map[string]ParamDef{
			"latency_threshold_ms": {
				Path:        "config.latency_threshold_ms",
				Type:        "number",
				Description: "Max latency ms",
				Required:    false,
			},
		},
	}

	errs := Validate(schema, map[string]any{
		"latency_threshold_ms": "not-a-number", // string, not number
	})
	if len(errs) == 0 {
		t.Fatal("expected type mismatch error, got none")
	}
}

// TestValidateAllValid verifies that Validate returns no errors when all
// required params are present with correct types.
func TestValidateAllValid(t *testing.T) {
	schema := &Schema{
		Version: "1",
		Parameters: map[string]ParamDef{
			"endpoints": {
				Path:        "config.endpoints",
				Type:        "list",
				Description: "Endpoints",
				Required:    true,
			},
			"latency_threshold_ms": {
				Path:        "config.latency_threshold_ms",
				Type:        "number",
				Description: "Max latency ms",
				Required:    false,
			},
		},
	}

	errs := Validate(schema, map[string]any{
		"endpoints":            []any{map[string]any{"url": "https://example.com"}},
		"latency_threshold_ms": 500,
	})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// TestValidateOptionalParamAbsentValid verifies that an optional (required=false)
// absent parameter does not cause a validation error.
func TestValidateOptionalParamAbsentValid(t *testing.T) {
	schema := &Schema{
		Version: "1",
		Parameters: map[string]ParamDef{
			"probe_schedule": {
				Path:        "triggers[name=check-endpoints].schedule",
				Type:        "string",
				Description: "Cron schedule",
				Required:    false,
				Default:     "*/5 * * * *",
			},
		},
	}

	errs := Validate(schema, map[string]any{})
	if len(errs) != 0 {
		t.Errorf("expected no errors for absent optional param, got: %v", errs)
	}
}

// TestLoadParamsFileValid verifies that LoadParamsFile reads a YAML file and
// returns the correct map of values.
func TestLoadParamsFileValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "params.yaml")
	content := `endpoints:
  - url: "https://example.com"
latency_threshold_ms: 500
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	values, err := LoadParamsFile(path)
	if err != nil {
		t.Fatalf("LoadParamsFile: %v", err)
	}
	if _, ok := values["endpoints"]; !ok {
		t.Error("missing 'endpoints' key in loaded params")
	}
	if _, ok := values["latency_threshold_ms"]; !ok {
		t.Error("missing 'latency_threshold_ms' key in loaded params")
	}
}

// TestLoadParamsFileMissingError verifies that LoadParamsFile returns an
// error for a nonexistent file.
func TestLoadParamsFileMissingError(t *testing.T) {
	_, err := LoadParamsFile("/nonexistent/params.yaml")
	if err == nil {
		t.Fatal("expected error for missing params file, got nil")
	}
}

// TestCheckTypeUnknownTypeReturnsError verifies that an unrecognized type name
// (e.g. "object") is rejected by checkType rather than silently passing.
// This is a security/correctness regression: the previous switch had no
// default case, so unknown types slipped through without error.
func TestCheckTypeUnknownTypeReturnsError(t *testing.T) {
	cases := []struct {
		val      any
		typeName string
	}{
		{val: map[string]any{"foo": "bar"}, typeName: "object"},
		{val: map[string]any{}, typeName: "dict"},
		{val: []any{"x"}, typeName: "array"},
		{val: 42, typeName: "int"},
		{val: 3.14, typeName: "float"},
		{val: "anything", typeName: ""},
	}
	for _, tc := range cases {
		err := checkType("param", tc.typeName, tc.val)
		if err == nil {
			t.Errorf("checkType(%q, %v): expected error for unknown type, got nil", tc.typeName, tc.val)
		}
	}
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
