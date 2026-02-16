package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

// --- WI-3: Kind Cluster Detection Tests ---
// These tests validate DetectKindCluster, isLocalhostServer, and LoadImageToKind.
// Build tag: wi3 -- run with: go test -tags wi3 ./pkg/k8s/...

func writeKubeconfig(t *testing.T, dir, content string) string {
	t.Helper()
	kubeDir := filepath.Join(dir, ".kube")
	os.MkdirAll(kubeDir, 0o755)
	path := filepath.Join(kubeDir, "config")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}
	return path
}

func TestDetectKindClusterWithKindContext(t *testing.T) {
	tmpHome := t.TempDir()
	kubeconfig := writeKubeconfig(t, tmpHome, `apiVersion: v1
kind: Config
current-context: kind-tentacular
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: kind-tentacular
contexts:
- context:
    cluster: kind-tentacular
    user: kind-tentacular
  name: kind-tentacular
users:
- name: kind-tentacular
  user: {}
`)

	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfig)
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsKind {
		t.Error("expected IsKind to be true for kind- prefixed context with localhost server")
	}
	if info.ClusterName != "tentacular" {
		t.Errorf("expected cluster name tentacular, got %s", info.ClusterName)
	}
	if info.Context != "kind-tentacular" {
		t.Errorf("expected context kind-tentacular, got %s", info.Context)
	}
}

func TestDetectKindClusterWithNonKindContext(t *testing.T) {
	tmpHome := t.TempDir()
	kubeconfig := writeKubeconfig(t, tmpHome, `apiVersion: v1
kind: Config
current-context: my-production-cluster
clusters:
- cluster:
    server: https://api.production.example.com:6443
  name: production
contexts:
- context:
    cluster: production
    user: admin
  name: my-production-cluster
users:
- name: admin
  user: {}
`)

	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfig)
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsKind {
		t.Error("expected IsKind to be false for non-kind context")
	}
	if info.Context != "my-production-cluster" {
		t.Errorf("expected context my-production-cluster, got %s", info.Context)
	}
	if info.ClusterName != "" {
		t.Errorf("expected empty cluster name for non-kind, got %s", info.ClusterName)
	}
}

func TestDetectKindClusterWithMissingKubeconfig(t *testing.T) {
	tmpHome := t.TempDir()
	// Point KUBECONFIG to a non-existent file
	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", filepath.Join(tmpHome, "nonexistent", "config"))
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("expected no error for missing kubeconfig, got: %v", err)
	}
	if info.IsKind {
		t.Error("expected IsKind to be false when kubeconfig is missing")
	}
	if info.Context != "" {
		t.Errorf("expected empty context when kubeconfig is missing, got %s", info.Context)
	}
}

func TestDetectKindClusterWithEmptyCurrentContext(t *testing.T) {
	tmpHome := t.TempDir()
	kubeconfig := writeKubeconfig(t, tmpHome, `apiVersion: v1
kind: Config
current-context: ""
clusters: []
contexts: []
users: []
`)

	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfig)
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsKind {
		t.Error("expected IsKind to be false with empty current-context")
	}
}

func TestDetectKindClusterKindPrefixButRemoteServer(t *testing.T) {
	// Edge case: context has kind- prefix but server is NOT localhost
	tmpHome := t.TempDir()
	kubeconfig := writeKubeconfig(t, tmpHome, `apiVersion: v1
kind: Config
current-context: kind-remote
clusters:
- cluster:
    server: https://api.remote-cluster.example.com:6443
  name: kind-remote
contexts:
- context:
    cluster: kind-remote
    user: kind-remote
  name: kind-remote
users:
- name: kind-remote
  user: {}
`)

	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfig)
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// kind- prefix but NOT localhost -> should NOT be detected as kind
	if info.IsKind {
		t.Error("expected IsKind to be false when server is not localhost despite kind- prefix")
	}
	if info.Context != "kind-remote" {
		t.Errorf("expected context kind-remote, got %s", info.Context)
	}
}

func TestDetectKindClusterLocalhostIPv6(t *testing.T) {
	tmpHome := t.TempDir()
	kubeconfig := writeKubeconfig(t, tmpHome, `apiVersion: v1
kind: Config
current-context: kind-ipv6test
clusters:
- cluster:
    server: https://[::1]:6443
  name: kind-ipv6test
contexts:
- context:
    cluster: kind-ipv6test
    user: kind-ipv6test
  name: kind-ipv6test
users:
- name: kind-ipv6test
  user: {}
`)

	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfig)
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsKind {
		t.Error("expected IsKind to be true for IPv6 localhost [::1]")
	}
	if info.ClusterName != "ipv6test" {
		t.Errorf("expected cluster name ipv6test, got %s", info.ClusterName)
	}
}

func TestDetectKindClusterLocalhostHostname(t *testing.T) {
	tmpHome := t.TempDir()
	kubeconfig := writeKubeconfig(t, tmpHome, `apiVersion: v1
kind: Config
current-context: kind-localtest
clusters:
- cluster:
    server: https://localhost:6443
  name: kind-localtest
contexts:
- context:
    cluster: kind-localtest
    user: kind-localtest
  name: kind-localtest
users:
- name: kind-localtest
  user: {}
`)

	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfig)
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.IsKind {
		t.Error("expected IsKind to be true for localhost hostname")
	}
}

func TestDetectKindClusterContextMissing(t *testing.T) {
	// current-context references a context that doesn't exist in the contexts list
	tmpHome := t.TempDir()
	kubeconfig := writeKubeconfig(t, tmpHome, `apiVersion: v1
kind: Config
current-context: kind-nonexistent
clusters: []
contexts: []
users: []
`)

	origKubeconfig := os.Getenv("KUBECONFIG")
	os.Setenv("KUBECONFIG", kubeconfig)
	defer os.Setenv("KUBECONFIG", origKubeconfig)

	info, err := DetectKindCluster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsKind {
		t.Error("expected IsKind to be false when context is not found in contexts list")
	}
}

func TestLoadImageToKindReturnsErrorOnMissingBinary(t *testing.T) {
	// Ensure kind binary is not found by using empty PATH
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	err := LoadImageToKind("test-image:latest", "test-cluster")
	if err == nil {
		t.Error("expected error when kind binary is not found")
	}
}

func TestClusterInfoFieldsZeroValue(t *testing.T) {
	info := ClusterInfo{}
	if info.IsKind {
		t.Error("expected zero-value IsKind to be false")
	}
	if info.ClusterName != "" {
		t.Error("expected zero-value ClusterName to be empty")
	}
	if info.Context != "" {
		t.Error("expected zero-value Context to be empty")
	}
}
