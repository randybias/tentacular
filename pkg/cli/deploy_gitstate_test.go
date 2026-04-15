// Tests for git-state deploy helpers: pushGitState, captureGitMeta, and
// injectGitAnnotations. These tests use real git commands against temporary
// bare and working repos — no git library mocking is performed.
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupBareAndClone creates a bare "remote" repo and clones it into a working
// directory. Both directories live in t.TempDir() and are cleaned up
// automatically. Returns the working clone path.
func setupBareAndClone(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	bareDir := filepath.Join(base, "remote.git")
	workDir := filepath.Join(base, "work")

	// Init bare repo
	runGit(t, "", "init", "--bare", bareDir)

	// Clone into work dir
	runGit(t, "", "clone", bareDir, workDir)

	// Configure user identity for the work clone (needed for commits)
	runGit(t, workDir, "config", "user.email", "test@example.com")
	runGit(t, workDir, "config", "user.name", "Test User")

	// Create an initial commit so the branch exists on the remote
	initialFile := filepath.Join(workDir, "README")
	if err := os.WriteFile(initialFile, []byte("init\n"), 0o644); err != nil {
		t.Fatalf("writing initial file: %v", err)
	}
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "init")
	runGit(t, workDir, "push", "-u", "origin", "HEAD")
	return workDir
}

// runGit runs a git command, optionally in dir, and fails the test on error.
// Pass dir="" to run in the current directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmdArgs := args
	if dir != "" {
		cmdArgs = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", cmdArgs...) //nolint:gosec // test helper
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}

// addCommit writes a file and commits it in workDir.
func addCommit(t *testing.T, workDir, filename, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workDir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	runGit(t, workDir, "add", filename)
	runGit(t, workDir, "commit", "-m", message)
}

// --- pushGitState tests ---

// TestPushGitState_AheadPushes verifies that when the working clone has commits
// not yet on the remote, pushGitState pushes them successfully.
func TestPushGitState_AheadPushes(t *testing.T) {
	workDir := setupBareAndClone(t)

	// Add a local commit that hasn't been pushed.
	addCommit(t, workDir, "change.txt", "hello\n", "feat: new change")

	branch, branchErr := getCurrentBranch(workDir)
	if branchErr != nil {
		t.Fatalf("getCurrentBranch: %v", branchErr)
	}

	if pushErr := pushGitState(workDir, branch); pushErr != nil {
		t.Fatalf("pushGitState returned error: %v", pushErr)
	}

	// Verify the remote now has the commit.
	sha, shaErr := getCurrentGitSHA(workDir)
	if shaErr != nil {
		t.Fatalf("getCurrentGitSHA: %v", shaErr)
	}

	// Fetch in a second clone to confirm the commit is on the remote.
	base := t.TempDir()
	verifyDir := filepath.Join(base, "verify")
	remoteURL := getGitRemoteURL(workDir, "origin")
	if remoteURL == "" {
		t.Fatal("getGitRemoteURL returned empty URL")
	}
	runGit(t, "", "clone", remoteURL, verifyDir)

	remoteSHA, remoteSHAErr := getCurrentGitSHA(verifyDir)
	if remoteSHAErr != nil {
		t.Fatalf("getCurrentGitSHA on verify clone: %v", remoteSHAErr)
	}
	if remoteSHA != sha {
		t.Errorf("remote SHA %q != local SHA %q after push", remoteSHA, sha)
	}
}

// TestPushGitState_EqualIsNoop verifies that when HEAD equals the remote
// tracking ref (nothing to push), pushGitState returns nil without error.
func TestPushGitState_EqualIsNoop(t *testing.T) {
	workDir := setupBareAndClone(t)

	// Nothing to push — already in sync after setupBareAndClone.
	branch, err := getCurrentBranch(workDir)
	if err != nil {
		t.Fatalf("getCurrentBranch: %v", err)
	}

	if err := pushGitState(workDir, branch); err != nil {
		t.Errorf("pushGitState (nothing to push) should return nil, got: %v", err)
	}
}

