package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSecretFromDirMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "github-token"), []byte("ghp_abc123"), 0644)
	os.WriteFile(filepath.Join(dir, "stripe-key"), []byte("sk_test_xyz"), 0644)

	m, err := buildSecretFromDir(dir, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "github-token") {
		t.Error("expected github-token in stringData")
	}
	if !strings.Contains(m.Content, "stripe-key") {
		t.Error("expected stripe-key in stringData")
	}
}

func TestBuildSecretFromDirSkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)
	os.WriteFile(filepath.Join(dir, "visible"), []byte("value"), 0644)

	m, err := buildSecretFromDir(dir, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if strings.Contains(m.Content, ".hidden") {
		t.Error("expected .hidden file to be skipped")
	}
	if !strings.Contains(m.Content, "visible") {
		t.Error("expected visible file to be included")
	}
}

func TestBuildSecretFromDirSkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "nested"), []byte("value"), 0644)
	os.WriteFile(filepath.Join(dir, "top-level"), []byte("value"), 0644)

	m, err := buildSecretFromDir(dir, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if strings.Contains(m.Content, "subdir") {
		t.Error("expected subdirectories to be excluded")
	}
	if !strings.Contains(m.Content, "top-level") {
		t.Error("expected top-level file to be included")
	}
}

func TestBuildSecretFromDirEmptyDir(t *testing.T) {
	dir := t.TempDir()

	m, err := buildSecretFromDir(dir, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Error("expected nil manifest for empty directory")
	}
}

func TestBuildSecretFromDirTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "token"), []byte("  abc123  \n"), 0644)

	m, err := buildSecretFromDir(dir, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "abc123") {
		t.Error("expected trimmed value in manifest")
	}
	// The value should be quoted and trimmed — no leading/trailing whitespace
	if strings.Contains(m.Content, `"  abc123`) {
		t.Error("expected whitespace to be trimmed from value")
	}
}

func TestBuildSecretFromYAMLValid(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, ".secrets.yaml")
	os.WriteFile(yamlFile, []byte("github_token: ghp_abc123\nstripe_key: sk_test\n"), 0644)

	m, err := buildSecretFromYAML(yamlFile, "my-secrets", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "kind: Secret") {
		t.Error("expected kind: Secret")
	}
	if !strings.Contains(m.Content, "name: my-secrets") {
		t.Error("expected name: my-secrets")
	}
	if !strings.Contains(m.Content, "namespace: staging") {
		t.Error("expected namespace: staging")
	}
	if !strings.Contains(m.Content, "stringData:") {
		t.Error("expected stringData section")
	}
	if !strings.Contains(m.Content, "github_token") {
		t.Error("expected github_token in stringData")
	}
}

func TestBuildSecretFromYAMLEmpty(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, ".secrets.yaml")
	os.WriteFile(yamlFile, []byte("{}\n"), 0644)

	m, err := buildSecretFromYAML(yamlFile, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Error("expected nil manifest for empty YAML")
	}
}

func TestBuildSecretFromYAMLInvalid(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, ".secrets.yaml")
	os.WriteFile(yamlFile, []byte(":::not valid yaml:::[[["), 0644)

	_, err := buildSecretFromYAML(yamlFile, "test-secrets", "default")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing secrets YAML") {
		t.Errorf("expected error containing 'parsing secrets YAML', got: %v", err)
	}
}

func TestBuildSecretFromYAMLNested(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, ".secrets.yaml")
	content := `slack:
  webhook_url: "https://hooks.slack.com/services/T00/B00/xxx"
`
	os.WriteFile(yamlFile, []byte(content), 0644)

	m, err := buildSecretFromYAML(yamlFile, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "kind: Secret") {
		t.Error("expected kind: Secret")
	}
	if !strings.Contains(m.Content, "stringData:") {
		t.Error("expected stringData section")
	}
	// Nested map should be JSON-serialized
	if !strings.Contains(m.Content, "slack") {
		t.Error("expected slack key in stringData")
	}
	if !strings.Contains(m.Content, "webhook_url") {
		t.Error("expected webhook_url in JSON-serialized value")
	}
}

