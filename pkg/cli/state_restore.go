package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// forwardRevert writes a new commit on the current branch whose tree at
// <tentacle> matches the tree at <targetSHA>. Returns the new HEAD SHA.
//
// If HEAD already matches targetSHA's tree at <tentacle>, no commit is
// created and the existing HEAD SHA is returned (idempotent).
//
// This is NOT a hard reset — prior commits remain reachable in history.
func forwardRevert(repoPath, tentacle, targetSHA string) (string, error) {
	headTree, err := gitTreeAt(repoPath, "HEAD", tentacle)
	if err != nil {
		return "", fmt.Errorf("read HEAD tree: %w", err)
	}
	targetTree, err := gitTreeAt(repoPath, targetSHA, tentacle)
	if err != nil {
		return "", fmt.Errorf("read target tree: %w", err)
	}
	if headTree == targetTree {
		// Already at the target tree — idempotent no-op.
		return gitResolveRef(repoPath, "HEAD")
	}

	// Check out the target SHA's content for <tentacle> only into the
	// working tree, then commit on the current branch.
	if err := gitRun(repoPath, "checkout", targetSHA, "--", tentacle); err != nil {
		return "", fmt.Errorf("checkout target tree: %w", err)
	}
	msg := fmt.Sprintf("revert: restore %s to tree at %s", tentacle, targetSHA[:8])
	if err := gitRun(repoPath, "add", tentacle); err != nil {
		return "", fmt.Errorf("stage: %w", err)
	}
	if err := gitRun(repoPath, "commit", "-m", msg); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return gitResolveRef(repoPath, "HEAD")
}

