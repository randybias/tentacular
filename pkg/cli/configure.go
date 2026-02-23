package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigureCmd creates the "configure" subcommand.
func NewConfigureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Set default configuration",
		RunE:  runConfigure,
	}
	cmd.Flags().String("registry", "", "Default container registry URL")
	cmd.Flags().String("namespace", "", "Default Kubernetes namespace")
	cmd.Flags().String("runtime-class", "", "Default RuntimeClass name")
	cmd.Flags().Bool("project", false, "Write to project config (.tentacular/config.yaml) instead of user config")
	return cmd
}

func runConfigure(cmd *cobra.Command, args []string) error {
	registry, _ := cmd.Flags().GetString("registry")
	namespace, _ := cmd.Flags().GetString("namespace")
	runtimeClass, _ := cmd.Flags().GetString("runtime-class")
	project, _ := cmd.Flags().GetBool("project")

	// Determine config file path
	var configPath string
	if project {
		configPath = filepath.Join(".tentacular", "config.yaml")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determining home directory: %w", err)
		}
		configPath = filepath.Join(home, ".tentacular", "config.yaml")
	}

	// Load existing config
	cfg := TentacularConfig{}
	if data, err := os.ReadFile(configPath); err == nil {
		yaml.Unmarshal(data, &cfg)
	}

	// Apply flag overrides
	if cmd.Flags().Changed("registry") {
		cfg.Registry = registry
	}
	if cmd.Flags().Changed("namespace") {
		cfg.Namespace = namespace
	}
	if cmd.Flags().Changed("runtime-class") {
		cfg.RuntimeClass = runtimeClass
	}

	// Write config
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

	fmt.Printf("Configuration saved to %s\n", configPath)
	if cfg.Registry != "" {
		fmt.Printf("  registry: %s\n", cfg.Registry)
	}
	if cfg.Namespace != "" {
		fmt.Printf("  namespace: %s\n", cfg.Namespace)
	}
	if cfg.RuntimeClass != "" {
		fmt.Printf("  runtime_class: %s\n", cfg.RuntimeClass)
	}

	// Auto-profile all configured environments (best-effort; skips unreachable clusters)
	if len(cfg.Environments) > 0 {
		fmt.Println("\nGenerating cluster profiles...")
		AutoProfileEnvironments()
	}

	return nil
}
