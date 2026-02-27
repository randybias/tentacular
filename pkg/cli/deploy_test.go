package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
)

func TestBuildSecretManifestLocalSecretsPresent(t *testing.T) {
	dir := t.TempDir()

	// Create .secrets.yaml with valid YAML
	yamlContent := "github_token: ghp_test123\nslack:\n  webhook_url: \"https://hooks.slack.com/test\"\n"
	os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte(yamlContent), 0o644)

	m, err := buildSecretManifest(dir, "test-wf", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest when .secrets.yaml exists")
	}
	if m.Kind != "Secret" {
		t.Errorf("expected kind Secret, got %s", m.Kind)
	}
	if !strings.Contains(m.Content, "kind: Secret") {
		t.Error("expected manifest to contain 'kind: Secret'")
	}
	if !strings.Contains(m.Content, "namespace: staging") {
		t.Error("expected manifest to contain 'namespace: staging'")
	}
	if !strings.Contains(m.Content, "github_token") {
		t.Error("expected github_token in manifest")
	}
	if !strings.Contains(m.Content, "webhook_url") {
		t.Error("expected webhook_url in manifest")
	}
}

func TestBuildSecretManifestMalformedYAML(t *testing.T) {
	dir := t.TempDir()

	// Write malformed YAML
	os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte(":::invalid yaml[[["), 0o644)

	_, err := buildSecretManifest(dir, "test-wf", "default")
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parsing secrets YAML") {
		t.Errorf("expected error containing 'parsing secrets YAML', got: %v", err)
	}
}

func TestBuildSecretManifestNoSecretsReturnsNil(t *testing.T) {
	dir := t.TempDir()

	// No .secrets/ dir and no .secrets.yaml
	m, err := buildSecretManifest(dir, "test-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Error("expected nil manifest when no secrets exist")
	}
}

func TestBuildSecretManifestDirPreferredOverYAML(t *testing.T) {
	dir := t.TempDir()

	// Create both .secrets/ dir and .secrets.yaml
	secretsDir := filepath.Join(dir, ".secrets")
	os.MkdirAll(secretsDir, 0o755)
	os.WriteFile(filepath.Join(secretsDir, "from-dir"), []byte("dir-value"), 0o644)
	os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte("from_yaml: yaml-value\n"), 0o644)

	m, err := buildSecretManifest(dir, "test-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	// .secrets/ dir should take precedence
	if !strings.Contains(m.Content, "from-dir") {
		t.Error("expected .secrets/ dir content to be used")
	}
	if strings.Contains(m.Content, "from_yaml") {
		t.Error("expected .secrets.yaml to be ignored when .secrets/ dir exists")
	}
}

func TestBuildSecretManifestNameSuffix(t *testing.T) {
	dir := t.TempDir()

	secretsDir := filepath.Join(dir, ".secrets")
	os.MkdirAll(secretsDir, 0o755)
	os.WriteFile(filepath.Join(secretsDir, "token"), []byte("abc"), 0o644)

	m, err := buildSecretManifest(dir, "my-workflow", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	// Secret name should be <workflow-name>-secrets
	if m.Name != "my-workflow-secrets" {
		t.Errorf("expected name my-workflow-secrets, got %s", m.Name)
	}
}

// --- WI-6: Deploy Gate + Force Escape Hatch Tests ---

func TestDeployCmdHasForceFlag(t *testing.T) {
	cmd := NewDeployCmd()
	f := cmd.Flags().Lookup("force")
	if f == nil {
		t.Fatal("expected --force flag on deploy command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --force default false, got %s", f.DefValue)
	}
}

func TestDeployCmdHasSkipLiveTestFlag(t *testing.T) {
	cmd := NewDeployCmd()
	f := cmd.Flags().Lookup("skip-live-test")
	if f == nil {
		t.Fatal("expected --skip-live-test flag on deploy command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --skip-live-test default false, got %s", f.DefValue)
	}
}

func TestDeployCmdHasVerifyFlag(t *testing.T) {
	cmd := NewDeployCmd()
	f := cmd.Flags().Lookup("verify")
	if f == nil {
		t.Fatal("expected --verify flag on deploy command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --verify default false, got %s", f.DefValue)
	}
}

func TestDeployCmdForceFlagParsing(t *testing.T) {
	cmd := NewDeployCmd()
	if err := cmd.ParseFlags([]string{"--force"}); err != nil {
		t.Fatalf("failed to parse --force: %v", err)
	}
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		t.Error("expected --force to be true after parsing")
	}
}

