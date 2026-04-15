package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// NewStateCmd creates the "state" parent command with subcommands.
func NewStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Manage git-backed state repository for tentacles",
	}
	cmd.AddCommand(newStateInitCmd())
	cmd.AddCommand(newStateStatusCmd())
	cmd.AddCommand(newStateCommitCmd())
	return cmd
}

// --- state init ---

func newStateInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a git-state repository for tentacle source and secrets",
		Args:  cobra.NoArgs,
		RunE:  runStateInit,
	}
	cmd.Flags().String("repo-path", "", "Path to the git-state repo (required)")
	_ = cmd.MarkFlagRequired("repo-path")
	return cmd
}

func runStateInit(cmd *cobra.Command, _ []string) error {
	repoPath, _ := cmd.Flags().GetString("repo-path")

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolving repo path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0o700); err != nil {
		return fmt.Errorf("creating repo directory: %w", err)
	}

	// Initialize git repo if not already one
	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		initCmd := exec.CommandContext(context.Background(), "git", "init", absPath) //nolint:gosec // absPath is caller-controlled
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr
		if err := initCmd.Run(); err != nil {
			return fmt.Errorf("initializing git repo: %w", err)
		}
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(absPath, "enclaves"),
		filepath.Join(absPath, "archive"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	// Write .gitkeep files to track empty directories
	for _, d := range []string{"enclaves", "archive"} {
		keepFile := filepath.Join(absPath, d, ".gitkeep")
		if _, err := os.Stat(keepFile); os.IsNotExist(err) {
			if err := os.WriteFile(keepFile, []byte(""), 0o600); err != nil {
				return fmt.Errorf("writing %s/.gitkeep: %w", d, err)
			}
		}
	}

	// Write .gitignore
	gitignorePath := filepath.Join(absPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignoreContent := "# Tentacular git-state repo\n*.secrets\n.secrets.yaml\nscratch/\n"
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o600); err != nil {
			return fmt.Errorf("writing .gitignore: %w", err)
		}
	}

	// Write .gitleaks.toml
	gitleaksPath := filepath.Join(absPath, ".gitleaks.toml")
	if _, err := os.Stat(gitleaksPath); os.IsNotExist(err) {
		gitleaksContent := "# Gitleaks configuration for tentacular git-state repo\ntitle = \"tentacular-state\"\n\n[[rules]]\ndescription = \"Generic Secret\"\nid = \"generic-api-key\"\nregex = '''(?i)(api[_-]?key|secret|token)[\\s]*[=:][\\s]*['\"]?[a-zA-Z0-9_\\-]{16,}['\"]?'''\nseverity = \"HIGH\"\n"
		if err := os.WriteFile(gitleaksPath, []byte(gitleaksContent), 0o600); err != nil {
			return fmt.Errorf("writing .gitleaks.toml: %w", err)
		}
	}

	// Write .pre-commit-config.yaml
	preCommitPath := filepath.Join(absPath, ".pre-commit-config.yaml")
	if _, err := os.Stat(preCommitPath); os.IsNotExist(err) {
		preCommitContent := "repos:\n  - repo: https://github.com/gitleaks/gitleaks\n    rev: v8.18.0\n    hooks:\n      - id: gitleaks\n"
		if err := os.WriteFile(preCommitPath, []byte(preCommitContent), 0o600); err != nil {
			return fmt.Errorf("writing .pre-commit-config.yaml: %w", err)
		}
	}

	// Write README
	readmePath := filepath.Join(absPath, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		readmeContent := "# Tentacular Git-State Repository\n\nThis repository tracks tentacle source and metadata managed by `tntc`.\n\n## Structure\n\n- `enclaves/` -- tentacle source organized by enclave\n- `archive/` -- retired tentacles\n\nConfiguration lives in `~/.tentacular/config.yaml` (user-level only).\n\n## Commit Convention\n\nUse [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):\n\n```\nfeat(enclave/tentacle): add initial scaffold\nfix(enclave/tentacle): correct API endpoint\nchore(enclave/tentacle): update secrets reference\n```\n"
		if err := os.WriteFile(readmePath, []byte(readmeContent), 0o644); err != nil { //nolint:gosec // README is non-sensitive
			return fmt.Errorf("writing README.md: %w", err)
		}
	}

	fmt.Printf("Initialized git-state repo at %s\n", absPath)
	fmt.Printf("\nNext: add the following to your ~/.tentacular/config.yaml:\n")
	fmt.Printf("  workspace: %s\n", absPath)
	fmt.Printf("  git_state:\n")
	fmt.Printf("    enabled: true\n")
	fmt.Printf("    repo_path: %s\n", absPath)
	return nil
}

