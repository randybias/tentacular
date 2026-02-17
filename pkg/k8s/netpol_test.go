package k8s

import (
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

// --- Phase 4: NetworkPolicy Generation Tests ---

func TestGenerateNetworkPolicyNilContract(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		// No contract
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest != nil {
		t.Error("expected nil manifest for workflow without contract")
	}
}

func TestGenerateNetworkPolicyEmptyContract(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version:      "1",
			Dependencies: map[string]spec.Dependency{},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest for empty contract")
	}

	// Should still have DNS egress
	if !strings.Contains(manifest.Content, "kube-dns") {
		t.Error("expected DNS egress rule for empty contract")
	}

	// Should have default-deny for ingress (no ingress rules)
	if !strings.Contains(manifest.Content, "policyTypes") {
		t.Error("expected policyTypes section")
	}
}

func TestGenerateNetworkPolicySingleHTTPSDependency(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "manual"},
		},
		Nodes: map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"github": {
					Protocol: "https",
					Host:     "api.github.com",
					Port:     443,
				},
			},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Check metadata
	if manifest.Kind != "NetworkPolicy" {
		t.Errorf("expected kind NetworkPolicy, got %s", manifest.Kind)
	}
	if manifest.Name != "test-workflow-netpol" {
		t.Errorf("expected name test-workflow-netpol, got %s", manifest.Name)
	}

	// Check DNS egress
	if !strings.Contains(manifest.Content, "kube-dns") {
		t.Error("expected DNS egress rule")
	}
	if !strings.Contains(manifest.Content, "port: 53") {
		t.Error("expected DNS port 53")
	}

	// Check HTTPS egress
	if !strings.Contains(manifest.Content, "port: 443") {
		t.Error("expected HTTPS port 443 egress")
	}

	// Check namespace
	if !strings.Contains(manifest.Content, "namespace: default") {
		t.Error("expected namespace: default")
	}

	// Check no ingress rules (manual trigger)
	if strings.Contains(manifest.Content, "ingress:") {
		t.Error("expected no ingress rules for manual trigger")
	}
}

func TestGenerateNetworkPolicyPostgreSQLDependency(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"postgres": {
					Protocol: "postgresql",
					Host:     "postgres.svc.cluster.local",
					Port:     5432,
					Database: "appdb",
					User:     "postgres",
				},
			},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "pd-test")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Check PostgreSQL egress port
	if !strings.Contains(manifest.Content, "port: 5432") {
		t.Error("expected PostgreSQL port 5432 egress")
	}

	// Check namespace
	if !strings.Contains(manifest.Content, "namespace: pd-test") {
		t.Error("expected namespace: pd-test")
	}
}

func TestGenerateNetworkPolicyMultipleDependencies(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"github": {
					Protocol: "https",
					Host:     "api.github.com",
					Port:     443,
				},
				"postgres": {
					Protocol: "postgresql",
					Host:     "postgres.svc",
					Port:     5432,
					Database: "appdb",
					User:     "postgres",
				},
				"nats": {
					Protocol: "nats",
					Host:     "nats.svc",
					Port:     4222,
					Subject:  "events",
				},
			},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Check all egress ports
	if !strings.Contains(manifest.Content, "port: 443") {
		t.Error("expected HTTPS port 443")
	}
	if !strings.Contains(manifest.Content, "port: 5432") {
		t.Error("expected PostgreSQL port 5432")
	}
	if !strings.Contains(manifest.Content, "port: 4222") {
		t.Error("expected NATS port 4222")
	}
	if !strings.Contains(manifest.Content, "port: 53") {
		t.Error("expected DNS port 53")
	}
}

func TestGenerateNetworkPolicyWebhookTriggerIngress(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "webhook", Path: "/hook"},
		},
		Nodes: map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version:      "1",
			Dependencies: map[string]spec.Dependency{},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Check ingress section exists
	if !strings.Contains(manifest.Content, "ingress:") {
		t.Error("expected ingress section for webhook trigger")
	}

	// Check webhook ingress port
	if !strings.Contains(manifest.Content, "port: 8080") {
		t.Error("expected ingress port 8080 for webhook")
	}

	// Check policyTypes includes Ingress
	if !strings.Contains(manifest.Content, "- Ingress") {
		t.Error("expected policyTypes to include Ingress")
	}
}

