package k8s

import (
	"strings"
	"testing"
)

func TestGenerateMCPServerManifests_Count(t *testing.T) {
	manifests := GenerateMCPServerManifests("tentacular-system", "tentacular-mcp:latest", "test-token")
	if len(manifests) != 6 {
		t.Errorf("expected 6 manifests, got %d", len(manifests))
	}
}

func TestGenerateMCPServerManifests_Kinds(t *testing.T) {
	manifests := GenerateMCPServerManifests("tentacular-system", "", "test-token")
	kinds := make(map[string]bool)
	for _, m := range manifests {
		kinds[m.Kind] = true
	}
	for _, want := range []string{"ServiceAccount", "ClusterRole", "ClusterRoleBinding", "Secret", "Deployment", "Service"} {
		if !kinds[want] {
			t.Errorf("missing manifest kind: %s", want)
		}
	}
}

func TestGenerateMCPServerManifests_DefaultImage(t *testing.T) {
	manifests := GenerateMCPServerManifests("tentacular-system", "", "test-token")
	for _, m := range manifests {
		if m.Kind == "Deployment" {
			if !strings.Contains(m.Content, DefaultMCPImage) {
				t.Errorf("deployment should use default image %s", DefaultMCPImage)
			}
		}
	}
}

func TestGenerateMCPServerManifests_TokenInSecret(t *testing.T) {
	manifests := GenerateMCPServerManifests("tentacular-system", "", "my-secret-token")
	for _, m := range manifests {
		if m.Kind == "Secret" {
			if !strings.Contains(m.Content, "my-secret-token") {
				t.Errorf("secret manifest should contain the token")
			}
		}
	}
}

func TestGenerateMCPServerManifests_NamespaceInAllManifests(t *testing.T) {
	ns := "my-custom-ns"
	manifests := GenerateMCPServerManifests(ns, "", "token")
	for _, m := range manifests {
		// ClusterRole and ClusterRoleBinding don't have a namespace field in metadata
		if m.Kind == "ClusterRole" {
			continue
		}
		if !strings.Contains(m.Content, ns) {
			t.Errorf("manifest %s/%s should contain namespace %s", m.Kind, m.Name, ns)
		}
	}
}

func TestGenerateMCPServerManifests_DeploymentUsesTokenPath(t *testing.T) {
	manifests := GenerateMCPServerManifests("tentacular-system", "", "test-token")
	for _, m := range manifests {
		if m.Kind != "Deployment" {
			continue
		}
		if !strings.Contains(m.Content, "TOKEN_PATH") {
			t.Error("deployment should set TOKEN_PATH env var, not inject token value directly")
		}
		if strings.Contains(m.Content, "TENTACULAR_MCP_TOKEN") {
			t.Error("deployment should not use TENTACULAR_MCP_TOKEN env var (use TOKEN_PATH instead)")
		}
		if !strings.Contains(m.Content, "/etc/tentacular-mcp/token") {
			t.Error("deployment should mount token at /etc/tentacular-mcp/token")
		}
		if !strings.Contains(m.Content, "volumeMounts") {
			t.Error("deployment should have volumeMounts for secret")
		}
		if !strings.Contains(m.Content, "volumes") {
			t.Error("deployment should have volumes section for token secret")
		}
	}
}

func TestMCPEndpointInCluster(t *testing.T) {
	endpoint := MCPEndpointInCluster("tentacular-system")
	expected := "http://tentacular-mcp.tentacular-system.svc.cluster.local:8080"
	if endpoint != expected {
		t.Errorf("expected %s, got %s", expected, endpoint)
	}
}

func TestGenerateMCPToken(t *testing.T) {
	token1, err := GenerateMCPToken()
	if err != nil {
		t.Fatalf("GenerateMCPToken: %v", err)
	}
	if len(token1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(token1))
	}

	// Should be unique
	token2, err := GenerateMCPToken()
	if err != nil {
		t.Fatalf("GenerateMCPToken: %v", err)
	}
	if token1 == token2 {
		t.Error("tokens should be unique")
	}
}
