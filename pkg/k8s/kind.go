package k8s

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// ClusterInfo holds details about the current K8s cluster.
type ClusterInfo struct {
	IsKind      bool
	ClusterName string
	Context     string
}

// DetectKindCluster reads the kubeconfig and checks whether the current context
// points to a kind cluster (context name has "kind-" prefix and server is localhost).
func DetectKindCluster() (*ClusterInfo, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return &ClusterInfo{}, nil // No kubeconfig, not kind
	}

	currentContext := config.CurrentContext
	if currentContext == "" {
		return &ClusterInfo{}, nil
	}

	ctx, ok := config.Contexts[currentContext]
	if !ok {
		return &ClusterInfo{}, nil
	}

	// Check for "kind-" prefix in context name
	if !strings.HasPrefix(currentContext, "kind-") {
		return &ClusterInfo{Context: currentContext}, nil
	}

	// Verify the cluster server is localhost
	cluster, ok := config.Clusters[ctx.Cluster]
	if !ok {
		return &ClusterInfo{Context: currentContext}, nil
	}

	if !isLocalhostServer(cluster) {
		return &ClusterInfo{Context: currentContext}, nil
	}

	// Extract cluster name from "kind-<name>" context
	clusterName := strings.TrimPrefix(currentContext, "kind-")

	return &ClusterInfo{
		IsKind:      true,
		ClusterName: clusterName,
		Context:     currentContext,
	}, nil
}

// isLocalhostServer checks if the cluster server URL points to localhost.
func isLocalhostServer(cluster *api.Cluster) bool {
	server := cluster.Server
	return strings.Contains(server, "127.0.0.1") ||
		strings.Contains(server, "localhost") ||
		strings.Contains(server, "[::1]")
}

// LoadImageToKind loads a docker image into a kind cluster.
func LoadImageToKind(imageName, clusterName string) error {
	args := []string{"load", "docker-image", imageName}
	if clusterName != "" {
		args = append(args, "--name", clusterName)
	}

	cmd := exec.Command("kind", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kind load docker-image: %w", err)
	}
	return nil
}