func TestGenerateNetworkPolicyNonWebhookTriggerNoIngress(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 * * * *"},
			{Type: "manual"},
		},
		Nodes: map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version:      "1",
			Dependencies: map[string]spec.Dependency{},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Should NOT have ingress rules
	if strings.Contains(manifest.Content, "ingress:") {
		t.Error("expected no ingress section for non-webhook triggers")
	}
}

func TestGenerateNetworkPolicyLabels(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "my-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version:      "1",
			Dependencies: map[string]spec.Dependency{},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Check labels
	if !strings.Contains(manifest.Content, "app.kubernetes.io/name: my-workflow") {
		t.Error("expected app.kubernetes.io/name label")
	}
	if !strings.Contains(manifest.Content, "app.kubernetes.io/managed-by: tentacular") {
		t.Error("expected managed-by label")
	}

	// Check pod selector
	if !strings.Contains(manifest.Content, "podSelector:") {
		t.Error("expected podSelector section")
	}
}

func TestGenerateNetworkPolicyPolicyTypes(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version:      "1",
			Dependencies: map[string]spec.Dependency{},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Check policyTypes
	if !strings.Contains(manifest.Content, "policyTypes:") {
		t.Error("expected policyTypes section")
	}
	if !strings.Contains(manifest.Content, "- Ingress") {
		t.Error("expected Ingress policy type")
	}
	if !strings.Contains(manifest.Content, "- Egress") {
		t.Error("expected Egress policy type")
	}
}

func TestGenerateNetworkPolicyDNSAlwaysIncluded(t *testing.T) {
	tests := []struct {
		name         string
		dependencies map[string]spec.Dependency
	}{
		{
			name:         "empty dependencies",
			dependencies: map[string]spec.Dependency{},
		},
		{
			name: "with https dependency",
			dependencies: map[string]spec.Dependency{
				"api": {Protocol: "https", Host: "api.example.com", Port: 443},
			},
		},
		{
			name: "with postgres dependency",
			dependencies: map[string]spec.Dependency{
				"db": {Protocol: "postgresql", Host: "postgres.svc", Port: 5432, Database: "app", User: "user"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &spec.Workflow{
				Name:    "test",
				Version: "1.0",
				Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
				Contract: &spec.Contract{
					Version:      "1",
					Dependencies: tt.dependencies,
				},
			}

			manifest := GenerateNetworkPolicy(wf, "default")

			if manifest == nil {
				t.Fatal("expected non-nil manifest")
			}

			// DNS should always be included
			if !strings.Contains(manifest.Content, "kube-dns") {
				t.Error("expected DNS egress (kube-dns)")
			}
			if !strings.Contains(manifest.Content, "port: 53") {
				t.Error("expected DNS port 53")
			}
		})
	}
}

func TestGenerateNetworkPolicyValidYAML(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"api": {Protocol: "https", Host: "api.example.com", Port: 443},
			},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Check YAML structure
	if !strings.HasPrefix(manifest.Content, "apiVersion:") {
		t.Error("expected manifest to start with apiVersion")
	}
	if !strings.Contains(manifest.Content, "kind: NetworkPolicy") {
		t.Error("expected kind: NetworkPolicy")
	}
	if !strings.Contains(manifest.Content, "metadata:") {
		t.Error("expected metadata section")
	}
	if !strings.Contains(manifest.Content, "spec:") {
		t.Error("expected spec section")
	}
}

func TestGenerateNetworkPolicyNamespacing(t *testing.T) {
	namespaces := []string{"default", "pd-test", "production", "staging-123"}

	for _, ns := range namespaces {
		t.Run("namespace_"+ns, func(t *testing.T) {
			wf := &spec.Workflow{
				Name:    "test",
				Version: "1.0",
				Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
				Contract: &spec.Contract{
					Version:      "1",
					Dependencies: map[string]spec.Dependency{},
				},
			}

			manifest := GenerateNetworkPolicy(wf, ns)

			if manifest == nil {
				t.Fatal("expected non-nil manifest")
			}

			expected := "namespace: " + ns
			if !strings.Contains(manifest.Content, expected) {
				t.Errorf("expected namespace: %s in manifest", ns)
			}
		})
	}
}

func TestGenerateNetworkPolicyDefaultPortApplication(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"api": {
					Protocol: "https",
					Host:     "api.example.com",
					// Port omitted - should default to 443
				},
			},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Should have default HTTPS port 443
	if !strings.Contains(manifest.Content, "port: 443") {
		t.Error("expected default port 443 for HTTPS")
	}
}
