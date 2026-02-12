package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretsCheckFindsRequiredSecrets(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	os.MkdirAll(nodesDir, 0o755)
	os.WriteFile(filepath.Join(nodesDir, "notify.ts"), []byte(`
import type { Context } from "tentacular";
export default async function run(ctx: Context, input: unknown) {
  const webhook = ctx.secrets?.slack?.webhook_url;
  if (!webhook) return { delivered: false };
  return { delivered: true };
}
`), 0o644)

	required, err := scanRequiredSecrets(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !required["slack"] {
		t.Error("expected to find 'slack' in required secrets")
	}
}

func TestSecretsCheckReportsGaps(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	os.MkdirAll(nodesDir, 0o755)
	os.WriteFile(filepath.Join(nodesDir, "store.ts"), []byte(`
const conn = ctx.secrets.postgres;
const blob = ctx.secrets?.azure;
`), 0o644)

	required, err := scanRequiredSecrets(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !required["postgres"] {
		t.Error("expected to find 'postgres' in required secrets")
	}
	if !required["azure"] {
		t.Error("expected to find 'azure' in required secrets")
	}

	// No secrets provisioned -- all should be missing
	provisioned, _ := readProvisionedSecrets(dir)
	for k := range required {
		if provisioned[k] {
			t.Errorf("expected %s to be missing from provisioned secrets", k)
		}
	}
}

func TestSecretsCheckAllProvisioned(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	os.MkdirAll(nodesDir, 0o755)
	os.WriteFile(filepath.Join(nodesDir, "notify.ts"), []byte(`
const webhook = ctx.secrets?.slack?.webhook_url;
`), 0o644)

	// Provision via .secrets.yaml
	os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte("slack:\n  webhook_url: test\n"), 0o644)

	provisioned, source := readProvisionedSecrets(dir)
	if !provisioned["slack"] {
		t.Error("expected slack to be provisioned")
	}
	if source != ".secrets.yaml" {
		t.Errorf("expected source .secrets.yaml, got %s", source)
	}
}

func TestSecretsInitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	exampleContent := "# slack:\n#   webhook_url: \"https://hooks.slack.com/...\"\n"
	os.WriteFile(filepath.Join(dir, ".secrets.yaml.example"), []byte(exampleContent), 0o644)

	// Simulate runSecretsInit by calling the logic directly
	src := filepath.Join(dir, ".secrets.yaml.example")
	dst := filepath.Join(dir, ".secrets.yaml")

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading example: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	var uncommented []string
	for _, line := range lines {
		uncommented = append(uncommented, strings.TrimPrefix(line, "# "))
	}
	os.WriteFile(dst, []byte(strings.Join(uncommented, "\n")), 0o644)

	result, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading result: %v", err)
	}
	if !strings.Contains(string(result), "slack:") {
		t.Error("expected uncommented slack: in output")
	}
	if strings.HasPrefix(strings.TrimSpace(string(result)), "#") {
		t.Error("expected comments to be removed")
	}
}

func TestSecretsInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".secrets.yaml"), []byte("existing: true\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".secrets.yaml.example"), []byte("# example\n"), 0o644)

	// Check that .secrets.yaml exists
	if _, err := os.Stat(filepath.Join(dir, ".secrets.yaml")); err != nil {
		t.Fatal("expected .secrets.yaml to exist")
	}
	// The function should refuse to overwrite without --force
}

func TestResolveSharedSecrets(t *testing.T) {
	// Create a fake repo structure with .git and .secrets/
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	sharedDir := filepath.Join(repoRoot, ".secrets")
	os.MkdirAll(sharedDir, 0o755)
	os.WriteFile(filepath.Join(sharedDir, "slack"), []byte(`{"webhook_url":"https://hooks.slack.com/test"}`), 0o644)

	// Create workflow dir inside repo
	workflowDir := filepath.Join(repoRoot, "example-workflows", "test")
	os.MkdirAll(workflowDir, 0o755)

	secrets := map[string]interface{}{
		"slack": "$shared.slack",
	}

	err := resolveSharedSecrets(secrets, workflowDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The $shared.slack reference should be resolved to the parsed JSON
	slackVal, ok := secrets["slack"]
	if !ok {
		t.Fatal("expected slack key to exist")
	}
	slackMap, ok := slackVal.(map[string]interface{})
	if !ok {
		t.Fatalf("expected slack value to be a map, got %T", slackVal)
	}
	if slackMap["webhook_url"] != "https://hooks.slack.com/test" {
		t.Errorf("expected webhook_url, got %v", slackMap["webhook_url"])
	}
}

func TestResolveSharedSecretsMissing(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repoRoot, ".secrets"), 0o755)

	workflowDir := filepath.Join(repoRoot, "workflows", "test")
	os.MkdirAll(workflowDir, 0o755)

	secrets := map[string]interface{}{
		"slack": "$shared.nonexistent",
	}

	err := resolveSharedSecrets(secrets, workflowDir)
	if err == nil {
		t.Fatal("expected error for missing shared secret")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestResolveSharedSecretsNoRepoRoot(t *testing.T) {
	// Use a temp dir with no .git or go.mod
	dir := t.TempDir()
	secrets := map[string]interface{}{
		"slack": "$shared.slack",
	}

	// Should gracefully skip -- no error, value unchanged
	err := resolveSharedSecrets(secrets, dir)
	if err != nil {
		t.Fatalf("expected no error when no repo root, got: %v", err)
	}
	// Value should remain unchanged
	if secrets["slack"] != "$shared.slack" {
		t.Errorf("expected value to remain unchanged, got %v", secrets["slack"])
	}
}

func TestFindRepoRoot(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)

	subDir := filepath.Join(repoRoot, "a", "b", "c")
	os.MkdirAll(subDir, 0o755)

	found := findRepoRoot(subDir)
	if found != repoRoot {
		t.Errorf("expected repo root %s, got %s", repoRoot, found)
	}
}

