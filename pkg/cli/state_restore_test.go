package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// seedRepoWith3CommitsTouchingTentacle creates a git repo with 3 commits, each
// modifying a file inside the named tentacle directory. The repo has a git user
// configured so commits don't fail in CI.
func seedRepoWith3CommitsTouchingTentacle(t *testing.T, repoDir, tentacleName string) {
	t.Helper()

	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	mustGit("init", "-b", "main")
	mustGit("config", "user.email", "test@example.com")
	mustGit("config", "user.name", "Test User")

	tentacleDir := filepath.Join(repoDir, tentacleName)
	if err := os.MkdirAll(tentacleDir, 0o755); err != nil {
		t.Fatalf("mkdir tentacle: %v", err)
	}

	for i, content := range []string{"v1 content", "v2 content", "v3 content"} {
		filePath := filepath.Join(tentacleDir, "workflow.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("write v%d: %v", i+1, err)
		}
		mustGit("add", tentacleName)
		mustGit("commit", "-m", "chore: v"+[]string{"1", "2", "3"}[i])
	}
}

// gitRevParse returns the resolved SHA for the given ref in the given repo.
func gitRevParse(t *testing.T, repoDir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// gitTree returns the tree object SHA for path under the given ref.
func gitTree(t *testing.T, repoDir, ref, path string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref+":"+path)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s:%s: %v", ref, path, err)
	}
	return strings.TrimSpace(string(out))
}

// gitRevReachable returns true if sha appears in the git log (i.e. is reachable from HEAD).
func gitRevReachable(t *testing.T, repoDir, sha string) bool {
	t.Helper()
	cmd := exec.Command("git", "log", "--format=%H")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == sha {
			return true
		}
	}
	return false
}

func TestForwardRevertProducesNewCommitMatchingTargetTree(t *testing.T) {
	repo := t.TempDir()
	seedRepoWith3CommitsTouchingTentacle(t, repo, "ai-news-digest")

	// Get the SHA of the middle commit (call it v2).
	v2SHA := gitRevParse(t, repo, "HEAD~1")
	v3SHA := gitRevParse(t, repo, "HEAD")

	// Forward-revert onto v2's tree.
	newSHA, err := forwardRevert(repo, "ai-news-digest", v2SHA)
	if err != nil {
		t.Fatalf("forwardRevert: %v", err)
	}

	// Assert: HEAD is no longer v3, and the tree at HEAD matches v2's tree.
	if gitRevParse(t, repo, "HEAD") == v3SHA {
		t.Fatal("HEAD did not advance — forward-revert was a no-op")
	}
	if gitRevParse(t, repo, "HEAD") != newSHA {
		t.Fatal("HEAD does not match returned SHA")
	}
	headTree := gitTree(t, repo, "HEAD", "ai-news-digest")
	v2Tree := gitTree(t, repo, v2SHA, "ai-news-digest")
	if headTree != v2Tree {
		t.Fatalf("forward-revert tree mismatch:\nHEAD:  %s\nv2:    %s", headTree, v2Tree)
	}
}

func TestForwardRevertOnHEADIsIdempotentNoOp(t *testing.T) {
	repo := t.TempDir()
	seedRepoWith3CommitsTouchingTentacle(t, repo, "ai-news-digest")
	headSHA := gitRevParse(t, repo, "HEAD")

	newSHA, err := forwardRevert(repo, "ai-news-digest", headSHA)
	if err != nil {
		t.Fatalf("forwardRevert: %v", err)
	}

	// Idempotent: HEAD already matches target tree, no new commit.
	if newSHA != headSHA {
		t.Fatalf("expected no-op (HEAD SHA unchanged), got new SHA %s", newSHA)
	}
}

func TestForwardRevertPreservesV3InHistory(t *testing.T) {
	repo := t.TempDir()
	seedRepoWith3CommitsTouchingTentacle(t, repo, "ai-news-digest")
	v3SHA := gitRevParse(t, repo, "HEAD")
	v2SHA := gitRevParse(t, repo, "HEAD~1")

	if _, err := forwardRevert(repo, "ai-news-digest", v2SHA); err != nil {
		t.Fatalf("forwardRevert: %v", err)
	}

	// v3 must still be reachable in git history (not lost via reset).
	if !gitRevReachable(t, repo, v3SHA) {
		t.Fatal("v3 SHA was lost — forward-revert used hard reset (BUG)")
	}
}
