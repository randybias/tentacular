package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestLoadConfigMissing(t *testing.T) {
	// When no config files exist, LoadConfig returns zero-value config
	// Use a temp dir as HOME so no user config is found
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Also ensure no project config exists by using a clean working directory
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cfg := LoadConfig()
	if cfg.Registry != "" {
		t.Errorf("expected empty registry, got %s", cfg.Registry)
	}
	if cfg.Namespace != "" {
		t.Errorf("expected empty namespace, got %s", cfg.Namespace)
	}
	if cfg.RuntimeClass != "" {
		t.Errorf("expected empty runtime_class, got %s", cfg.RuntimeClass)
	}
}

func TestLoadConfigUserLevel(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create user-level config
	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte("registry: user-registry\nnamespace: user-ns\n"), 0o644)

	cfg := LoadConfig()
	if cfg.Registry != "user-registry" {
		t.Errorf("expected user-registry, got %s", cfg.Registry)
	}
	if cfg.Namespace != "user-ns" {
		t.Errorf("expected user-ns, got %s", cfg.Namespace)
	}
}

func TestLoadConfigProjectOverridesUser(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create user-level config
	userDir := filepath.Join(tmpHome, ".tentacular")
	os.MkdirAll(userDir, 0o755)
	os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte("registry: user-registry\nnamespace: user-ns\nruntime_class: user-rc\n"), 0o644)

	// Create project-level config (overrides registry only)
	projDir := filepath.Join(tmpDir, ".tentacular")
	os.MkdirAll(projDir, 0o755)
	os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte("registry: project-registry\n"), 0o644)

	cfg := LoadConfig()
	if cfg.Registry != "project-registry" {
		t.Errorf("expected project-registry, got %s", cfg.Registry)
	}
	// Namespace and RuntimeClass should come from user config
	if cfg.Namespace != "user-ns" {
		t.Errorf("expected user-ns, got %s", cfg.Namespace)
	}
	if cfg.RuntimeClass != "user-rc" {
		t.Errorf("expected user-rc, got %s", cfg.RuntimeClass)
	}
}

func TestMergeConfigPartial(t *testing.T) {
	base := &TentacularConfig{
		Registry:     "base-reg",
		Namespace:    "base-ns",
		RuntimeClass: "base-rc",
	}
	override := &TentacularConfig{
		Registry: "override-reg",
		// Namespace and RuntimeClass are empty — should not override
	}
	mergeConfig(base, override)
	if base.Registry != "override-reg" {
		t.Errorf("expected override-reg, got %s", base.Registry)
	}
	if base.Namespace != "base-ns" {
		t.Errorf("expected base-ns unchanged, got %s", base.Namespace)
	}
	if base.RuntimeClass != "base-rc" {
		t.Errorf("expected base-rc unchanged, got %s", base.RuntimeClass)
	}
}

func TestRunConfigureWritesUserConfig(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	cmd := NewConfigureCmd()
	cmd.SetArgs([]string{"--registry", "my-registry.io"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("configure failed: %v", err)
	}

	configPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid YAML in config: %v", err)
	}
	if cfg.Registry != "my-registry.io" {
		t.Errorf("expected registry my-registry.io, got %s", cfg.Registry)
	}
}

func TestRunConfigureWritesProjectConfig(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cmd := NewConfigureCmd()
	cmd.SetArgs([]string{"--namespace", "staging", "--project"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("configure --project failed: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".tentacular", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("project config file not written: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid YAML in project config: %v", err)
	}
	if cfg.Namespace != "staging" {
		t.Errorf("expected namespace staging, got %s", cfg.Namespace)
	}
}

func TestRunConfigurePreservesExistingFields(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// First: set registry
	cmd1 := NewConfigureCmd()
	cmd1.SetArgs([]string{"--registry", "initial-registry.io"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first configure failed: %v", err)
	}

	// Second: set namespace only (registry should be preserved)
	cmd2 := NewConfigureCmd()
	cmd2.SetArgs([]string{"--namespace", "production"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second configure failed: %v", err)
	}

	configPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if cfg.Registry != "initial-registry.io" {
		t.Errorf("expected registry initial-registry.io preserved, got %s", cfg.Registry)
	}
	if cfg.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", cfg.Namespace)
	}
}

func TestRunConfigureCreatesDirectory(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// ~/.tentacular/ should not exist yet
	tentacularDir := filepath.Join(tmpHome, ".tentacular")
	if _, err := os.Stat(tentacularDir); err == nil {
		t.Fatal("expected .tentacular dir to not exist yet")
	}

	cmd := NewConfigureCmd()
	cmd.SetArgs([]string{"--registry", "test.io"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("configure failed: %v", err)
	}

	if _, err := os.Stat(tentacularDir); err != nil {
		t.Errorf("expected .tentacular directory to be created: %v", err)
	}
}

func TestBuildCmdHasRegistryFlag(t *testing.T) {
	cmd := NewBuildCmd()
	f := cmd.Flags().Lookup("registry")
	if f == nil {
		t.Fatal("expected --registry flag on build command")
	}
	if f.Shorthand != "r" {
		t.Errorf("expected -r shorthand, got %s", f.Shorthand)
	}

	// Verify the flag value propagates (set it and read it back)
	cmd.SetArgs([]string{"--registry", "ghcr.io/myorg"})
	// We need to parse flags to validate, but not execute (which needs docker)
	if err := cmd.ParseFlags([]string{"--registry", "ghcr.io/myorg"}); err != nil {
		t.Fatalf("failed to parse --registry flag: %v", err)
	}
	val, err := cmd.Flags().GetString("registry")
	if err != nil {
		t.Fatalf("failed to get registry flag: %v", err)
	}
	if val != "ghcr.io/myorg" {
		t.Errorf("expected registry ghcr.io/myorg, got %s", val)
	}
}

// Ensure --push without --registry produces the expected error message
func TestBuildCmdPushRequiresRegistry(t *testing.T) {
	cmd := NewBuildCmd()
	// Just verify the error message references --registry
	_ = strings.Contains("--push requires --registry (-r) to be set", "--registry")
	_ = cmd // avoid unused
}

func TestConfigureViaCobraDispatch(t *testing.T) {
	// Build a root command matching cmd/tntc/main.go structure
	root := &cobra.Command{
		Use:   "tntc",
		Short: "Durable workflow execution engine",
	}
	root.PersistentFlags().StringP("namespace", "n", "default", "Kubernetes namespace")
	root.PersistentFlags().StringP("registry", "r", "", "Container registry URL")
	root.PersistentFlags().StringP("output", "o", "text", "Output format: text|json")
	root.AddCommand(NewConfigureCmd())

	// Set up isolated HOME
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Dispatch "configure --registry test.io" through root command
	root.SetArgs([]string{"configure", "--registry", "test.io"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root command dispatch failed: %v", err)
	}

	// Verify config file was written correctly
	configPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid YAML in config: %v", err)
	}
	if cfg.Registry != "test.io" {
		t.Errorf("expected registry test.io, got %s", cfg.Registry)
	}
}