func TestBuildSecretFromYAMLMixed(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, ".secrets.yaml")
	content := `api_key: sk_test_123
slack:
  webhook_url: "https://hooks.slack.com/services/T00/B00/xxx"
`
	os.WriteFile(yamlFile, []byte(content), 0644)

	m, err := buildSecretFromYAML(yamlFile, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	// Flat string should still work
	if !strings.Contains(m.Content, "api_key") {
		t.Error("expected api_key in stringData")
	}
	// Nested map should also be present
	if !strings.Contains(m.Content, "slack") {
		t.Error("expected slack key in stringData")
	}
}

func TestBuildSecretFromYAMLDeeplyNested(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, ".secrets.yaml")
	content := `azure:
  storage:
    account_name: "myaccount"
    sas_token: "sv=2023"
`
	os.WriteFile(yamlFile, []byte(content), 0644)

	m, err := buildSecretFromYAML(yamlFile, "test-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "azure") {
		t.Error("expected azure key in stringData")
	}
	// Deep nesting should be JSON-serialized
	if !strings.Contains(m.Content, "account_name") {
		t.Error("expected account_name in JSON-serialized nested value")
	}
	if !strings.Contains(m.Content, "sas_token") {
		t.Error("expected sas_token in JSON-serialized nested value")
	}
}

func TestBuildSecretManifestPrefersDir(t *testing.T) {
	dir := t.TempDir()

	// Create both .secrets/ dir and .secrets.yaml
	secretsDir := filepath.Join(dir, ".secrets")
	os.MkdirAll(secretsDir, 0755)
	os.WriteFile(filepath.Join(secretsDir, "from-dir"), []byte("dir-value"), 0644)
	os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte("from_yaml: yaml-value\n"), 0644)

	m, err := buildSecretManifest(dir, "test-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "from-dir") {
		t.Error("expected dir secret to be used")
	}
	if strings.Contains(m.Content, "from_yaml") {
		t.Error("expected YAML secret to be ignored when .secrets/ dir exists")
	}
}

func TestBuildSecretManifestFallsBackToYAML(t *testing.T) {
	dir := t.TempDir()

	// Only create .secrets.yaml, no .secrets/ dir
	os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte("api_key: sk_test\n"), 0644)

	m, err := buildSecretManifest(dir, "test-wf", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if !strings.Contains(m.Content, "api_key") {
		t.Error("expected YAML secret to be used as fallback")
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

func TestBuildSecretFromYAMLResolvesSharedSecrets(t *testing.T) {
	// Create a fake repo root with .git marker and .secrets/ directory
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	sharedDir := filepath.Join(repoRoot, ".secrets")
	os.MkdirAll(sharedDir, 0o755)
	os.WriteFile(filepath.Join(sharedDir, "slack"), []byte(`{"webhook_url":"https://hooks.slack.com/T00"}`), 0o644)

	// Create a workflow directory under the repo root
	wfDir := filepath.Join(repoRoot, "workflows", "my-wf")
	os.MkdirAll(wfDir, 0o755)

	// Create .secrets.yaml with a $shared reference
	yamlContent := "slack: $shared.slack\napi_key: sk_test_123\n"
	yamlFile := filepath.Join(wfDir, ".secrets.yaml")
	os.WriteFile(yamlFile, []byte(yamlContent), 0o644)

	m, err := buildSecretFromYAML(yamlFile, "my-wf-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	// The $shared.slack reference should have been resolved to the JSON content
	if !strings.Contains(m.Content, "webhook_url") {
		t.Error("expected shared secret to be resolved with webhook_url content")
	}
	// The plain string secret should still be present
	if !strings.Contains(m.Content, "api_key") {
		t.Error("expected api_key in stringData")
	}
	// The raw $shared. reference should NOT remain
	if strings.Contains(m.Content, "$shared.") {
		t.Error("expected $shared. reference to be resolved, not left as-is")
	}
}

func TestBuildSecretManifestSecretName(t *testing.T) {
	dir := t.TempDir()

	secretsDir := filepath.Join(dir, ".secrets")
	os.MkdirAll(secretsDir, 0755)
	os.WriteFile(filepath.Join(secretsDir, "token"), []byte("abc"), 0644)

	m, err := buildSecretManifest(dir, "my-workflow", "default")
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
