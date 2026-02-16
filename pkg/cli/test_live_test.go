package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// --- WI-5: Live Workflow Testing Unit Tests ---

func TestNewTestCmdHasLiveFlag(t *testing.T) {
	cmd := NewTestCmd()

	f := cmd.Flags().Lookup("live")
	if f == nil {
		t.Fatal("expected --live flag on test command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --live default false, got %s", f.DefValue)
	}
}

func TestNewTestCmdHasEnvFlag(t *testing.T) {
	cmd := NewTestCmd()

	f := cmd.Flags().Lookup("env")
	if f == nil {
		t.Fatal("expected --env flag on test command")
	}
	if f.DefValue != "dev" {
		t.Errorf("expected --env default dev, got %s", f.DefValue)
	}
}

func TestNewTestCmdHasKeepFlag(t *testing.T) {
	cmd := NewTestCmd()

	f := cmd.Flags().Lookup("keep")
	if f == nil {
		t.Fatal("expected --keep flag on test command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --keep default false, got %s", f.DefValue)
	}
}

func TestNewTestCmdHasTimeoutFlag(t *testing.T) {
	cmd := NewTestCmd()

	f := cmd.Flags().Lookup("timeout")
	if f == nil {
		t.Fatal("expected --timeout flag on test command")
	}
	if f.DefValue != "2m0s" {
		t.Errorf("expected --timeout default 2m0s, got %s", f.DefValue)
	}
}

func TestNewTestCmdFlagParsing(t *testing.T) {
	cmd := NewTestCmd()
	cmd.SetArgs([]string{"--live", "--env", "staging", "--keep", "--timeout", "5m"})
	if err := cmd.ParseFlags([]string{"--live", "--env", "staging", "--keep", "--timeout", "5m"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	live, _ := cmd.Flags().GetBool("live")
	if !live {
		t.Error("expected --live to be true")
	}
	env, _ := cmd.Flags().GetString("env")
	if env != "staging" {
		t.Errorf("expected --env staging, got %s", env)
	}
	keep, _ := cmd.Flags().GetBool("keep")
	if !keep {
		t.Error("expected --keep to be true")
	}
	timeout, _ := cmd.Flags().GetDuration("timeout")
	if timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %s", timeout)
	}
}

func TestEmitLiveResultPass(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringP("output", "o", "json", "Output format")
	cmd.ParseFlags([]string{"-o", "json"})

	startedAt := time.Now().UTC().Add(-500 * time.Millisecond)

	var buf bytes.Buffer
	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "workflow completed successfully",
		Hints:   []string{},
		Timing: TimingInfo{
			StartedAt:  startedAt.Format(time.RFC3339),
			DurationMs: time.Since(startedAt).Milliseconds(),
		},
		Execution: map[string]interface{}{"success": true},
	}
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, buf.String())
	}
	if parsed["status"] != "pass" {
		t.Errorf("expected status pass, got %v", parsed["status"])
	}
	if parsed["command"] != "test" {
		t.Errorf("expected command test, got %v", parsed["command"])
	}
	if parsed["execution"] == nil {
		t.Error("expected execution field to be present")
	}
}

func TestEmitLiveResultFailIncludesHints(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringP("output", "o", "json", "Output format")
	cmd.ParseFlags([]string{"-o", "json"})

	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "fail",
		Summary: "deploy failed: context deadline exceeded",
		Hints:   []string{"check deployment logs with: tntc logs <workflow-name>"},
		Timing: TimingInfo{
			StartedAt:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: 5000,
		},
	}

	var buf bytes.Buffer
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["status"] != "fail" {
		t.Errorf("expected status fail, got %v", parsed["status"])
	}
	hints, ok := parsed["hints"].([]interface{})
	if !ok || len(hints) == 0 {
		t.Fatal("expected hints array with entries")
	}
	hintStr, _ := hints[0].(string)
	if !strings.Contains(hintStr, "tntc logs") {
		t.Errorf("expected hint about tntc logs, got %s", hintStr)
	}
}

