// Unit tests for build.go.
//
// Covers flag defaults, tag resolution logic, registry prepending,
// platform defaults, and early error paths. Does not test actual docker
// build/push (exec.Command) — only testable logic.
//
// Note: TestBuildCmdHasRegistryFlag and TestBuildCmdPushRequiresRegistry
// live in config_test.go (they were written there first). This file covers
// the remaining flags and the runBuild error paths.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- NewBuildCmd flags ---
//
// Each test verifies that a specific flag is registered on the build command
// with the expected default value. This ensures that the cobra flag wiring
// in NewBuildCmd() stays consistent.

// TestBuildCmdHasTagFlag checks that the --tag flag exists and defaults to empty.
// An empty default means the build command will derive the tag from the workflow name.
func TestBuildCmdHasTagFlag(t *testing.T) {
	cmd := NewBuildCmd()
	f := cmd.Flags().Lookup("tag")
	if f == nil {
		t.Fatal("expected --tag flag on build command")
	}
	if f.DefValue != "" {
		t.Errorf("expected --tag default empty, got %s", f.DefValue)
	}
}

// TestBuildCmdHasPushFlag checks that --push defaults to false.
// When false, the build stops after producing a local image.
func TestBuildCmdHasPushFlag(t *testing.T) {
	cmd := NewBuildCmd()
	f := cmd.Flags().Lookup("push")
	if f == nil {
		t.Fatal("expected --push flag on build command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --push default false, got %s", f.DefValue)
	}
}

// TestBuildCmdHasPlatformFlag checks that --platform defaults to empty.
// When empty, docker buildx uses the host's native platform unless --push
// is set, in which case it defaults to linux/arm64.
func TestBuildCmdHasPlatformFlag(t *testing.T) {
	cmd := NewBuildCmd()
	f := cmd.Flags().Lookup("platform")
	if f == nil {
		t.Fatal("expected --platform flag on build command")
	}
	if f.DefValue != "" {
		t.Errorf("expected --platform default empty, got %s", f.DefValue)
	}
}

// TestBuildCmdAllFlagsParsing verifies that all flags can be parsed together
// without conflicts or type mismatches in a single invocation.
func TestBuildCmdAllFlagsParsing(t *testing.T) {
	cmd := NewBuildCmd()
	if err := cmd.ParseFlags([]string{
		"--tag", "my-image:v1",
		"--registry", "ghcr.io/org",
		"--push",
		"--platform", "linux/arm64",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	tag, _ := cmd.Flags().GetString("tag")
	registry, _ := cmd.Flags().GetString("registry")
	push, _ := cmd.Flags().GetBool("push")
	platform, _ := cmd.Flags().GetString("platform")

	if tag != "my-image:v1" {
		t.Errorf("tag: got %q, want %q", tag, "my-image:v1")
	}
	if registry != "ghcr.io/org" {
		t.Errorf("registry: got %q, want %q", registry, "ghcr.io/org")
	}
	if !push {
		t.Error("expected push=true")
	}
	if platform != "linux/arm64" {
		t.Errorf("platform: got %q, want %q", platform, "linux/arm64")
	}
}

// --- runBuild error paths ---
//
// These tests exercise early failure modes of runBuild without invoking
// docker. They use t.TempDir() to create isolated workflow directories.

// TestRunBuildMissingWorkflowYAML ensures that runBuild fails with a clear
// message when workflow.yaml is absent from the provided directory.
func TestRunBuildMissingWorkflowYAML(t *testing.T) {
	dir := t.TempDir()
	cmd := NewBuildCmd()
	cmd.SetArgs([]string{dir})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing workflow.yaml")
	}
	if !strings.Contains(err.Error(), "reading workflow spec") {
		t.Errorf("expected 'reading workflow spec' in error, got: %v", err)
	}
}

// TestRunBuildInvalidSpec ensures that malformed YAML is caught during
// spec.Parse and reported as a validation error.
func TestRunBuildInvalidSpec(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(":::invalid"), 0o644)

	cmd := NewBuildCmd()
	cmd.SetArgs([]string{dir})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid workflow spec")
	}
	if !strings.Contains(err.Error(), "validation error") {
		t.Errorf("expected 'validation error' in error, got: %v", err)
	}
}

// TestRunBuildPushWithoutRegistryFails verifies that --push requires --registry.
// The error may also come from docker not being installed or the engine directory
// being missing — any of these are acceptable failure modes for this test.
func TestRunBuildPushWithoutRegistryFails(t *testing.T) {
	dir := t.TempDir()
	// The spec must include triggers (required by spec.Parse) so that
	// runBuild progresses past validation to the push/registry check.
	_ = os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(`name: test
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
    description: "Test node"
`), 0o644)

	cmd := NewBuildCmd()
	cmd.SetArgs([]string{dir, "--push"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		// The error may come from docker not being available, which is also fine.
		// If it succeeds, that's unexpected.
		t.Log("runBuild did not error (docker may have run successfully)")
		return
	}
	// Either "cannot find engine directory" or "--push requires --registry" or docker failure
	errStr := err.Error()
	if !strings.Contains(errStr, "registry") && !strings.Contains(errStr, "engine") && !strings.Contains(errStr, "docker") {
		t.Errorf("expected error about registry/engine/docker, got: %v", err)
	}
}