// --- state status ---

func newStateStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show git-state repo status across enclaves",
		Args:  cobra.NoArgs,
		RunE:  runStateStatus,
	}
	cmd.Flags().Bool("assert-clean", false, "Exit non-zero if there are uncommitted changes in the git-state repo")
	return cmd
}

func runStateStatus(cmd *cobra.Command, _ []string) error {
	assertClean, _ := cmd.Flags().GetBool("assert-clean")

	cfg := LoadConfig()
	if !cfg.GitState.Enabled || cfg.GitState.RepoPath == "" {
		return errors.New("git-state is not configured; run 'tntc state init --repo-path <path>' first")
	}

	repoPath := cfg.GitState.RepoPath

	// Show branch and remote tracking status
	branchCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "status", "-sb") //nolint:gosec // repoPath from config
	branchOut, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("reading git-state repo status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(branchOut)), "\n")
	if len(lines) > 0 {
		fmt.Printf("Branch: %s\n\n", strings.TrimPrefix(lines[0], "## "))
	}

	// List enclaves
	enclavesDir := filepath.Join(repoPath, "enclaves")
	dirEntries, dirErr := os.ReadDir(enclavesDir)
	if dirErr != nil {
		if os.IsNotExist(dirErr) {
			fmt.Println("No enclaves found.")
			return nil
		}
		return fmt.Errorf("reading enclaves directory: %w", dirErr)
	}

	enclaves := make([]string, 0, len(dirEntries))
	for _, e := range dirEntries {
		if e.IsDir() {
			enclaves = append(enclaves, e.Name())
		}
	}

	if len(enclaves) == 0 {
		fmt.Println("No enclaves found.")
	} else {
		fmt.Printf("Enclaves (%d):\n", len(enclaves))
		for _, enclave := range enclaves {
			enclavePath := filepath.Join(enclavesDir, enclave)
			tentacles, readErr := os.ReadDir(enclavePath)
			if readErr != nil {
				fmt.Printf("  %s (error reading)\n", enclave)
				continue
			}
			tentacleCount := 0
			for _, t := range tentacles {
				if t.IsDir() {
					tentacleCount++
				}
			}
			fmt.Printf("  %s (%d tentacle(s))\n", enclave, tentacleCount)
		}
	}

	// Show dirty files if any
	porcelainCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "status", "--porcelain") //nolint:gosec // repoPath from config
	porcelainOut, err := porcelainCmd.Output()
	if err != nil {
		return fmt.Errorf("checking dirty files: %w", err)
	}
	dirty := strings.TrimSpace(string(porcelainOut))
	if dirty == "" {
		fmt.Println("\nWorking tree: clean")
	} else {
		fmt.Printf("\nUncommitted changes:\n")
		for _, line := range strings.Split(dirty, "\n") {
			fmt.Printf("  %s\n", line)
		}
		if assertClean {
			return errors.New("git-state repo has uncommitted changes")
		}
	}

	return nil
}

// --- state commit ---

func newStateCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Stage and commit changes under enclaves/ in the git-state repo",
		Args:  cobra.NoArgs,
		RunE:  runStateCommit,
	}
	cmd.Flags().StringP("message", "m", "", "Commit message (required)")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

func runStateCommit(cmd *cobra.Command, _ []string) error {
	message, _ := cmd.Flags().GetString("message")

	cfg := LoadConfig()
	if !cfg.GitState.Enabled || cfg.GitState.RepoPath == "" {
		return errors.New("git-state is not configured; run 'tntc state init --repo-path <path>' first")
	}

	repoPath := cfg.GitState.RepoPath
	enclavesPath := filepath.Join(repoPath, "enclaves")

	// Stage all changes under enclaves/
	addCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "add", enclavesPath) //nolint:gosec // repoPath from config
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("staging enclaves/: %w", err)
	}

	// Check if there is anything staged
	diffCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "diff", "--cached", "--quiet") //nolint:gosec // repoPath from config
	if err := diffCmd.Run(); err == nil {
		fmt.Println("Nothing to commit — working tree clean under enclaves/")
		return nil
	}

	// Commit
	commitCmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "commit", "-m", message) //nolint:gosec // repoPath from config, message from user flag
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}