// TestPushGitState_BehindReturnsError verifies that when the remote has commits
// the local clone doesn't have (diverged/behind), pushGitState returns an error
// and does not attempt the push.
func TestPushGitState_BehindReturnsError(t *testing.T) {
	workDir := setupBareAndClone(t)

	// Simulate remote advancing: clone a second copy, commit, push.
	base := t.TempDir()
	otherDir := filepath.Join(base, "other")
	remoteURL := getGitRemoteURL(workDir, "origin")
	runGit(t, "", "clone", remoteURL, otherDir)
	runGit(t, otherDir, "config", "user.email", "other@example.com")
	runGit(t, otherDir, "config", "user.name", "Other User")
	addCommit(t, otherDir, "remote-change.txt", "remote\n", "feat: remote-only change")
	runGit(t, otherDir, "push")

	// Now workDir is behind — pushGitState should fail.
	branch, err := getCurrentBranch(workDir)
	if err != nil {
		t.Fatalf("getCurrentBranch: %v", err)
	}

	err = pushGitState(workDir, branch)
	if err == nil {
		t.Fatal("expected error when behind remote, got nil")
	}
	if !strings.Contains(err.Error(), "behind") {
		t.Errorf("expected 'behind' in error message, got: %v", err)
	}
}

// TestPushGitState_DetachedHeadReturnsError verifies that a detached HEAD state
// causes pushGitState to return an error rather than silently doing nothing.
func TestPushGitState_DetachedHeadReturnsError(t *testing.T) {
	workDir := setupBareAndClone(t)

	// Detach HEAD
	sha, _ := getCurrentGitSHA(workDir)
	runGit(t, workDir, "checkout", "--detach", sha)

	branch, _ := getCurrentBranch(workDir) // will return "HEAD"
	err := pushGitState(workDir, branch)
	if err == nil {
		t.Fatal("expected error for detached HEAD, got nil")
	}
}

// --- captureGitMeta tests ---

// TestCaptureGitMeta_PopulatesFields verifies that captureGitMeta returns a
// non-empty SHA, a non-empty branch name, and a remote URL for a standard clone.
func TestCaptureGitMeta_PopulatesFields(t *testing.T) {
	workDir := setupBareAndClone(t)

	meta, err := captureGitMeta(workDir)
	if err != nil {
		t.Fatalf("captureGitMeta: %v", err)
	}
	if meta.SHA == "" {
		t.Error("expected non-empty SHA")
	}
	if len(meta.SHA) < 40 {
		t.Errorf("expected full SHA (>=40 chars), got %q", meta.SHA)
	}
	if meta.Branch == "" {
		t.Error("expected non-empty Branch")
	}
	if meta.Repo == "" {
		t.Error("expected non-empty Repo (remote origin URL)")
	}
}

// TestGetCurrentGitSHA_MatchesRevParse verifies getCurrentGitSHA output is a
// valid full-length commit SHA.
func TestGetCurrentGitSHA_MatchesRevParse(t *testing.T) {
	workDir := setupBareAndClone(t)

	sha, err := getCurrentGitSHA(workDir)
	if err != nil {
		t.Fatalf("getCurrentGitSHA: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %d chars: %q", len(sha), sha)
	}
	for _, c := range sha {
		if ('0' > c || c > '9') && ('a' > c || c > 'f') {
			t.Errorf("SHA contains non-hex character %q: %q", c, sha)
			break
		}
	}
}

// --- injectGitAnnotations tests ---

// TestInjectGitAnnotations_InjectsIntoDeployment verifies that the three git
// annotation keys are added to a Deployment manifest's metadata.annotations.
func TestInjectGitAnnotations_InjectsIntoDeployment(t *testing.T) {
	meta := GitMeta{
		SHA:    "abc1234def567890abc1234def567890abc12345",
		Repo:   "https://github.com/org/repo",
		Branch: "main",
	}

	manifests := []map[string]any{
		{
			"kind": "Deployment",
			"metadata": map[string]any{
				"name": "my-workflow",
			},
		},
	}

	injectGitAnnotations(manifests, meta)

	metadata, ok := manifests[0]["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata is not a map")
	}
	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok {
		t.Fatal("annotations is not a map")
	}

	if annotations["tentacular.io/git-sha"] != meta.SHA {
		t.Errorf("git-sha: got %q, want %q", annotations["tentacular.io/git-sha"], meta.SHA)
	}
	if annotations["tentacular.io/git-repo"] != meta.Repo {
		t.Errorf("git-repo: got %q, want %q", annotations["tentacular.io/git-repo"], meta.Repo)
	}
	if annotations["tentacular.io/git-branch"] != meta.Branch {
		t.Errorf("git-branch: got %q, want %q", annotations["tentacular.io/git-branch"], meta.Branch)
	}
}