// gitTreeAt returns the tree object SHA for the given path under ref.
// e.g. "HEAD:ai-news-digest" returns the blob/tree SHA.
func gitTreeAt(repoPath, ref, path string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", ref+":"+path) //nolint:gosec // ref and path are package-controlled
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s:%s: %w", ref, path, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitResolveRef resolves a symbolic ref (e.g. "HEAD") to a full SHA.
func gitResolveRef(repoPath, ref string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", ref) //nolint:gosec // ref is package-controlled
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitRun runs a git command in repoPath, returning any error.
func gitRun(repoPath string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // args are controlled by callers in this package
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// --- state restore command ---

func newStateRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <name> <ref>",
		Short: "Restore a tentacle to a prior version and redeploy",
		Long: `Forward-revert <name>'s tree on the tentacles repo to match <ref>,
commit the change, and redeploy. Writes a new deploy event.

<ref> is any git ref (SHA, branch, HEAD~N). For idempotent re-deploy
of the current state, use HEAD.`,
		Args: cobra.ExactArgs(2),
		RunE: runStateRestore,
	}
	cmd.Flags().String("enclave", "", "Target enclave name (resolves to enclave namespace)")
	cmd.Flags().Bool("no-push", false, "Skip git push step (emergency bypass; cluster state will diverge from git)")
	return cmd
}

func runStateRestore(cmd *cobra.Command, args []string) error {
	startedAt := time.Now().UTC()

	tentacleName := args[0]
	ref := args[1]

	enclaveFlagValue, _ := cmd.Flags().GetString("enclave")
	noPush, _ := cmd.Flags().GetBool("no-push")

	enclaveName, err := resolveEnclaveName(enclaveFlagValue, "")
	if err != nil {
		return emitRestoreResult(cmd, "fail", err.Error(), startedAt)
	}

	cfg := LoadConfig()
	if !cfg.GitState.Enabled || cfg.GitState.RepoPath == "" {
		return emitRestoreResult(cmd, "fail", "git-state is not configured; run 'tntc state init --repo-path <path>' first", startedAt)
	}

	tentaclesRepo := cfg.GitState.RepoPath

	// Tentacle path within the repo: enclaves/<enclave>/<name>/
	tentaclePath := filepath.Join("enclaves", enclaveName, tentacleName)

	w := StatusWriter(cmd)

	// Step 1: Resolve ref to a full SHA.
	targetSHA, err := gitResolveRef(tentaclesRepo, ref)
	if err != nil {
		return emitRestoreResult(cmd, "fail", fmt.Sprintf("resolve ref %q: %v", ref, err), startedAt)
	}

	// Step 2: Forward-revert. New commit whose tree matches targetSHA's tree.
	_, _ = fmt.Fprintf(w, "Restoring %s to tree at %s...\n", tentacleName, targetSHA[:8])
	newSHA, err := forwardRevert(tentaclesRepo, tentaclePath, targetSHA)
	if err != nil {
		return emitRestoreResult(cmd, "fail", fmt.Sprintf("forward-revert: %v", err), startedAt)
	}

	if newSHA == targetSHA {
		_, _ = fmt.Fprintf(w, "  Already at target tree — no new commit needed\n")
	} else {
		_, _ = fmt.Fprintf(w, "  Committed restore as %s\n", newSHA[:8])
	}

	// Step 3: Push (unless --no-push).
	if noPush {
		_, _ = fmt.Fprintln(w, "WARNING: --no-push bypasses remote sync; cluster state will diverge from git")
	} else {
		branch, branchErr := getCurrentBranch(tentaclesRepo)
		if branchErr != nil {
			return emitRestoreResult(cmd, "fail", "reading git branch: "+branchErr.Error(), startedAt)
		}
		if pushErr := pushGitState(tentaclesRepo, branch); pushErr != nil {
			return emitRestoreResult(cmd, "fail", "git push failed — restore aborted: "+pushErr.Error(), startedAt)
		}
	}

	// Step 4: Deploy. Reuse the existing deploy path.
	tentacleDir := filepath.Join(tentaclesRepo, tentaclePath)

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return emitRestoreResult(cmd, "fail", err.Error(), startedAt)
	}

	enclaveNS, nsErr := resolveEnclaveNamespace(cmd, mcpClient, enclaveName)
	if nsErr != nil {
		return emitRestoreResult(cmd, "fail", nsErr.Error(), startedAt)
	}

	// Capture git meta for the new commit we just made.
	gitMeta, metaErr := captureGitMeta(tentaclesRepo)
	if metaErr != nil {
		return emitRestoreResult(cmd, "fail", "reading git metadata: "+metaErr.Error(), startedAt)
	}

	cfg2 := LoadConfig()
	imageTag := resolveDefaultEngineImage(cfg2)

	deployOpts := InternalDeployOptions{
		Namespace:    enclaveNS,
		Image:        imageTag,
		RuntimeClass: "gvisor",
		StatusOut:    w,
		GitMeta:      gitMeta,
	}

	_, _ = fmt.Fprintf(w, "Deploying restored %s to %s...\n", tentacleName, enclaveNS)
	_, deployErr := deployWorkflow(tentacleDir, deployOpts, mcpClient)
	if deployErr != nil {
		return emitRestoreResult(cmd, "fail", fmt.Sprintf("deploy failed: %v", deployErr), startedAt)
	}

	summary := fmt.Sprintf("restored %s to tree at %s, redeployed to %s", tentacleName, targetSHA[:8], enclaveNS)
	return emitRestoreResult(cmd, "pass", summary, startedAt)
}

func emitRestoreResult(cmd *cobra.Command, status, summary string, startedAt time.Time) error {
	result := CommandResult{
		Version: "1",
		Command: "state restore",
		Status:  status,
		Summary: summary,
		Hints:   []string{},
		Timing: TimingInfo{
			StartedAt:  startedAt.Format(time.RFC3339),
			DurationMs: time.Since(startedAt).Milliseconds(),
		},
	}
	if status == "fail" {
		result.Hints = append(result.Hints,
			"check that the ref exists: git -C <repo> log --oneline",
			"check that the tentacle path exists in the repo at that ref",
		)
	}

	if err := EmitResult(cmd, result, os.Stdout); err != nil {
		return err
	}
	if status == "fail" {
		return fmt.Errorf("%s", summary)
	}
	return nil
}
