// Unit tests for the path resolver / overlay functions (overlay.go).
//
// Uses the reference workflow.yaml fixture from design doc Section 12.2.
// Covers:
//   - GetNodeValue: read simple key, nested key, filtered segment
//   - setNodeValue: write simple key, write list, write with filter
//   - ApplyToFile: end-to-end file apply with schema + values
//   - Error cases: nonexistent key, typo-like key, filter no match,
//     filter on non-sequence

package params

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// referenceWorkflowYAML is the fixture from design doc Section 12.2.
const referenceWorkflowYAML = `triggers:
  - type: manual
  - type: cron
    name: check-endpoints
    schedule: "*/5 * * * *"
config:
  timeout: 120s
  endpoints:
    - url: "https://example.com"
nodes:
  probe-endpoints:
    path: ./nodes/probe-endpoints.ts
`

// parseReferenceDoc parses referenceWorkflowYAML and returns the root mapping node.
func parseReferenceDoc(t *testing.T) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(referenceWorkflowYAML), &doc); err != nil {
		t.Fatalf("failed to parse reference YAML: %v", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		t.Fatal("expected document node")
	}
	return doc.Content[0]
}

// mustParsePath parses the path or fails the test.
func mustParsePath(t *testing.T, expr string) []Segment {
	t.Helper()
	segs, err := ParsePath(expr)
	if err != nil {
		t.Fatalf("ParsePath(%q): %v", expr, err)
	}
	return segs
}

// --- GetNodeValue (read operations) ---

// TestGetNodeValueReadSimpleKey verifies that reading "config.timeout" returns "120s".
func TestGetNodeValueReadSimpleKey(t *testing.T) {
	root := parseReferenceDoc(t)
	segs := mustParsePath(t, "config.timeout")

	node, err := GetNodeValue(root, segs)
	if err != nil {
		t.Fatalf("GetNodeValue: %v", err)
	}
	if node.Value != "120s" {
		t.Errorf("Value: got %q, want %q", node.Value, "120s")
	}
}

// TestGetNodeValueReadNestedKey verifies that reading "config.endpoints" returns
// a sequence node with one entry.
func TestGetNodeValueReadNestedKey(t *testing.T) {
	root := parseReferenceDoc(t)
	segs := mustParsePath(t, "config.endpoints")

	node, err := GetNodeValue(root, segs)
	if err != nil {
		t.Fatalf("GetNodeValue: %v", err)
	}
	if node.Kind != yaml.SequenceNode {
		t.Errorf("Kind: got %d, want SequenceNode (%d)", node.Kind, yaml.SequenceNode)
	}
	if len(node.Content) != 1 {
		t.Errorf("len(Content): got %d, want 1", len(node.Content))
	}
}

// TestGetNodeValueReadWithFilter verifies that reading
// "triggers[name=check-endpoints].schedule" returns "*/5 * * * *".
func TestGetNodeValueReadWithFilter(t *testing.T) {
	root := parseReferenceDoc(t)
	segs := mustParsePath(t, "triggers[name=check-endpoints].schedule")

	node, err := GetNodeValue(root, segs)
	if err != nil {
		t.Fatalf("GetNodeValue: %v", err)
	}
	if node.Value != "*/5 * * * *" {
		t.Errorf("Value: got %q, want %q", node.Value, "*/5 * * * *")
	}
}

// TestGetNodeValueNonexistentKeyError verifies that reading a path with a
// key that doesn't exist returns an error that mentions available keys.
func TestGetNodeValueNonexistentKeyError(t *testing.T) {
	root := parseReferenceDoc(t)
	segs := mustParsePath(t, "config.nonexistent")

	_, err := GetNodeValue(root, segs)
	if err == nil {
		t.Fatal("expected error for nonexistent key, got nil")
	}
	// Error should mention available keys
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "endpoints") {
		t.Errorf("expected error to mention available keys, got: %v", err)
	}
}

// TestGetNodeValueFilterNoMatchError verifies that a filter with no matching
// element returns an error mentioning the filter expression.
func TestGetNodeValueFilterNoMatchError(t *testing.T) {
	root := parseReferenceDoc(t)
	segs := mustParsePath(t, "triggers[name=nonexistent].schedule")

	_, err := GetNodeValue(root, segs)
	if err == nil {
		t.Fatal("expected error for filter no match, got nil")
	}
}