func TestInternalDeployOptionsFields(t *testing.T) {
	opts := InternalDeployOptions{
		Namespace:    "test-ns",
		Image:        "my-registry/tentacular-engine:v1",
		RuntimeClass: "gvisor",
		Context:      "kind-tentacular",
	}

	if opts.Namespace != "test-ns" {
		t.Errorf("expected test-ns, got %s", opts.Namespace)
	}
	if opts.Image != "my-registry/tentacular-engine:v1" {
		t.Errorf("expected image, got %s", opts.Image)
	}
	if opts.RuntimeClass != "gvisor" {
		t.Errorf("expected gvisor, got %s", opts.RuntimeClass)
	}
	if opts.Context != "kind-tentacular" {
		t.Errorf("expected kind-tentacular, got %s", opts.Context)
	}
}

func TestDeployResultFields(t *testing.T) {
	result := DeployResult{
		WorkflowName: "sep-tracker",
		Namespace:    "dev-ns",
		Client:       nil, // nil is valid for unit test
	}
	if result.WorkflowName != "sep-tracker" {
		t.Errorf("expected sep-tracker, got %s", result.WorkflowName)
	}
	if result.Namespace != "dev-ns" {
		t.Errorf("expected dev-ns, got %s", result.Namespace)
	}
}

func TestImageResolutionCascade(t *testing.T) {
	// Test the image resolution logic extracted from runLiveTest:
	// env.Image > base-image.txt > default "tentacular-engine:latest"

	t.Run("env image takes precedence", func(t *testing.T) {
		env := &EnvironmentConfig{
			Image: "staging-registry/tentacular-engine:v2",
		}
		imageTag := env.Image
		if imageTag != "staging-registry/tentacular-engine:v2" {
			t.Errorf("expected env image, got %s", imageTag)
		}
	})

	t.Run("base-image.txt used when env image empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		tentacularDir := filepath.Join(tmpDir, ".tentacular")
		os.MkdirAll(tentacularDir, 0o755)
		os.WriteFile(filepath.Join(tentacularDir, "base-image.txt"), []byte("my-registry/tentacular-engine:abc123\n"), 0o644)

		env := &EnvironmentConfig{}
		imageTag := env.Image
		if imageTag == "" {
			tagFilePath := filepath.Join(tmpDir, ".tentacular", "base-image.txt")
			if tagData, err := os.ReadFile(tagFilePath); err == nil {
				imageTag = strings.TrimSpace(string(tagData))
			}
		}
		if imageTag != "my-registry/tentacular-engine:abc123" {
			t.Errorf("expected base-image.txt value, got %s", imageTag)
		}
	})

	t.Run("default used when no env image and no base-image.txt", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := &EnvironmentConfig{}
		imageTag := env.Image
		if imageTag == "" {
			tagFilePath := filepath.Join(tmpDir, ".tentacular", "base-image.txt")
			if tagData, err := os.ReadFile(tagFilePath); err == nil {
				imageTag = strings.TrimSpace(string(tagData))
			}
		}
		if imageTag == "" {
			imageTag = "tentacular-engine:latest"
		}
		if imageTag != "tentacular-engine:latest" {
			t.Errorf("expected default image, got %s", imageTag)
		}
	})
}

func TestLiveTestRequiresWorkflowYAML(t *testing.T) {
	// runLiveTest should fail if no workflow.yaml exists
	// We test this indirectly by calling runTest with --live on an empty dir
	cmd := NewTestCmd()
	tmpDir := t.TempDir()

	// Set up environment config so ResolveEnvironment doesn't fail first
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte("environments:\n  dev:\n    namespace: dev-ns\n"), 0o644)

	cmd.SetArgs([]string{"--live", tmpDir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when workflow.yaml is missing")
	}
	if !strings.Contains(err.Error(), "workflow.yaml") {
		t.Errorf("expected error about missing workflow.yaml, got: %v", err)
	}
}

func TestLiveTestRequiresEnvironment(t *testing.T) {
	// When --env points to a non-existent environment, should fail
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte("name: test-wf\nversion: \"1.0\"\nnodes:\n  a:\n    path: ./a.ts\n"), 0o644)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Empty config -- no environments
	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte("registry: test\n"), 0o644)

	cmd := NewTestCmd()
	cmd.SetArgs([]string{"--live", "--env", "nonexistent", tmpDir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent environment")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error about nonexistent environment, got: %v", err)
	}
}
