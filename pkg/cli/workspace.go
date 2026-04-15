package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewInitWorkspaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init-workspace [path]",
		Short: "Initialize a tentacular workspace directory",
		Long: `Creates a workspace directory for tentacular workflows (tentacles).

The workspace contains:
  - .secrets/    shared secrets pool
  - .gitignore   ignoring .secrets.yaml, scratch/, .secrets/

The workspace is git-initialized and the path is written to
~/.tentacular/config.yaml with git_state.enabled: true.

Default path: ~/tentacular`,
		Args: cobra.MaximumNArgs(1),
		RunE: runInitWorkspace,
	}
}

func runInitWorkspace(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determining home directory: %w", err)
	}

	wsPath := filepath.Join(home, "tentacular")
	if len(args) > 0 {
		wsPath = args[0]
		wsPath = expandHome(wsPath)
	}

	absPath, err := filepath.Abs(wsPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Create workspace directory
	if mkdirErr := os.MkdirAll(absPath, 0o755); mkdirErr != nil { //nolint:gosec // non-sensitive directory
		return fmt.Errorf("creating workspace directory: %w", mkdirErr)
	}

	// Create .secrets/ pool
	secretsDir := filepath.Join(absPath, ".secrets")
	if mkdirErr2 := os.MkdirAll(secretsDir, 0o755); mkdirErr2 != nil { //nolint:gosec // non-sensitive directory
		return fmt.Errorf("creating .secrets directory: %w", mkdirErr2)
	}

	// Create .gitignore
	gitignorePath := filepath.Join(absPath, ".gitignore")
	if _, statErr := os.Stat(gitignorePath); os.IsNotExist(statErr) {
		gitignore := `.secrets.yaml
scratch/
.secrets/
`
		if writeErr := os.WriteFile(gitignorePath, []byte(gitignore), 0o644); writeErr != nil { //nolint:gosec // non-sensitive gitignore file
			return fmt.Errorf("writing .gitignore: %w", writeErr)
		}
	}

	// Initialize git repo if not already one
	gitInitialized := false
	gitDir := filepath.Join(absPath, ".git")
	if _, statErr := os.Stat(gitDir); os.IsNotExist(statErr) {
		initCmd := exec.CommandContext(context.Background(), "git", "init", absPath) //nolint:gosec // absPath is caller-controlled
		initCmd.Stdout = os.Stdout
		initCmd.Stderr = os.Stderr
		if runErr := initCmd.Run(); runErr != nil {
			return fmt.Errorf("initializing git repo: %w", runErr)
		}
		gitInitialized = true
	}

	// Write workspace to config
	configPath := filepath.Join(home, ".tentacular", "config.yaml")
	cfg := TentacularConfig{}
	if cfgData, readErr := os.ReadFile(configPath); readErr == nil { //nolint:gosec // configPath is known config file
		_ = yaml.Unmarshal(cfgData, &cfg)
	}

	cfg.Workspace = absPath
	cfg.GitState.Enabled = true
	cfg.GitState.RepoPath = absPath

	if mkdirErr3 := os.MkdirAll(filepath.Dir(configPath), 0o755); mkdirErr3 != nil { //nolint:gosec // non-sensitive config directory
		return fmt.Errorf("creating config directory: %w", mkdirErr3)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil { //nolint:gosec // non-sensitive config file
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Workspace initialized at %s\n", absPath)
	fmt.Printf("  .secrets/   — shared secrets pool\n")
	fmt.Printf("  .gitignore  — default ignores\n")
	if gitInitialized {
		fmt.Printf("  .git/       — git repository\n")
	}
	fmt.Printf("\nWorkspace path written to %s\n", configPath)
	fmt.Printf("  git_state.enabled: true\n")
	fmt.Printf("  git_state.repo_path: %s\n", absPath)
	if gitInitialized {
		fmt.Printf("\nTo connect a remote:\n")
		fmt.Printf("  git -C %s remote add origin <url>\n", absPath)
		fmt.Printf("  git -C %s push -u origin main\n", absPath)
	}
	return nil
}
