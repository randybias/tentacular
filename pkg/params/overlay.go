package params

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ApplyToFile reads workflow.yaml at path, applies the given parameter values
// according to the schema, and writes the result back to path.
func ApplyToFile(workflowPath string, schema *Schema, values map[string]any) error {
	data, readErr := os.ReadFile(workflowPath) //nolint:gosec // path is controlled by caller
	if readErr != nil {
		return fmt.Errorf("reading workflow.yaml: %w", readErr)
	}

	var doc yaml.Node
	if parseErr := yaml.Unmarshal(data, &doc); parseErr != nil {
		return fmt.Errorf("parsing workflow.yaml: %w", parseErr)
	}

	// yaml.Unmarshal into yaml.Node produces a document node wrapping the root mapping.
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return errors.New("workflow.yaml has unexpected structure")
	}
	root := doc.Content[0]

	for paramName, val := range values {
		def, ok := schema.Parameters[paramName]
		if !ok {
			continue // ignore unknown params
		}
		segs, parseErr := ParsePath(def.Path)
		if parseErr != nil {
			return fmt.Errorf("parameter '%s': %w", paramName, parseErr)
		}
		if setErr := setNodeValue(root, segs, val); setErr != nil {
			return fmt.Errorf("parameter '%s' (path '%s'): %w", paramName, def.Path, setErr)
		}
	}

	out, marshalErr := yaml.Marshal(&doc)
	if marshalErr != nil {
		return fmt.Errorf("marshaling workflow.yaml: %w", marshalErr)
	}
	if writeErr := os.WriteFile(workflowPath, out, 0o644); writeErr != nil { //nolint:gosec // writing user's workflow file
		return fmt.Errorf("writing workflow.yaml: %w", writeErr)
	}
	return nil
}

// GetNodeValue resolves a path expression in a YAML document and returns the matching
// node's value, or an error if the path does not exist.
func GetNodeValue(root *yaml.Node, segs []Segment) (*yaml.Node, error) {
	cur := root
	for _, seg := range segs {
		next, err := navigateSegment(cur, seg)
		if err != nil {
			return nil, err
		}
		cur = next
	}
	return cur, nil
}

// setNodeValue resolves a path and sets the leaf node to the given Go value.
func setNodeValue(root *yaml.Node, segs []Segment, val any) error {
	if len(segs) == 0 {
		return errors.New("empty path")
	}

	// Navigate to the parent of the target node.
	parent := root
	for _, seg := range segs[:len(segs)-1] {
		next, err := navigateSegment(parent, seg)
		if err != nil {
			return err
		}
		parent = next
	}

	lastSeg := segs[len(segs)-1]

	// The parent must be a mapping node for the final key lookup.
	if parent.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got kind %d", parent.Kind)
	}

	// Find the key in the mapping and set its value node.
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == lastSeg.Key {
			newVal, err := toYAMLNode(val)
			if err != nil {
				return err
			}
			parent.Content[i+1] = newVal
			return nil
		}
	}

	// Key not found -- suggest available keys
	available := mappingKeys(parent)
	return fmt.Errorf("key '%s' not found in mapping; available keys: %v", lastSeg.Key, available)
}

// navigateSegment moves from cur into the child specified by seg.
func navigateSegment(cur *yaml.Node, seg Segment) (*yaml.Node, error) {
	switch cur.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(cur.Content); i += 2 {
			if cur.Content[i].Value == seg.Key {
				child := cur.Content[i+1]
				if seg.FilterField != "" {
					return applyFilter(child, seg)
				}
				return child, nil
			}
		}
		available := mappingKeys(cur)
		return nil, fmt.Errorf("key '%s' not found; available keys: %v", seg.Key, available)

	case yaml.SequenceNode:
		// Sequence is navigated via filter
		if seg.FilterField == "" {
			return nil, errors.New("sequence navigation requires a filter (use key[field=value])")
		}
		return applyFilter(cur, seg)

	default:
		return nil, fmt.Errorf("cannot navigate into node of kind %d", cur.Kind)
	}
}

// applyFilter searches a sequence node for an element whose FilterField equals FilterValue.
func applyFilter(seq *yaml.Node, seg Segment) (*yaml.Node, error) {
	target := seq
	if target.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("filter on '%s': expected sequence, got kind %d", seg.Key, target.Kind)
	}

	for _, elem := range target.Content {
		if elem.Kind != yaml.MappingNode {
			continue
		}
		// Look for field=value in this mapping element.
		for i := 0; i+1 < len(elem.Content); i += 2 {
			if elem.Content[i].Value == seg.FilterField && elem.Content[i+1].Value == seg.FilterValue {
				return elem, nil
			}
		}
	}
	return nil, fmt.Errorf("no element in sequence has %s=%s", seg.FilterField, seg.FilterValue)
}

// mappingKeys returns the list of key strings for a mapping node (for error messages).
func mappingKeys(n *yaml.Node) []string {
	var keys []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		keys = append(keys, n.Content[i].Value)
	}
	return keys
}

// toYAMLNode converts a Go value to a yaml.Node for insertion into a document.
func toYAMLNode(val any) (*yaml.Node, error) {
	raw, err := yaml.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("marshaling parameter value: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("re-parsing parameter value: %w", err)
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
}
