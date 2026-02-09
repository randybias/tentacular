package spec

import (
	"strings"
	"testing"
)

func TestParseValidSpec(t *testing.T) {
	yaml := `
name: test-workflow
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
  transform:
    path: ./nodes/transform.ts
edges:
  - from: fetch
    to: transform
config:
  timeout: 30s
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Name != "test-workflow" {
		t.Errorf("expected name test-workflow, got %s", wf.Name)
	}
	if len(wf.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(wf.Nodes))
	}
	if len(wf.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(wf.Edges))
	}
}

func TestParseMissingName(t *testing.T) {
	yaml := `
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected errors for missing name")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "name is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'name is required' error, got: %v", errs)
	}
}

func TestParseInvalidName(t *testing.T) {
	yaml := `
name: NotKebab
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected errors for invalid name")
	}
}

func TestParseCycleDetection(t *testing.T) {
	yaml := `
name: cyclic
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
  b:
    path: ./b.ts
  c:
    path: ./c.ts
edges:
  - from: a
    to: b
  - from: b
    to: c
  - from: c
    to: a
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected cycle detection error")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "cycle") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cycle error, got: %v", errs)
	}
}

func TestParseEdgeReferenceIntegrity(t *testing.T) {
	yaml := `
name: bad-refs
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges:
  - from: a
    to: nonexistent
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected reference integrity error")
	}
}

func TestParseTriggerValidation(t *testing.T) {
	yaml := `
name: bad-trigger
version: "1.0"
triggers:
  - type: cron
nodes:
  a:
    path: ./a.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected cron schedule error")
	}
}
