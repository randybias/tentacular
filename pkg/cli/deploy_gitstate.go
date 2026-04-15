package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/randybias/tentacular/pkg/scaffold"
)

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

// getGitRemoteURL returns the fetch URL of the given remote (typically "origin").
// Returns an empty string if the remote is not configured or the command fails.
func getGitRemoteURL(repoPath, remote string) string {
	cmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "remote", "get-url", remote) //nolint:gosec // repoPath is config-controlled
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
	repo := getGitRemoteURL(repoPath, "origin")
	return GitMeta{SHA: sha, Branch: branch, Repo: repo}, nil
}

// pushGitState pushes the current branch to its configured remote tracking branch.
//
// It first checks whether HEAD is ahead of the remote tracking ref:
//   - If ahead: runs "git push" and returns any push error.
//   - If equal (nothing to push): returns nil immediately.
//   - If behind or diverged: returns an error — the caller must pull/rebase first.
//
// The push uses whatever git credential helper is configured on the host; no
// credentials are injected by this function.
func pushGitState(repoPath, branch string) error {
	if branch == "" || branch == "HEAD" {
		return errors.New("cannot push: not on a named branch (detached HEAD)")
	}

	// Fetch the remote tracking state so our comparison is current.
	fetchCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "fetch", "--prune") //nolint:gosec // repoPath is config-controlled
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed before push: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	// Determine whether we are ahead of, behind, or equal to the remote.
	revListCmd := exec.CommandContext( //nolint:gosec // repoPath and branch are config-controlled
		context.Background(),
		"git", "-C", repoPath,
		"rev-list", "--left-right", "--count",
		"@{u}...HEAD",
	)
	out, err := revListCmd.Output()
	if err != nil {
		// No upstream tracking branch set — push anyway to origin/<branch>.
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
		return fmt.Errorf(
			"git-state repo is %d commit(s) behind remote; pull and rebase before deploying (git -C %s pull --rebase)",
			behind, repoPath,
		)
	}

	if ahead == 0 {
		// Nothing to push — already in sync.
		return nil
	}

	// We are ahead: push.
	pushCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "push") //nolint:gosec // repoPath is config-controlled
	if pushOut, pushErr := pushCmd.CombinedOutput(); pushErr != nil {
		return fmt.Errorf("git push failed: %w\n%s", pushErr, strings.TrimSpace(string(pushOut)))
	}
	return nil
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
