package builder

import (
	"os/exec"
	"strings"
)

// deriveVersion computes the version string for a tentacle.
//
// Algorithm:
//  1. base = specVersion (from workflow.yaml). If empty, default to "0.1.0".
//  2. Try to read git state from tentacleDir.
//  3. If git is available: combine base with short SHA and dirty suffix.
//     - If specVersion != "": "<specVersion>+<shortSHA>" (or "+<shortSHA>.dirty")
//     - If specVersion == "": "0.1.0+<commitCount>.<shortSHA>" (or ".dirty")
//  4. If no git: return base as-is.
func deriveVersion(specVersion, tentacleDir string) string {
	base := specVersion
	if base == "" {
		base = "0.1.0"
	}

	shortSHA := gitCommand(tentacleDir, "rev-parse", "--short=7", "HEAD")
	if shortSHA == "" {
		return base // not a git repo or git not available
	}

	isDirty := gitCommand(tentacleDir, "status", "--porcelain") != ""
	dirtySuffix := ""
	if isDirty {
		dirtySuffix = ".dirty"
	}

	if specVersion != "" {
		return specVersion + "+" + shortSHA + dirtySuffix
	}

	commitCount := gitCommand(tentacleDir, "rev-list", "--count", "HEAD")
	if commitCount == "" {
		commitCount = "0"
	}
	return "0.1.0+" + commitCount + "." + shortSHA + dirtySuffix
}

// gitCommand runs a git subcommand in dir and returns trimmed stdout.
// Returns empty string on any error (not a git repo, git not found, etc.).
func gitCommand(dir string, args ...string) string {
	cmd := exec.Command("git", args...) //nolint:gosec,noctx // git metadata reads are fire-and-forget; context cancellation not needed
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// readGitProvenance reads git state for the given directory.
// Returns nil if the directory is not a git repo.
func readGitProvenance(dir string) *GitProvenance {
	commit := gitCommand(dir, "rev-parse", "--short=7", "HEAD")
	if commit == "" {
		return nil
	}

	branch := gitCommand(dir, "rev-parse", "--abbrev-ref", "HEAD")
	repo := gitCommand(dir, "config", "--get", "remote.origin.url")
	dirty := gitCommand(dir, "status", "--porcelain") != ""

	return &GitProvenance{
		Commit: commit,
		Branch: branch,
		Repo:   repo,
		Dirty:  dirty,
	}
}
