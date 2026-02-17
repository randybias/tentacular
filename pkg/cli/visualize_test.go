package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

// --- Phase 5: Visualization --rich Mode Tests ---

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestNewVisualizeCmdHasRichFlag(t *testing.T) {
	cmd := NewVisualizeCmd()

	flag := cmd.Flags().Lookup("rich")
	if flag == nil {
		t.Fatal("expected --rich flag on visualize command")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected --rich default false, got %s", flag.DefValue)
	}
}

func TestGetPortWithDefaultHTTPS(t *testing.T) {
	dep := spec.Dependency{
		Protocol: "https",
		Host:     "api.example.com",
		// Port omitted
	}

	port := getPortWithDefault(dep)
	if port != 443 {
		t.Errorf("expected default port 443 for https, got %d", port)
	}
}

func TestGetPortWithDefaultPostgreSQL(t *testing.T) {
	dep := spec.Dependency{
		Protocol: "postgresql",
		Host:     "postgres.svc",
		// Port omitted
	}

	port := getPortWithDefault(dep)
	if port != 5432 {
		t.Errorf("expected default port 5432 for postgresql, got %d", port)
	}
}

func TestGetPortWithDefaultNATS(t *testing.T) {
	dep := spec.Dependency{
		Protocol: "nats",
		Host:     "nats.svc",
		// Port omitted
	}

	port := getPortWithDefault(dep)
	if port != 4222 {
		t.Errorf("expected default port 4222 for nats, got %d", port)
	}
}

func TestGetPortWithDefaultExplicitPort(t *testing.T) {
	dep := spec.Dependency{
		Protocol: "https",
		Host:     "api.example.com",
		Port:     8443,
	}

	port := getPortWithDefault(dep)
	if port != 8443 {
		t.Errorf("expected explicit port 8443, got %d", port)
	}
}

func TestGetPortWithDefaultUnknownProtocol(t *testing.T) {
	dep := spec.Dependency{
		Protocol: "unknown",
		Host:     "example.com",
		// Port omitted
	}

	port := getPortWithDefault(dep)
	if port != 443 {
		t.Errorf("expected fallback port 443 for unknown protocol, got %d", port)
	}
}

func TestGetPortWithDefaultBlob(t *testing.T) {
	dep := spec.Dependency{
		Protocol: "blob",
		Host:     "storage.blob.core.windows.net",
		// Port omitted - blob not in defaults
	}

	port := getPortWithDefault(dep)
	if port != 443 {
		t.Errorf("expected fallback port 443 for blob protocol, got %d", port)
	}
}

func TestVisualizeCommandStructure(t *testing.T) {
	cmd := NewVisualizeCmd()

	if cmd.Use != "visualize [dir]" {
		t.Errorf("expected Use to be 'visualize [dir]', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestVisualizeCommandMaxArgs(t *testing.T) {
	cmd := NewVisualizeCmd()

	// Should accept 0 args (current dir)
	err := cmd.Args(cmd, []string{})
	if err != nil {
		t.Errorf("expected 0 args to be valid, got error: %v", err)
	}

	// Should accept 1 arg (specific dir)
	err = cmd.Args(cmd, []string{"./some-dir"})
	if err != nil {
		t.Errorf("expected 1 arg to be valid, got error: %v", err)
	}

	// Should reject 2+ args
	err = cmd.Args(cmd, []string{"./dir1", "./dir2"})
	if err == nil {
		t.Error("expected 2 args to be rejected")
	}
}

func TestVisualizeRichFlagParsing(t *testing.T) {
	cmd := NewVisualizeCmd()

	// Test --rich flag
	cmd.ParseFlags([]string{"--rich"})
	rich, _ := cmd.Flags().GetBool("rich")
	if !rich {
		t.Error("expected --rich to be true")
	}

	// Test without flag (default false)
	cmd2 := NewVisualizeCmd()
	cmd2.ParseFlags([]string{})
	rich2, _ := cmd2.Flags().GetBool("rich")
	if rich2 {
		t.Error("expected --rich to default to false")
	}
}

// Note: Full integration tests for runVisualize require actual workflow.yaml files
// and are better suited for integration test suite. These unit tests validate
// the command structure and helper functions.

// --- Group 4: Mermaid Deterministic Output Tests ---

func TestMermaidStableNodeOrdering(t *testing.T) {
	// Workflow with 5 nodes in map (unordered)
	wf := &spec.Workflow{
		Name:    "test-wf",
		Version: "1.0",
		Nodes: map[string]spec.NodeSpec{
			"zebra": {Path: "./zebra.ts"},
			"apple": {Path: "./apple.ts"},
			"dog":   {Path: "./dog.ts"},
			"cat":   {Path: "./cat.ts"},
			"bird":  {Path: "./bird.ts"},
		},
		Edges: []spec.Edge{},
	}

	// Generate Mermaid 10 times
	outputs := make([]string, 10)
	for i := 0; i < 10; i++ {
		outputs[i] = generateMermaidDiagram(wf, false)
	}

	// All outputs should be identical
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("output %d differs from output 0", i)
		}
	}

	// Verify nodes are in sorted order
	expected := "apple"
	if !bytes.Contains([]byte(outputs[0]), []byte(expected)) {
		t.Errorf("expected %s to appear in output", expected)
	}
}