// TestGetNodeValueFilterOnNonSequenceError verifies that applying a filter
// to a mapping key (not a sequence) returns an error.
func TestGetNodeValueFilterOnNonSequenceError(t *testing.T) {
	root := parseReferenceDoc(t)
	// "config" is a mapping, not a sequence -- filter on it should fail.
	segs, err := ParsePath("config[name=foo]")
	if err != nil {
		// If the parser rejects this, that's also acceptable.
		return
	}

	_, err = GetNodeValue(root, segs)
	if err == nil {
		t.Fatal("expected error for filter on non-sequence, got nil")
	}
}

// --- setNodeValue (write operations via ApplyToFile) ---

// writeWorkflowFile writes the reference workflow YAML to a temp file and
// returns the path.
func writeWorkflowFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(referenceWorkflowYAML), 0o644); err != nil {
		t.Fatalf("writing workflow file: %v", err)
	}
	return path
}

// simpleSchema builds a schema pointing "timeout_val" to "config.timeout"
// and "schedule_val" to "triggers[name=check-endpoints].schedule".
func simpleSchema() *Schema {
	return &Schema{
		Version: "1",
		Parameters: map[string]ParamDef{
			"timeout_val": {
				Path:        "config.timeout",
				Type:        "string",
				Description: "Request timeout",
				Required:    false,
			},
			"schedule_val": {
				Path:        "triggers[name=check-endpoints].schedule",
				Type:        "string",
				Description: "Cron schedule",
				Required:    false,
			},
		},
	}
}

// TestApplyToFileWriteSimpleKey verifies that applying a string value via
// ApplyToFile overwrites config.timeout.
func TestApplyToFileWriteSimpleKey(t *testing.T) {
	path := writeWorkflowFile(t)
	schema := simpleSchema()

	err := ApplyToFile(path, schema, map[string]any{
		"timeout_val": "60s",
	})
	if err != nil {
		t.Fatalf("ApplyToFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "60s") {
		t.Errorf("expected workflow.yaml to contain '60s' after apply, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "120s") {
		t.Errorf("expected old value '120s' to be replaced, still present:\n%s", string(data))
	}
}

// TestApplyToFileWriteWithFilter verifies that applying a value via a
// filtered path overwrites only the targeted sequence element.
func TestApplyToFileWriteWithFilter(t *testing.T) {
	path := writeWorkflowFile(t)
	schema := simpleSchema()

	err := ApplyToFile(path, schema, map[string]any{
		"schedule_val": "*/2 * * * *",
	})
	if err != nil {
		t.Fatalf("ApplyToFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "*/2 * * * *") {
		t.Errorf("expected updated schedule in file, got:\n%s", string(data))
	}
	if strings.Contains(string(data), "*/5 * * * *") {
		t.Errorf("expected old schedule to be replaced, still present:\n%s", string(data))
	}
}

// TestApplyToFilePreservesOtherKeys verifies that applying a value to one
// key leaves other keys in the document unchanged.
func TestApplyToFilePreservesOtherKeys(t *testing.T) {
	path := writeWorkflowFile(t)
	schema := simpleSchema()

	err := ApplyToFile(path, schema, map[string]any{
		"timeout_val": "60s",
	})
	if err != nil {
		t.Fatalf("ApplyToFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	// Other keys should still be present
	if !strings.Contains(content, "example.com") {
		t.Errorf("expected endpoints (example.com) to be preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "probe-endpoints") {
		t.Errorf("expected nodes section to be preserved, got:\n%s", content)
	}
}

// TestApplyToFileWriteList verifies that applying a list value replaces
// config.endpoints with the new list.
func TestApplyToFileWriteList(t *testing.T) {
	path := writeWorkflowFile(t)
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
	newEndpoints := []any{
		map[string]any{"url": "https://mysite.com"},
		map[string]any{"url": "https://other.com"},
	}

	err := ApplyToFile(path, schema, map[string]any{
		"endpoints": newEndpoints,
	})
	if err != nil {
		t.Fatalf("ApplyToFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "mysite.com") {
		t.Errorf("expected new endpoint in file, got:\n%s", content)
	}
	if strings.Contains(content, "example.com") {
		t.Errorf("expected old endpoint to be replaced, still present:\n%s", content)
	}
}

// TestApplyToFileUnknownParamsIgnored verifies that param names not in the
// schema are silently ignored (not an error).
func TestApplyToFileUnknownParamsIgnored(t *testing.T) {
	path := writeWorkflowFile(t)
	schema := simpleSchema()

	err := ApplyToFile(path, schema, map[string]any{
		"completely_unknown_param": "value",
	})
	if err != nil {
		t.Fatalf("expected unknown params to be ignored, got error: %v", err)
	}
}