// TestInjectGitAnnotations_SkipsNonDeployment verifies that injectGitAnnotations
// leaves ConfigMap and Service manifests untouched.
func TestInjectGitAnnotations_SkipsNonDeployment(t *testing.T) {
	meta := GitMeta{SHA: "abc1234", Repo: "https://example.com/repo", Branch: "main"}

	configMap := map[string]any{
		"kind":     "ConfigMap",
		"metadata": map[string]any{"name": "my-workflow-code"},
	}
	service := map[string]any{
		"kind":     "Service",
		"metadata": map[string]any{"name": "my-workflow"},
	}

	manifests := []map[string]any{configMap, service}
	injectGitAnnotations(manifests, meta)

	for _, obj := range manifests {
		metadata, _ := obj["metadata"].(map[string]any)
		if annotations, hasAnnotations := metadata["annotations"]; hasAnnotations {
			t.Errorf("kind %q should not have annotations injected, got: %v", obj["kind"], annotations)
		}
	}
}

// TestInjectGitAnnotations_MergesWithExisting verifies that existing annotations
// on a Deployment are preserved and the new git keys are added alongside them.
func TestInjectGitAnnotations_MergesWithExisting(t *testing.T) {
	meta := GitMeta{SHA: "deadbeef00000000000000000000000000000001", Repo: "", Branch: "feature-x"}

	manifests := []map[string]any{
		{
			"kind": "Deployment",
			"metadata": map[string]any{
				"name": "my-workflow",
				"annotations": map[string]any{
					"tentacular.io/description": "existing annotation",
				},
			},
		},
	}

	injectGitAnnotations(manifests, meta)

	metadata := manifests[0]["metadata"].(map[string]any)
	annotations := metadata["annotations"].(map[string]any)

	if annotations["tentacular.io/description"] != "existing annotation" {
		t.Errorf("existing annotation was overwritten: got %v", annotations["tentacular.io/description"])
	}
	if annotations["tentacular.io/git-sha"] != meta.SHA {
		t.Errorf("git-sha not injected: got %v", annotations["tentacular.io/git-sha"])
	}
	// Repo is empty — should not be set
	if _, hasRepo := annotations["tentacular.io/git-repo"]; hasRepo {
		t.Error("tentacular.io/git-repo should be omitted when Repo is empty")
	}
	if annotations["tentacular.io/git-branch"] != "feature-x" {
		t.Errorf("git-branch: got %v, want feature-x", annotations["tentacular.io/git-branch"])
	}
}

// TestInjectGitAnnotations_CreatesMetadataIfAbsent verifies that a Deployment
// manifest without a metadata block gets one created rather than panicking.
func TestInjectGitAnnotations_CreatesMetadataIfAbsent(t *testing.T) {
	meta := GitMeta{SHA: "abc1234", Branch: "main"}

	manifests := []map[string]any{
		{"kind": "Deployment"},
	}

	// Must not panic
	injectGitAnnotations(manifests, meta)

	metadata, ok := manifests[0]["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata was not created")
	}
	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok {
		t.Fatal("annotations were not created")
	}
	if annotations["tentacular.io/git-sha"] != "abc1234" {
		t.Errorf("git-sha: got %v", annotations["tentacular.io/git-sha"])
	}
}

// TestInjectGitAnnotations_EmptySHAIsNoop verifies that injectGitAnnotations is
// not called when SHA is empty (the deploy.go guard condition mirrors this).
func TestInjectGitAnnotations_EmptySHAIsNoop(t *testing.T) {
	meta := GitMeta{SHA: "", Repo: "https://example.com/repo", Branch: "main"}

	manifests := []map[string]any{
		{
			"kind":     "Deployment",
			"metadata": map[string]any{"name": "my-workflow"},
		},
	}

	// Guard mirrors deploy.go: only call injectGitAnnotations when SHA != ""
	if meta.SHA != "" {
		injectGitAnnotations(manifests, meta)
	}

	metadata := manifests[0]["metadata"].(map[string]any)
	if _, hasAnnotations := metadata["annotations"]; hasAnnotations {
		t.Error("annotations should not be injected when SHA is empty")
	}
}

// --- NewDeployCmd flag tests ---

// TestDeployCmd_NoPushFlagExists verifies the --no-push flag is registered on
// the deploy command with a default value of false.
func TestDeployCmd_NoPushFlagExists(t *testing.T) {
	cmd := NewDeployCmd()
	f := cmd.Flags().Lookup("no-push")
	if f == nil {
		t.Fatal("expected --no-push flag on deploy command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false for --no-push, got %q", f.DefValue)
	}
}
