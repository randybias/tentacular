package cli

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestInitWorkspaceDefaultPath verifies that the default workspace path is ~/tentacular.
func TestInitWorkspaceDefaultPath(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	cmd := NewInitWorkspaceCmd()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("init-workspace failed: %v", err)
	}

	expectedPath := filepath.Join(tmpHome, "tentacular")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected workspace at ~/tentacular (%s): %v", expectedPath, err)
	}

	// Verify config was written with correct workspace path
	configPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid config YAML: %v", err)
	}
	if cfg.Workspace != expectedPath {
		t.Errorf("expected workspace=%s in config, got %s", expectedPath, cfg.Workspace)
	}
}

// TestInitWorkspaceCreatesGitRepo verifies that init-workspace runs git init
// and sets git_state.enabled=true in the config.
func TestInitWorkspaceCreatesGitRepo(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	cmd := NewInitWorkspaceCmd()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("init-workspace failed: %v", err)
	}

	wsPath := filepath.Join(tmpHome, "tentacular")

	// Verify .git directory was created
	gitDir := filepath.Join(wsPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Errorf("expected .git directory at %s: %v", gitDir, err)
	}

	// Verify config has git_state.enabled=true
	configPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid config YAML: %v", err)
	}
	if !cfg.GitState.Enabled {
		t.Error("expected git_state.enabled=true in config")
	}
	if cfg.GitState.RepoPath != wsPath {
		t.Errorf("expected git_state.repo_path=%s in config, got %s", wsPath, cfg.GitState.RepoPath)
	}
}

// TestInitWorkspaceCustomPath verifies that a custom path arg is respected.
func TestInitWorkspaceCustomPath(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	customPath := filepath.Join(tmpHome, "my-workspace")

	cmd := NewInitWorkspaceCmd()
	if err := cmd.RunE(cmd, []string{customPath}); err != nil {
		t.Fatalf("init-workspace with custom path failed: %v", err)
	}

	if _, err := os.Stat(customPath); err != nil {
		t.Errorf("expected workspace at custom path %s: %v", customPath, err)
	}

	configPath := filepath.Join(tmpHome, ".tentacular", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	var cfg TentacularConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid config YAML: %v", err)
	}
	if cfg.Workspace != customPath {
		t.Errorf("expected workspace=%s in config, got %s", customPath, cfg.Workspace)
	}
}

// TestScaffoldInitRequiresEnclaveWhenGitState verifies that scaffold init
// returns an error when git_state is enabled and --enclave is not provided.
func TestScaffoldInitRequiresEnclaveWhenGitState(t *testing.T) {
	home, _ := scaffoldTestFixture(t, "test-scaffold", "")
	setTestHome(t, home)

	// Write user config with git_state.enabled=true
	configDir := filepath.Join(home, ".tentacular")
	_ = os.MkdirAll(configDir, 0o755)
	configContent := "git_state:\n  enabled: true\n  repo_path: /tmp/fake-repo\n"
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644)

	// Call scaffold init WITHOUT --dir and WITHOUT --enclave
	err := makeScaffoldInitCmd("test-scaffold", "my-tentacle",
		map[string]string{},
		map[string]bool{"no-params": true},
	)
	if err == nil {
		t.Fatal("expected error when --enclave not provided with git-state enabled, got nil")
	}
	if err.Error() != "--enclave is required (pass it explicitly)" {
		t.Errorf("expected specific error message, got: %v", err)
	}
}

// TestLoadConfigUserLevelOnly verifies that LoadConfig only reads
// ~/.tentacular/config.yaml and ignores any .tentacular/config.yaml in CWD.
func TestLoadConfigUserLevelOnly(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Write user-level config
	userDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte("registry: user-reg\n"), 0o644)

	// Write project-level config (should be ignored)
	projDir := filepath.Join(tmpDir, ".tentacular")
	_ = os.MkdirAll(projDir, 0o755)
	_ = os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte("registry: project-reg\n"), 0o644)

	cfg := LoadConfig()
	if cfg.Registry != "user-reg" {
		t.Errorf("expected user-reg (project-level ignored), got %s", cfg.Registry)
	}
}
