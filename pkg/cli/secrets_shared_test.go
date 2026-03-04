package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Phase 5: Centralized $shared secrets e2e tests ---
// These tests verify the end-to-end flow of deploying workflows that use
// $shared.<name> references resolved from a repo-root .secrets/ directory.

// TestSharedSecretsE2E_BuildSecretManifestResolvesSharedRef verifies that
// buildSecretManifest resolves $shared references when building the K8s secret.
// This is the core deploy-time integration.
func TestSharedSecretsE2E_BuildSecretManifestResolvesSharedRef(t *testing.T) {
	// Set up fake repo structure: repo root + workflow subdir
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)

	sharedDir := filepath.Join(repoRoot, ".secrets")
	os.MkdirAll(sharedDir, 0o755)
	// Shared secret as JSON
	os.WriteFile(filepath.Join(sharedDir, "github"), []byte(`{"token":"ghp_test123"}`), 0o644)
	// Shared secret as plain text
	os.WriteFile(filepath.Join(sharedDir, "hn-api-key"), []byte("hn_test_key\n"), 0o644)

	wfDir := filepath.Join(repoRoot, "example-workflows", "test-wf")
	os.MkdirAll(wfDir, 0o755)

	// Per-workflow .secrets.yaml using $shared references
	os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(`github: $shared.github
hn_api_key: $shared.hn-api-key
`), 0o644)

	manifest, err := buildSecretManifest(wfDir, "test-wf", "test-ns")
	if err != nil {
		t.Fatalf("unexpected error building secret manifest: %v", err)
	}
	if manifest == nil {
		t.Fatal("expected non-nil manifest when .secrets.yaml with $shared refs exists")
	}
	if manifest.Kind != "Secret" {
		t.Errorf("expected kind Secret, got %s", manifest.Kind)
	}
	// JSON-resolved secret should appear in stringData
	if !strings.Contains(manifest.Content, "token") {
		t.Error("expected resolved github secret to contain 'token' field")
	}
	// Plain text secret should be trimmed and present
	if !strings.Contains(manifest.Content, "hn_test_key") {
		t.Error("expected resolved hn-api-key secret value 'hn_test_key'")
	}
	// $shared. references must not appear verbatim in the manifest
	if strings.Contains(manifest.Content, "$shared.") {
		t.Error("expected all $shared. references to be resolved, not left as-is")
	}
}

// TestSharedSecretsE2E_MissingSharedSecretErrors verifies that deploying
// a workflow whose .secrets.yaml references a missing shared secret returns
// a clear error with the secret name.
func TestSharedSecretsE2E_MissingSharedSecretErrors(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repoRoot, ".secrets"), 0o755)
	// Do NOT create the "github" shared secret

	wfDir := filepath.Join(repoRoot, "example-workflows", "missing-wf")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(`github: $shared.github
`), 0o644)

	_, err := buildSecretManifest(wfDir, "missing-wf", "default")
	if err == nil {
		t.Fatal("expected error when referenced shared secret does not exist")
	}
	if !strings.Contains(err.Error(), "github") {
		t.Errorf("expected error to name the missing secret 'github', got: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to say 'not found', got: %v", err)
	}
}

// TestSharedSecretsE2E_SharedSecretsDirContainsMultipleSecrets verifies that
// multiple $shared references in one .secrets.yaml are all resolved correctly.
func TestSharedSecretsE2E_SharedSecretsDirContainsMultipleSecrets(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	sharedDir := filepath.Join(repoRoot, ".secrets")
	os.MkdirAll(sharedDir, 0o755)
	os.WriteFile(filepath.Join(sharedDir, "db"), []byte(`{"host":"db.internal","password":"secret"}`), 0o644)
	os.WriteFile(filepath.Join(sharedDir, "slack"), []byte(`{"webhook":"https://hooks.slack.com/test"}`), 0o644)
	os.WriteFile(filepath.Join(sharedDir, "api-token"), []byte("tok_plain\n"), 0o644)

	wfDir := filepath.Join(repoRoot, "workflows", "multi-secret")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(`db: $shared.db
slack: $shared.slack
api_token: $shared.api-token
`), 0o644)

	manifest, err := buildSecretManifest(wfDir, "multi-secret", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}
	// All three resolved values should appear
	if !strings.Contains(manifest.Content, "db.internal") {
		t.Error("expected db secret to be resolved with host=db.internal")
	}
	if !strings.Contains(manifest.Content, "hooks.slack.com") {
		t.Error("expected slack secret to be resolved with webhook URL")
	}
	if !strings.Contains(manifest.Content, "tok_plain") {
		t.Error("expected api_token to be resolved with plain text value")
	}
}

// TestSharedSecretsE2E_PathTraversalInSharedRefErrors verifies that a
// $shared.<name> with path traversal characters is rejected with an error.
func TestSharedSecretsE2E_PathTraversalInSharedRefErrors(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repoRoot, ".secrets"), 0o755)

	wfDir := filepath.Join(repoRoot, "workflows", "evil-wf")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(`steal: $shared.../../etc/passwd
`), 0o644)

	_, err := buildSecretManifest(wfDir, "evil-wf", "default")
	if err == nil {
		t.Fatal("expected error for path traversal in $shared reference")
	}
	if !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("expected 'invalid path' error for traversal attempt, got: %v", err)
	}
}

// TestSharedSecretsE2E_WorkflowWithNoSecretsSucceeds verifies that a workflow
// with no .secrets.yaml and no .secrets/ returns nil without error.
func TestSharedSecretsE2E_WorkflowWithNoSecretsSucceeds(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	wfDir := filepath.Join(repoRoot, "workflows", "no-secrets")
	os.MkdirAll(wfDir, 0o755)

	manifest, err := buildSecretManifest(wfDir, "no-secrets", "default")
	if err != nil {
		t.Fatalf("unexpected error when no secrets: %v", err)
	}
	if manifest != nil {
		t.Error("expected nil manifest when workflow has no secrets")
	}
}

// TestSharedSecretsE2E_RepoRootSecretsCheck verifies that readProvisionedSecrets
// reports shared secrets as provisioned when the .secrets/ dir at repo root exists.
func TestSharedSecretsE2E_RepoRootSecretsCheck(t *testing.T) {
	repoRoot := t.TempDir()
	os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755)
	sharedDir := filepath.Join(repoRoot, ".secrets")
	os.MkdirAll(sharedDir, 0o755)
	os.WriteFile(filepath.Join(sharedDir, "github"), []byte("token"), 0o644)

	wfDir := filepath.Join(repoRoot, "workflows", "check-wf")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, ".secrets.yaml"), []byte(`github: $shared.github
`), 0o644)

	// readProvisionedSecrets should see the local .secrets.yaml as provisioned
	provisioned, source := readProvisionedSecrets(wfDir)
	if source != ".secrets.yaml" {
		t.Errorf("expected source .secrets.yaml, got %q", source)
	}
	if !provisioned["github"] {
		t.Error("expected 'github' to appear in provisioned secrets")
	}
}