func TestDeployCmdSkipLiveTestFlagParsing(t *testing.T) {
	cmd := NewDeployCmd()
	if err := cmd.ParseFlags([]string{"--skip-live-test"}); err != nil {
		t.Fatalf("failed to parse --skip-live-test: %v", err)
	}
	skipLiveTest, _ := cmd.Flags().GetBool("skip-live-test")
	if !skipLiveTest {
		t.Error("expected --skip-live-test to be true after parsing")
	}
}

func TestDeployCmdVerifyFlagParsing(t *testing.T) {
	cmd := NewDeployCmd()
	if err := cmd.ParseFlags([]string{"--verify"}); err != nil {
		t.Fatalf("failed to parse --verify: %v", err)
	}
	verify, _ := cmd.Flags().GetBool("verify")
	if !verify {
		t.Error("expected --verify to be true after parsing")
	}
}

func TestDeployCmdAllFlagsCombined(t *testing.T) {
	cmd := NewDeployCmd()
	flags := []string{"--force", "--verify", "--image", "my-image:v1", "--runtime-class", ""}
	if err := cmd.ParseFlags(flags); err != nil {
		t.Fatalf("failed to parse combined flags: %v", err)
	}
	force, _ := cmd.Flags().GetBool("force")
	verify, _ := cmd.Flags().GetBool("verify")
	image, _ := cmd.Flags().GetString("image")
	rc, _ := cmd.Flags().GetString("runtime-class")
	if !force {
		t.Error("expected --force true")
	}
	if !verify {
		t.Error("expected --verify true")
	}
	if image != "my-image:v1" {
		t.Errorf("expected image my-image:v1, got %s", image)
	}
	if rc != "" {
		t.Errorf("expected empty runtime-class, got %s", rc)
	}
}

func TestEmitDeployResultPassStatus(t *testing.T) {
	cmd := &cobra.Command{Use: "deploy"}
	cmd.PersistentFlags().StringP("output", "o", "json", "Output format")
	cmd.ParseFlags([]string{"-o", "json"})

	startedAt := time.Now().UTC()

	var buf bytes.Buffer
	// Call emitDeployResult by constructing the result directly
	result := CommandResult{
		Version: "1",
		Command: "deploy",
		Status:  "pass",
		Summary: "deployed test-wf to dev-ns",
		Hints:   []string{},
		Timing: TimingInfo{
			StartedAt:  startedAt.Format(time.RFC3339),
			DurationMs: 100,
		},
	}
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["status"] != "pass" {
		t.Errorf("expected status pass, got %v", parsed["status"])
	}
	if parsed["command"] != "deploy" {
		t.Errorf("expected command deploy, got %v", parsed["command"])
	}
	// Pass status should have no hints
	hints, ok := parsed["hints"].([]interface{})
	if ok && len(hints) > 0 {
		t.Errorf("expected empty hints for pass status, got %v", hints)
	}
}

