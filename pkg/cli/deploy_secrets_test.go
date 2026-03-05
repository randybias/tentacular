package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSecretManifestSharedRefsOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Create a fake repo root with .git marker and .secrets/ directory
	repoRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	sharedDir := filepath.Join(repoRoot, ".secrets")
	_ = os.MkdirAll(sharedDir, 0o755)
	_ = os.WriteFile(filepath.Join(sharedDir, "slack"), []byte(`{"webhook_url":"https://hooks.slack.com/T00"}`), 0o644)
	_ = os.WriteFile(filepath.Join(sharedDir, "github"), []byte(`{"token":"ghp_test123"}`), 0o644)

	// Create a workflow directory under the repo root
	wfDir := filepath.Join(repoRoot, "workflows", "my-wf")
	_ = os.MkdirAll(wfDir, 0o755)

	// Create .secrets.yaml with $shared references
	yamlContent := "slack: $shared.slack\ngithub: $shared.github\n"
	_ = os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(yamlContent), 0o644)

	m, err := buildSecretManifest(wfDir, "my-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "webhook_url") {
		t.Error("expected shared slack secret to be resolved")
	}
	if !strings.Contains(m.Content, "ghp_test123") {
		t.Error("expected shared github secret to be resolved")
	}
	if strings.Contains(m.Content, "$shared.") {
		t.Error("expected $shared references to be resolved, not left as-is")
	}
}

func TestBuildSecretManifestRejectsDirectValues(t *testing.T) {
	repoRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)

	wfDir := filepath.Join(repoRoot, "workflows", "my-wf")
	_ = os.MkdirAll(wfDir, 0o755)

	// Create .secrets.yaml with a direct value (not $shared)
	yamlContent := "api_key: sk_test_direct_value\n"
	_ = os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(yamlContent), 0o644)

	_, err := buildSecretManifest(wfDir, "my-wf", "default")
	if err == nil {
		t.Fatal("expected error for direct secret value")
	}
	if !strings.Contains(err.Error(), "direct value") {
		t.Errorf("expected 'direct value' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "$shared") {
		t.Errorf("expected error to mention $shared, got: %v", err)
	}
}

func TestBuildSecretManifestRejectsNestedValues(t *testing.T) {
	repoRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)

	wfDir := filepath.Join(repoRoot, "workflows", "my-wf")
	_ = os.MkdirAll(wfDir, 0o755)

	// Create .secrets.yaml with nested map (not $shared)
	yamlContent := "slack:\n  webhook_url: \"https://hooks.slack.com/T00\"\n"
	_ = os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(yamlContent), 0o644)

	_, err := buildSecretManifest(wfDir, "my-wf", "default")
	if err == nil {
		t.Fatal("expected error for nested secret value")
	}
	if !strings.Contains(err.Error(), "direct value") {
		t.Errorf("expected 'direct value' error, got: %v", err)
	}
}

func TestBuildSecretManifestNoSecrets(t *testing.T) {
	dir := t.TempDir()

	m, err := buildSecretManifest(dir, "test-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Error("expected nil manifest when no secrets exist")
	}
}

func TestBuildSecretManifestEmptyYAML(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte("{}\n"), 0o644)

	m, err := buildSecretManifest(dir, "test-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Error("expected nil manifest for empty YAML")
	}
}

func TestBuildSecretManifestInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte(":::not valid:::[[["), 0o644)

	_, err := buildSecretManifest(dir, "test-wf", "default")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing secrets YAML") {
		t.Errorf("expected 'parsing secrets YAML' error, got: %v", err)
	}
}

func TestBuildSecretManifestMissingSharedSecret(t *testing.T) {
	repoRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoRoot, ".secrets"), 0o755)

	wfDir := filepath.Join(repoRoot, "workflows", "my-wf")
	_ = os.MkdirAll(wfDir, 0o755)

	// Reference a shared secret that doesn't exist
	yamlContent := "missing: $shared.nonexistent\n"
	_ = os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(yamlContent), 0o644)

	_, err := buildSecretManifest(wfDir, "my-wf", "default")
	if err == nil {
		t.Fatal("expected error for missing shared secret")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestBuildSecretManifestIgnoresPerWorkflowSecretsDir(t *testing.T) {
	repoRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)

	wfDir := filepath.Join(repoRoot, "workflows", "my-wf")
	_ = os.MkdirAll(wfDir, 0o755)

	// Create per-workflow .secrets/ directory (old pattern)
	secretsDir := filepath.Join(wfDir, ".secrets")
	_ = os.MkdirAll(secretsDir, 0o755)
	_ = os.WriteFile(filepath.Join(secretsDir, "token"), []byte("direct-value"), 0o644)

	// No .secrets.yaml — should return nil, NOT read from .secrets/ dir
	m, err := buildSecretManifest(wfDir, "my-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Error("expected nil manifest — per-workflow .secrets/ directory should be ignored")
	}
}

func TestBuildSecretManifestSecretName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repoRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	sharedDir := filepath.Join(repoRoot, ".secrets")
	_ = os.MkdirAll(sharedDir, 0o755)
	_ = os.WriteFile(filepath.Join(sharedDir, "token"), []byte(`{"value":"abc"}`), 0o644)

	wfDir := filepath.Join(repoRoot, "workflows", "my-workflow")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte("token: $shared.token\n"), 0o644)

	m, err := buildSecretManifest(wfDir, "my-workflow", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.Name != "my-workflow-secrets" {
		t.Errorf("expected secret name my-workflow-secrets, got %s", m.Name)
	}
	if !strings.Contains(m.Content, "name: my-workflow-secrets") {
		t.Error("expected manifest to contain name: my-workflow-secrets")
	}
}