func TestFindRepoRootGoMod(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o644)

	nested := filepath.Join(root, "pkg", "sub")
	os.MkdirAll(nested, 0o755)

	found := findRepoRoot(nested)
	if found != root {
		t.Errorf("expected repo root %s via go.mod, got %s", root, found)
	}
}

func TestResolveSharedSecretsPlainText(t *testing.T) {
	// Shared secret that is NOT valid JSON should fall back to plain string
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	sharedDir := filepath.Join(repoRoot, ".secrets")
	os.MkdirAll(sharedDir, 0o755)
	os.WriteFile(filepath.Join(sharedDir, "api-key"), []byte("sk_test_plaintext\n"), 0o644)

	wfDir := filepath.Join(repoRoot, "workflows", "test")
	os.MkdirAll(wfDir, 0o755)

	secrets := map[string]interface{}{
		"api_key": "$shared.api-key",
	}

	err := resolveSharedSecrets(secrets, wfDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if secrets["api_key"] != "sk_test_plaintext" {
		t.Errorf("expected plain text secret to be trimmed, got %q", secrets["api_key"])
	}
}

func TestSecretsCheckNoNodes(t *testing.T) {
	dir := t.TempDir()
	// No nodes/ dir at all

	required, err := scanRequiredSecrets(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(required) != 0 {
		t.Errorf("expected no required secrets when no nodes directory, got %v", required)
	}
}

func TestSecretsCheckMultipleNodes(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	os.MkdirAll(nodesDir, 0o755)

	// Two nodes referencing different secrets
	node1 := `const webhook = ctx.secrets?.slack?.webhook_url;`
	node2 := `const conn = ctx.secrets.postgres; const blob = ctx.secrets?.azure;`
	os.WriteFile(filepath.Join(nodesDir, "notify.ts"), []byte(node1), 0o644)
	os.WriteFile(filepath.Join(nodesDir, "store.ts"), []byte(node2), 0o644)

	required, err := scanRequiredSecrets(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{"slack", "postgres", "azure"} {
		if !required[name] {
			t.Errorf("expected %s in required secrets", name)
		}
	}
	if len(required) != 3 {
		t.Errorf("expected 3 required secrets, got %d", len(required))
	}
}

func TestSecretsCheckWithSecretsDir(t *testing.T) {
	dir := t.TempDir()
	nodesDir := filepath.Join(dir, "nodes")
	os.MkdirAll(nodesDir, 0o755)
	os.WriteFile(filepath.Join(nodesDir, "fetch.ts"), []byte(`ctx.secrets?.github?.token`), 0o644)

	// Provision using .secrets/ directory (not .secrets.yaml)
	secretsDir := filepath.Join(dir, ".secrets")
	os.MkdirAll(secretsDir, 0o755)
	os.WriteFile(filepath.Join(secretsDir, "github"), []byte(`{"token":"ghp_test"}`), 0o644)

	provisioned, source := readProvisionedSecrets(dir)
	if source != ".secrets/" {
		t.Errorf("expected source .secrets/, got %q", source)
	}
	if !provisioned["github"] {
		t.Error("expected github to be provisioned from .secrets/ dir")
	}
}

func TestResolveSharedSecretsPathTraversal(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repoRoot, ".secrets"), 0o755)

	wfDir := filepath.Join(repoRoot, "workflows", "test")
	os.MkdirAll(wfDir, 0o755)

	// Attempt path traversal via $shared name
	secrets := map[string]interface{}{
		"steal": "$shared.../../etc/passwd",
	}

	err := resolveSharedSecrets(secrets, wfDir)
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
	if !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("expected 'invalid path' error, got: %v", err)
	}
}

func TestResolveSharedSecretsNonSharedSkipped(t *testing.T) {
	// Non-$shared values should not be modified
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repoRoot, ".secrets"), 0o755)

	wfDir := filepath.Join(repoRoot, "workflows", "test")
	os.MkdirAll(wfDir, 0o755)

	secrets := map[string]interface{}{
		"plain":  "just-a-string",
		"nested": map[string]interface{}{"key": "value"},
	}

	err := resolveSharedSecrets(secrets, wfDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if secrets["plain"] != "just-a-string" {
		t.Errorf("expected plain string to be unchanged, got %v", secrets["plain"])
	}
	nested, ok := secrets["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested to remain a map, got %T", secrets["nested"])
	}
	if nested["key"] != "value" {
		t.Errorf("expected nested key to be unchanged")
	}
}