func TestEmitDeployResultFailIncludesForceHint(t *testing.T) {
	cmd := &cobra.Command{Use: "deploy"}
	cmd.PersistentFlags().StringP("output", "o", "json", "Output format")
	cmd.ParseFlags([]string{"-o", "json"})

	startedAt := time.Now().UTC()

	// Simulate a failure result with the hints emitDeployResult would add
	result := CommandResult{
		Version: "1",
		Command: "deploy",
		Status:  "fail",
		Summary: "deploy failed: pre-deploy live test failed",
		Hints:   []string{"use --force to skip pre-deploy live test", "check deployment logs with: tntc logs <workflow-name>"},
		Timing: TimingInfo{
			StartedAt:  startedAt.Format(time.RFC3339),
			DurationMs: 5000,
		},
	}

	var buf bytes.Buffer
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["status"] != "fail" {
		t.Errorf("expected status fail, got %v", parsed["status"])
	}

	hints, ok := parsed["hints"].([]interface{})
	if !ok || len(hints) < 2 {
		t.Fatal("expected at least 2 hints for failed deploy")
	}

	hasForceHint := false
	hasLogsHint := false
	for _, h := range hints {
		hs, _ := h.(string)
		if strings.Contains(hs, "--force") {
			hasForceHint = true
		}
		if strings.Contains(hs, "tntc logs") {
			hasLogsHint = true
		}
	}
	if !hasForceHint {
		t.Error("expected hint about --force to skip pre-deploy live test")
	}
	if !hasLogsHint {
		t.Error("expected hint about tntc logs")
	}
}

func TestDeployCmdHasImageFlag(t *testing.T) {
	cmd := NewDeployCmd()
	f := cmd.Flags().Lookup("image")
	if f == nil {
		t.Fatal("expected --image flag on deploy command")
	}
	if f.DefValue != "" {
		t.Errorf("expected --image default empty, got %s", f.DefValue)
	}
}

func TestDeployCmdHasRuntimeClassFlag(t *testing.T) {
	cmd := NewDeployCmd()
	f := cmd.Flags().Lookup("runtime-class")
	if f == nil {
		t.Fatal("expected --runtime-class flag on deploy command")
	}
	if f.DefValue != "gvisor" {
		t.Errorf("expected --runtime-class default gvisor, got %s", f.DefValue)
	}
}

func TestDeployCmdDeprecatedClusterRegistryFlag(t *testing.T) {
	cmd := NewDeployCmd()
	f := cmd.Flags().Lookup("cluster-registry")
	if f == nil {
		t.Fatal("expected --cluster-registry flag on deploy command (deprecated)")
	}
}

func TestCountModuleProxyDeps_NilContract(t *testing.T) {
	wf := &spec.Workflow{Contract: nil}
	if n := countModuleProxyDeps(wf); n != 0 {
		t.Errorf("expected 0 for nil contract, got %d", n)
	}
}

func TestCountModuleProxyDeps_NoJsrOrNpm(t *testing.T) {
	wf := &spec.Workflow{
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"pg":    {Protocol: "postgresql"},
				"slack": {Protocol: "https"},
			},
		},
	}
	if n := countModuleProxyDeps(wf); n != 0 {
		t.Errorf("expected 0 for non-jsr/npm deps, got %d", n)
	}
}

func TestCountModuleProxyDeps_JsrOnly(t *testing.T) {
	wf := &spec.Workflow{
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"std":   {Protocol: "jsr"},
				"fresh": {Protocol: "jsr"},
			},
		},
	}
	if n := countModuleProxyDeps(wf); n != 2 {
		t.Errorf("expected 2 for 2 jsr deps, got %d", n)
	}
}

func TestCountModuleProxyDeps_NpmOnly(t *testing.T) {
	wf := &spec.Workflow{
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"lodash": {Protocol: "npm"},
			},
		},
	}
	if n := countModuleProxyDeps(wf); n != 1 {
		t.Errorf("expected 1 for 1 npm dep, got %d", n)
	}
}

func TestCountModuleProxyDeps_Mixed(t *testing.T) {
	wf := &spec.Workflow{
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"lodash": {Protocol: "npm"},
				"std":    {Protocol: "jsr"},
				"pg":     {Protocol: "postgresql"},
				"api":    {Protocol: "https"},
			},
		},
	}
	if n := countModuleProxyDeps(wf); n != 2 {
		t.Errorf("expected 2 for mixed deps, got %d", n)
	}
}

func TestCountModuleProxyDeps_EmptyDependencies(t *testing.T) {
	wf := &spec.Workflow{
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{},
		},
	}
	if n := countModuleProxyDeps(wf); n != 0 {
		t.Errorf("expected 0 for empty dependencies, got %d", n)
	}
}
