package cli

import (
	"fmt"
	"os"
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

The workspace path is written to ~/.tentacular/config.yaml so that
$shared secret references resolve from this directory.

Default path: ~/tentacles`,
		Args: cobra.MaximumNArgs(1),
		RunE: runInitWorkspace,
	}
}

func runInitWorkspace(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determining home directory: %w", err)
	}

	wsPath := filepath.Join(home, "tentacles")
	if len(args) > 0 {
		wsPath = args[0]
		wsPath = expandHome(wsPath)
	}

	absPath, err := filepath.Abs(wsPath)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Create workspace directory
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return fmt.Errorf("creating workspace directory: %w", err)
	}

	// Create .secrets/ pool
	secretsDir := filepath.Join(absPath, ".secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		return fmt.Errorf("creating .secrets directory: %w", err)
	}

	// Create .gitignore
	gitignorePath := filepath.Join(absPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignore := `.secrets.yaml
scratch/
.secrets/
`
		if err := os.WriteFile(gitignorePath, []byte(gitignore), 0o644); err != nil {
			return fmt.Errorf("writing .gitignore: %w", err)
		}
	}

	// Write workspace to config
	configPath := filepath.Join(home, ".tentacular", "config.yaml")
	cfg := TentacularConfig{}
	if data, err := os.ReadFile(configPath); err == nil {
		_ = yaml.Unmarshal(data, &cfg)
	}

	cfg.Workspace = absPath

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Workspace initialized at %s\n", absPath)
	fmt.Printf("  .secrets/   — shared secrets pool\n")
	fmt.Printf("  .gitignore  — default ignores\n")
	fmt.Printf("\nWorkspace path written to %s\n", configPath)
	return nil
}
