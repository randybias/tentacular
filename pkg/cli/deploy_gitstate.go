package cli

import (
	"fmt"
	"os/exec"
	"strings"
)

// checkGitStateClean verifies the git-state repo has no uncommitted changes
// for the given enclave/tentacle path.
func checkGitStateClean(repoPath, enclaveName, tentacleName string) error {
	if enclaveName == "" {
		return fmt.Errorf("git-state is enabled but no enclave specified; use --enclave")
	}
	if strings.ContainsAny(enclaveName, `/\`) || strings.Contains(enclaveName, "..") {
		return fmt.Errorf("invalid enclave name %q: must not contain path separators or '..'", enclaveName)
	}
	if strings.ContainsAny(tentacleName, `/\`) || strings.Contains(tentacleName, "..") {
		return fmt.Errorf("invalid tentacle name %q: must not contain path separators or '..'", tentacleName)
	}
	subPath := fmt.Sprintf("enclaves/%s/%s/", enclaveName, tentacleName)
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain", "--", subPath) //nolint:gosec // repoPath and subPath are config-controlled
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking git-state repo: %w", err)
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return fmt.Errorf("git-state repo has uncommitted changes for %s; commit before deploying", subPath)
	}
	return nil
}