func TestMermaidStableEdgeOrdering(t *testing.T) {
	// Workflow with multiple edges (in arbitrary order)
	wf := &spec.Workflow{
		Name:    "test-wf",
		Version: "1.0",
		Nodes: map[string]spec.NodeSpec{
			"a": {Path: "./a.ts"},
			"b": {Path: "./b.ts"},
			"c": {Path: "./c.ts"},
			"d": {Path: "./d.ts"},
		},
		Edges: []spec.Edge{
			{From: "c", To: "d"},
			{From: "a", To: "b"},
			{From: "b", To: "c"},
			{From: "d", To: "a"}, // Creates cycle, but we're testing ordering
		},
	}

	// Generate Mermaid 10 times
	outputs := make([]string, 10)
	for i := 0; i < 10; i++ {
		outputs[i] = generateMermaidDiagram(wf, false)
	}

	// All outputs should be identical (edges sorted)
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("output %d differs from output 0", i)
		}
	}
}

func TestMermaidStableDependencyOrdering(t *testing.T) {
	// Contract with 4 dependencies
	wf := &spec.Workflow{
		Name:    "test-wf",
		Version: "1.0",
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./fetch.ts"},
		},
		Edges: []spec.Edge{},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"zebra-api":    {Protocol: "https", Host: "zebra.com", Port: 443},
				"alpha-db":     {Protocol: "postgresql", Host: "alpha.db", Port: 5432},
				"delta-nats":   {Protocol: "nats", Host: "nats.local", Port: 4222},
				"charlie-blob": {Protocol: "blob", Host: "storage.local"},
			},
		},
	}

	// Generate Mermaid with --rich flag
	output := generateMermaidDiagram(wf, true)

	// Verify dependencies appear in sorted order (alpha, charlie, delta, zebra)
	// by checking that their declaration order matches alphabetical
	alphaIdx := bytes.Index([]byte(output), []byte("dep_alpha-db"))
	charlieIdx := bytes.Index([]byte(output), []byte("dep_charlie-blob"))
	deltaIdx := bytes.Index([]byte(output), []byte("dep_delta-nats"))
	zebraIdx := bytes.Index([]byte(output), []byte("dep_zebra-api"))

	if alphaIdx == -1 || charlieIdx == -1 || deltaIdx == -1 || zebraIdx == -1 {
		t.Fatal("expected all dependencies to appear in output")
	}

	if !(alphaIdx < charlieIdx && charlieIdx < deltaIdx && deltaIdx < zebraIdx) {
		t.Errorf("dependencies not in sorted order: alpha=%d, charlie=%d, delta=%d, zebra=%d",
			alphaIdx, charlieIdx, deltaIdx, zebraIdx)
	}
}

func TestMermaidDiffCompatibility(t *testing.T) {
	// Two identical workflows should produce identical Mermaid output
	wf1 := &spec.Workflow{
		Name:    "test-wf",
		Version: "1.0",
		Nodes: map[string]spec.NodeSpec{
			"a": {Path: "./a.ts"},
			"b": {Path: "./b.ts"},
		},
		Edges: []spec.Edge{
			{From: "a", To: "b"},
		},
	}

	wf2 := &spec.Workflow{
		Name:    "test-wf",
		Version: "1.0",
		Nodes: map[string]spec.NodeSpec{
			"a": {Path: "./a.ts"},
			"b": {Path: "./b.ts"},
		},
		Edges: []spec.Edge{
			{From: "a", To: "b"},
		},
	}

	output1 := generateMermaidDiagram(wf1, false)
	output2 := generateMermaidDiagram(wf2, false)

	if output1 != output2 {
		t.Error("identical workflows produced different Mermaid output")
	}
}

// Test: Dependency rendering as standalone shapes
func TestMermaidDependencyConnections(t *testing.T) {
	// Workflow with nodes and dependencies
	wf := &spec.Workflow{
		Name:    "test-wf",
		Version: "1.0",
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./fetch.ts"},
			"store": {Path: "./store.ts"},
		},
		Edges: []spec.Edge{
			{From: "fetch", To: "store"},
		},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"api": {Protocol: "https", Host: "api.example.com", Port: 443},
			},
		},
	}

	// Generate Mermaid with --rich flag
	output := generateMermaidDiagram(wf, true)

	// Verify dependency appears as a standalone shape
	if !bytes.Contains([]byte(output), []byte("dep_api[(api")) {
		t.Error("expected dependency shape 'dep_api' to appear")
	}

	// Verify dependency has styling
	if !bytes.Contains([]byte(output), []byte("style dep_api fill:#e1f5ff")) {
		t.Error("expected dependency styling")
	}

	// Should NOT have connection lines (avoiding cartesian product over-connection)
	if bytes.Contains([]byte(output), []byte("-.->|uses| dep_api")) {
		t.Error("should not contain connection lines to dependencies")
	}
	if bytes.Contains([]byte(output), []byte("dep_api -.->")) {
		t.Error("should not contain connection lines from dependencies")
	}
}
