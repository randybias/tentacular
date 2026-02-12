package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/k8s"
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

func TestEvaluatePreflightResultsDowngradesSecretWarning(t *testing.T) {
	// When hasLocalSecrets=false and a secret-reference check fails,
	// the failure should be downgraded to a warning (return false, not true)
	results := []k8s.CheckResult{
		{Name: "K8s API reachable", Passed: true},
		{Name: "gVisor RuntimeClass", Passed: true},
		{Name: "Namespace 'default'", Passed: true},
		{Name: "RBAC permissions", Passed: true},
		{Name: "Secret references", Passed: false, Remediation: "Missing secrets in namespace default: my-wf-secrets"},
	}

	failed := evaluatePreflightResults(results, false)
	if failed {
		t.Error("expected evaluatePreflightResults to return false (warning) when hasLocalSecrets=false and secret check fails")
	}
}

func TestEvaluatePreflightResultsHardFailWithLocalSecrets(t *testing.T) {
	// When hasLocalSecrets=true and a secret-reference check fails,
	// it should remain a hard failure (return true)
	results := []k8s.CheckResult{
		{Name: "K8s API reachable", Passed: true},
		{Name: "Secret references", Passed: false, Remediation: "Missing secrets"},
	}

	failed := evaluatePreflightResults(results, true)
	if !failed {
		t.Error("expected evaluatePreflightResults to return true (hard failure) when hasLocalSecrets=true and secret check fails")
	}
}

func TestEvaluatePreflightResultsAllPass(t *testing.T) {
	// When all checks pass, should return false regardless of hasLocalSecrets
	results := []k8s.CheckResult{
		{Name: "K8s API reachable", Passed: true},
		{Name: "gVisor RuntimeClass", Passed: true},
		{Name: "Namespace 'default'", Passed: true},
		{Name: "RBAC permissions", Passed: true},
		{Name: "Secret references", Passed: true},
	}

	if evaluatePreflightResults(results, false) {
		t.Error("expected false when all checks pass (hasLocalSecrets=false)")
	}
	if evaluatePreflightResults(results, true) {
		t.Error("expected false when all checks pass (hasLocalSecrets=true)")
	}
}

func TestEvaluatePreflightResultsNonSecretFailure(t *testing.T) {
	// Non-secret failures should always be hard failures
	results := []k8s.CheckResult{
		{Name: "K8s API reachable", Passed: true},
		{Name: "RBAC permissions", Passed: false, Remediation: "Missing permissions"},
	}

	if !evaluatePreflightResults(results, false) {
		t.Error("expected true for non-secret failure even when hasLocalSecrets=false")
	}
	if !evaluatePreflightResults(results, true) {
		t.Error("expected true for non-secret failure when hasLocalSecrets=true")
	}
}
