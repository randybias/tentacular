package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Tests for resolveEnclaveName — the shared 4-step resolver used by
// list, status, logs, events, pods, and deploy.

func TestResolveEnclaveName_FlagWins(t *testing.T) {
	t.Setenv("TENTACULAR_ENCLAVE", "env-enclave")
	dir := makeEnclaveConfigDir(t, "config-enclave")

	name, err := resolveEnclaveName("flag-enclave", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "flag-enclave" {
		t.Errorf("expected flag-enclave, got %q", name)
	}
}

func TestResolveEnclaveName_EnvWinsOverConfig(t *testing.T) {
	t.Setenv("TENTACULAR_ENCLAVE", "env-enclave")
	dir := makeEnclaveConfigDir(t, "config-enclave")

	name, err := resolveEnclaveName("", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "env-enclave" {
		t.Errorf("expected env-enclave, got %q", name)
	}
}

func TestResolveEnclaveName_ConfigInCwd(t *testing.T) {
	t.Setenv("TENTACULAR_ENCLAVE", "")
	dir := makeEnclaveConfigDir(t, "config-enclave")

	name, err := resolveEnclaveName("", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "config-enclave" {
		t.Errorf("expected config-enclave, got %q", name)
	}
}

func TestResolveEnclaveName_ConfigInParent(t *testing.T) {
	t.Setenv("TENTACULAR_ENCLAVE", "")
	parent := makeEnclaveConfigDir(t, "parent-enclave")
	child := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	name, err := resolveEnclaveName("", child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "parent-enclave" {
		t.Errorf("expected parent-enclave, got %q", name)
	}
}

func TestResolveEnclaveName_AllMissingReturnsError(t *testing.T) {
	t.Setenv("TENTACULAR_ENCLAVE", "")
	dir := t.TempDir() // no .tentacular/config.yaml

	_, err := resolveEnclaveName("", dir)
	if err == nil {
		t.Fatal("expected error when no enclave specified, got nil")
	}
	want := "no enclave specified: pass --enclave, set TENTACULAR_ENCLAVE, or add enclave: <name> to .tentacular/config.yaml"
	if err.Error() != want {
		t.Errorf("expected error %q, got %q", want, err.Error())
	}
}

func TestResolveEnclaveName_EmptyFlagTreatedAsUnset(t *testing.T) {
	t.Setenv("TENTACULAR_ENCLAVE", "env-enclave")
	dir := t.TempDir()

	// empty string flag should fall through to env var
	name, err := resolveEnclaveName("", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "env-enclave" {
		t.Errorf("expected env-enclave, got %q", name)
	}
}

func TestResolveEnclaveName_ConfigWithNoEnclaveFieldIgnored(t *testing.T) {
	t.Setenv("TENTACULAR_ENCLAVE", "")
	dir := t.TempDir()
	// Write a .tentacular/config.yaml without an enclave field
	cfgDir := filepath.Join(dir, ".tentacular")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("namespace: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveEnclaveName("", dir)
	if err == nil {
		t.Fatal("expected error when enclave field absent from config, got nil")
	}
}

// makeEnclaveConfigDir creates a temp dir with a .tentacular/config.yaml
// containing the given enclave name and returns the dir path.
func makeEnclaveConfigDir(t *testing.T, enclaveName string) string {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".tentacular")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "enclave: " + enclaveName + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
