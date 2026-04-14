package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunLogout_Success(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Unsetenv("TENTACULAR_CLUSTER")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create config
	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(""), 0o644)

	// Save a token first
	store := &OIDCTokenStore{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		Email:       "user@example.com",
	}
	if err := SaveOIDCToken("default", store); err != nil {
		t.Fatalf("saving token: %v", err)
	}

	cmd := NewLogoutCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runLogout: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Logged out") {
		t.Errorf("expected 'Logged out' message, got: %s", output)
	}
	if !strings.Contains(output, "default") {
		t.Errorf("expected environment name in output, got: %s", output)
	}

	// Verify token is actually removed
	loaded, err := LoadOIDCToken("default")
	if err != nil {
		t.Fatalf("loading token after logout: %v", err)
	}
	if loaded != nil {
		t.Error("expected token to be removed after logout")
	}
}

func TestRunLogout_NoExistingToken(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Unsetenv("TENTACULAR_CLUSTER")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(""), 0o644)

	cmd := NewLogoutCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")

	var out bytes.Buffer
	cmd.SetOut(&out)

	// Should not error even if no token exists
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runLogout with no token: %v", err)
	}

	if !strings.Contains(out.String(), "Logged out") {
		t.Errorf("expected 'Logged out' message, got: %s", out.String())
	}
}

func TestRunLogout_SpecificEnv(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Unsetenv("TENTACULAR_CLUSTER")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(""), 0o644)

	// Save tokens for two environments
	for _, env := range []string{"staging", "prod"} {
		store := &OIDCTokenStore{
			AccessToken: "token-" + env,
			ExpiresAt:   time.Now().Add(1 * time.Hour),
			Email:       env + "@example.com",
		}
		if err := SaveOIDCToken(env, store); err != nil {
			t.Fatalf("saving %s token: %v", env, err)
		}
	}

	cmd := NewLogoutCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")
	_ = cmd.PersistentFlags().Set("cluster", "staging")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("runLogout -e staging: %v", err)
	}

	if !strings.Contains(out.String(), "staging") {
		t.Errorf("expected 'staging' in output, got: %s", out.String())
	}

	// Verify staging is removed but prod is still there
	staging, _ := LoadOIDCToken("staging")
	if staging != nil {
		t.Error("staging token should be removed")
	}
	prod, _ := LoadOIDCToken("prod")
	if prod == nil {
		t.Error("prod token should still exist")
	}
}

func TestRunLogin_MissingOIDCConfig(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Unsetenv("TENTACULAR_CLUSTER")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Config exists but has no OIDC settings
	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`default_cluster: test
clusters:
  test:
    namespace: test-ns
`), 0o644)

	cmd := NewLoginCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when OIDC is not configured")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "OIDC not configured") && !strings.Contains(errMsg, "oidc_issuer") {
		t.Errorf("expected clear OIDC config error, got: %v", err)
	}
}

func TestRunLogin_MissingClientID(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Unsetenv("TENTACULAR_CLUSTER")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`default_cluster: test
clusters:
  test:
    namespace: test-ns
    oidc_issuer: https://auth.example.com/realms/test
`), 0o644)

	cmd := NewLoginCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when oidc_client_id is missing")
	}
	if !strings.Contains(err.Error(), "oidc_client_id") {
		t.Errorf("expected oidc_client_id error, got: %v", err)
	}
}

func TestRunLogin_NoEnvironment(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	_ = os.Setenv("HOME", tmpHome)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	origEnv := os.Getenv("TENTACULAR_CLUSTER")
	_ = os.Unsetenv("TENTACULAR_CLUSTER")
	defer func() { _ = os.Setenv("TENTACULAR_CLUSTER", origEnv) }()

	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Empty config -- no environments
	cfgDir := filepath.Join(tmpHome, ".tentacular")
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(""), 0o644)

	cmd := NewLoginCmd()
	cmd.PersistentFlags().StringP("cluster", "c", "", "Target cluster")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error with no environment configured")
	}
	if !strings.Contains(err.Error(), "no environment") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected environment error, got: %v", err)
	}
}
