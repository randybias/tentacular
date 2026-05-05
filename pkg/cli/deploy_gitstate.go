package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/randybias/tentacular/pkg/scaffold"
)

// RebaseConflictError is returned when a git rebase fails due to merge conflicts.
// The caller must resolve the conflicts manually; the working tree is left clean
// (rebase aborted) when this error is returned.
type RebaseConflictError struct {
	Message string   // human-readable summary
	Files   []string // conflicted file paths
}

func (e *RebaseConflictError) Error() string { return e.Message }

// checkGitStateClean verifies the git-state repo has no uncommitted changes
// for the given enclave/tentacle path.
func checkGitStateClean(repoPath, enclaveName, tentacleName string) error {
	if enclaveName == "" {
		return errors.New("git-state is enabled but no enclave specified; use --enclave")
	}
	if strings.ContainsAny(enclaveName, `/\`) || strings.Contains(enclaveName, "..") {
		return fmt.Errorf("invalid enclave name %q: must not contain path separators or '..'", enclaveName)
	}
	if err := scaffold.ValidateScaffoldName(enclaveName); err != nil {
		return fmt.Errorf("invalid enclave name: %w", err)
	}
	if strings.ContainsAny(tentacleName, `/\`) || strings.Contains(tentacleName, "..") {
		return fmt.Errorf("invalid tentacle name %q: must not contain path separators or '..'", tentacleName)
	}
	subPath := fmt.Sprintf("enclaves/%s/%s/", enclaveName, tentacleName)
	cmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "status", "--porcelain", "--", subPath) //nolint:gosec // repoPath and subPath are config-controlled
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking git-state repo: %w", err)
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return fmt.Errorf("git-state repo has uncommitted changes for %s; commit before deploying", subPath)
	}
	return nil
}

// GitMeta holds git provenance metadata captured before a deploy.
type GitMeta struct {
	SHA    string // HEAD commit SHA (full)
	Repo   string // remote URL (origin)
	Branch string // current branch name
}

// getCurrentGitSHA returns the full HEAD commit SHA for the given repo.
func getCurrentGitSHA(repoPath string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "rev-parse", "HEAD") //nolint:gosec // repoPath is config-controlled
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reading git HEAD SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// getCurrentBranch returns the name of the currently checked-out branch.
// Returns "HEAD" (detached HEAD state) if no branch name is available.
func getCurrentBranch(repoPath string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD") //nolint:gosec // repoPath is config-controlled
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reading current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// getGitRemoteURL returns the fetch URL of the "origin" remote.
// Returns an empty string if origin is not configured or the command fails.
func getGitRemoteURL(repoPath string) string {
	cmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "remote", "get-url", "origin") //nolint:gosec // repoPath is config-controlled
	out, err := cmd.Output()
	if err != nil {
		// Remote may not be configured — return empty string silently.
		return ""
	}
	return strings.TrimSpace(string(out))
}

// captureGitMeta reads HEAD SHA, branch name, and origin URL from repoPath.
// Non-fatal: if any piece is unavailable (e.g. detached HEAD, no remote) the
// corresponding field is set to an empty string and no error is returned.
func captureGitMeta(repoPath string) (GitMeta, error) {
	sha, err := getCurrentGitSHA(repoPath)
	if err != nil {
		return GitMeta{}, err
	}
	branch, branchErr := getCurrentBranch(repoPath)
	if branchErr != nil {
		branch = "" // non-fatal
	}
	repo := getGitRemoteURL(repoPath)
	return GitMeta{SHA: sha, Branch: branch, Repo: repo}, nil
}

// pushGitState pushes the current branch to its configured remote tracking branch.
//
// Behavior by remote comparison state:
//   - Equal: no-op, returns nil.
//   - Ahead: pushes immediately.
//   - Behind or diverged: rebases onto origin/<branch> first, then pushes.
//     A real merge conflict aborts the rebase and returns *RebaseConflictError.
//   - No upstream set: pushes to origin/<branch> directly.
//
// Push races (non-fast-forward rejection) trigger a fetch+rebase+retry loop
// with exponential backoff (250ms, 500ms, 1s), up to 3 total attempts.
//
// The push uses whatever git credential helper is configured on the host; no
// credentials are injected by this function.
func pushGitState(repoPath, branch string) error {
	if branch == "" || branch == "HEAD" {
		return errors.New("cannot push: not on a named branch (detached HEAD)")
	}

	const maxAttempts = 3
	backoff := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second}

	for attempt := range maxAttempts {
		if attempt > 0 {
			time.Sleep(backoff[attempt-1])
		}

		// Fetch the remote tracking state so our comparison is current.
		fetchCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "fetch", "--prune") //nolint:gosec // repoPath is config-controlled
		if fetchOut, fetchErr := fetchCmd.CombinedOutput(); fetchErr != nil {
			return fmt.Errorf("git fetch failed before push: %w\n%s", fetchErr, strings.TrimSpace(string(fetchOut)))
		}

		// Determine whether we are ahead of, behind, or equal to the remote.
		revListCmd := exec.CommandContext( //nolint:gosec // repoPath and branch are config-controlled
			context.Background(),
			"git", "-C", repoPath,
			"rev-list", "--left-right", "--count",
			"@{u}...HEAD",
		)
		out, revErr := revListCmd.Output()
		if revErr != nil {
			// No upstream tracking branch set — push to origin/<branch>.
			// This handles the case where the branch has never been pushed.
			pushCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "push", "origin", branch) //nolint:gosec // repoPath and branch are config-controlled
			if pushOut, pushErr := pushCmd.CombinedOutput(); pushErr != nil {
				return fmt.Errorf("git push failed: %w\n%s", pushErr, strings.TrimSpace(string(pushOut)))
			}
			return nil
		}

		// Parse "behind\tahead" from rev-list output.
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) != 2 {
			return fmt.Errorf("unexpected rev-list output: %q", strings.TrimSpace(string(out)))
		}
		var behind, ahead int
		if _, scanErr := fmt.Sscan(parts[0], &behind); scanErr != nil {
			return fmt.Errorf("parsing rev-list behind count: %w", scanErr)
		}
		if _, scanErr := fmt.Sscan(parts[1], &ahead); scanErr != nil {
			return fmt.Errorf("parsing rev-list ahead count: %w", scanErr)
		}

		if behind > 0 {
			// Auto-rebase onto the remote branch.
			if rebaseErr := rebaseOntoRemote(repoPath, branch); rebaseErr != nil {
				return rebaseErr
			}
			// After rebase, loop back to re-fetch and re-check before pushing.
			continue
		}

		if ahead == 0 {
			// Nothing to push — already in sync.
			return nil
		}

		// We are ahead: push.
		pushCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "push") //nolint:gosec // repoPath is config-controlled
		pushOut, pushErr := pushCmd.CombinedOutput()
		if pushErr == nil {
			return nil
		}

		// If this is a non-fast-forward rejection, retry (push race).
		if isNonFastForward(string(pushOut)) && attempt < maxAttempts-1 {
			continue
		}
		return fmt.Errorf("git push failed: %w\n%s", pushErr, strings.TrimSpace(string(pushOut)))
	}

	return fmt.Errorf("git push failed after %d attempts: remote continues to reject (non-fast-forward)", maxAttempts)
}

// rebaseOntoRemote runs "git rebase origin/<branch>" in repoPath.
// On conflict it aborts the rebase and returns *RebaseConflictError.
func rebaseOntoRemote(repoPath, branch string) error {
	upstream := "origin/" + branch
	rebaseCmd := exec.CommandContext( //nolint:gosec // repoPath and branch are config-controlled
		context.Background(),
		"git", "-C", repoPath, "rebase", upstream,
	)
	rebaseOut, rebaseErr := rebaseCmd.CombinedOutput()
	if rebaseErr == nil {
		return nil
	}

	// Rebase failed — collect conflicted files and abort.
	conflictFiles := collectConflictedFiles(repoPath)

	// Abort the rebase to leave the working tree clean.
	abortCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "rebase", "--abort") //nolint:gosec // repoPath is config-controlled
	_ = abortCmd.Run()                                                                                // best-effort

	if len(conflictFiles) > 0 {
		return &RebaseConflictError{
			Files:   conflictFiles,
			Message: fmt.Sprintf("rebase conflict on %d file(s): %s; rebase aborted — resolve conflicts and redeploy", len(conflictFiles), strings.Join(conflictFiles, ", ")),
		}
	}

	// Generic rebase failure (non-conflict, e.g. network error during patch apply).
	return fmt.Errorf("git rebase %s failed: %w\n%s", upstream, rebaseErr, strings.TrimSpace(string(rebaseOut)))
}

// collectConflictedFiles returns the list of paths that are in a conflicted
// state in the working tree (UU/AA/DD markers in git status --porcelain).
func collectConflictedFiles(repoPath string) []string {
	statusCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "status", "--porcelain") //nolint:gosec // repoPath is config-controlled
	out, err := statusCmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(line) < 3 {
			continue
		}
		xy := line[:2]
		// Conflict markers: UU, AA, DD, AU, UA, DU, UD
		if strings.ContainsAny(string(xy[0]), "UAD") && strings.ContainsAny(string(xy[1]), "UAD") && xy != "  " {
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files
}

// isNonFastForward returns true when the git push output indicates a
// non-fast-forward rejection (i.e. someone pushed between our fetch and push).
func isNonFastForward(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "non-fast-forward") ||
		strings.Contains(lower, "rejected") ||
		strings.Contains(lower, "[rejected]")
}

// injectGitAnnotations merges git provenance annotations into the metadata.annotations
// block of every Deployment manifest in mcpManifests. Other manifest kinds are left
// unchanged. The function modifies mcpManifests in-place; no copy is made.
//
// Annotation keys written:
//
//	tentacular.io/git-sha    — full HEAD commit SHA
//	tentacular.io/git-repo   — remote origin URL (empty → key omitted)
//	tentacular.io/git-branch — branch name (empty → key omitted)
func injectGitAnnotations(mcpManifests []map[string]any, meta GitMeta) {
	for _, obj := range mcpManifests {
		kind, _ := obj["kind"].(string)
		if kind != "Deployment" {
			continue
		}

		// Navigate (and create if absent) metadata.annotations.
		metadata, ok := obj["metadata"].(map[string]any)
		if !ok {
			metadata = make(map[string]any)
			obj["metadata"] = metadata
		}
		annotations, ok := metadata["annotations"].(map[string]any)
		if !ok {
			annotations = make(map[string]any)
			metadata["annotations"] = annotations
		}

		annotations["tentacular.io/git-sha"] = meta.SHA
		if meta.Repo != "" {
			annotations["tentacular.io/git-repo"] = meta.Repo
		}
		if meta.Branch != "" {
			annotations["tentacular.io/git-branch"] = meta.Branch
		}
	}
}
