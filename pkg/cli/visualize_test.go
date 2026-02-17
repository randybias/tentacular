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
