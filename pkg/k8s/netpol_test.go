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

	// All workflows get namespace-local ingress on port 8080 (for runner pod / CronJob)
	if !strings.Contains(manifest.Content, "ingress:") {
		t.Error("expected ingress rules for namespace-local access on port 8080")
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

	// Check that istio-system namespace selector is present — required for Istio gateway routing
	if !strings.Contains(manifest.Content, "namespaceSelector:") {
		t.Error("expected namespaceSelector for istio-system ingress")
	}
	if !strings.Contains(manifest.Content, "istio-system") {
		t.Error("expected istio-system in namespaceSelector for webhook ingress")
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

	// All workflows get namespace-local ingress on port 8080
	if !strings.Contains(manifest.Content, "ingress:") {
		t.Error("expected ingress rules for namespace-local access on port 8080")
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

func TestGenerateNetworkPolicyAdditionalEgressOverride(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "test-workflow",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"a": {Path: "./a.ts"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"api": {Protocol: "https", Host: "api.example.com", Port: 443},
			},
			NetworkPolicy: &spec.NetworkPolicyConfig{
				AdditionalEgress: []spec.EgressOverride{
					{ToCIDR: "10.0.0.0/8", Ports: []string{"8080/TCP"}},
					{ToCIDR: "172.16.0.0/12"}, // no ports = any
				},
			},
		},
	}

	manifest := GenerateNetworkPolicy(wf, "default")

	if manifest == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Should have the dependency egress (port 443)
	if !strings.Contains(manifest.Content, "port: 443") {
		t.Error("expected HTTPS port 443 from dependency")
	}

	// Should have DNS egress
	if !strings.Contains(manifest.Content, "port: 53") {
		t.Error("expected DNS port 53")
	}

	// Should have additional egress override CIDR (10.0.0.0/8)
	if !strings.Contains(manifest.Content, "cidr: 10.0.0.0/8") {
		t.Error("expected additionalEgress CIDR 10.0.0.0/8")
	}

	// Should have the override port 8080
	if !strings.Contains(manifest.Content, "port: 8080") {
		t.Error("expected additionalEgress port 8080")
	}

	// Should have the second override CIDR (172.16.0.0/12)
	if !strings.Contains(manifest.Content, "cidr: 172.16.0.0/12") {
		t.Error("expected additionalEgress CIDR 172.16.0.0/12")
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

// TestFullPipelineYAMLToNetworkPolicy exercises the complete flow:
// YAML → spec.Parse → DeriveSecrets/DeriveEgressRules → GenerateNetworkPolicy
func TestFullPipelineYAMLToNetworkPolicy(t *testing.T) {
	yamlContent := `
name: pipeline-test
version: "1.0"

triggers:
  - type: cron
    schedule: "0 * * * *"
  - type: webhook
    path: /hook

nodes:
  fetch:
    path: ./nodes/fetch.ts
  process:
    path: ./nodes/process.ts

edges:
  - from: fetch
    to: process

contract:
  version: "1"
  dependencies:
    github:
      protocol: https
      host: api.github.com
      port: 443
      auth:
        type: bearer-token
        secret: github.token
    postgres:
      protocol: postgresql
      host: db.ns.svc.cluster.local
      port: 5432
      database: appdb
      user: app
      auth:
        type: password
        secret: postgres.password
    slack:
      protocol: https
      host: hooks.slack.com
      port: 443
      auth:
        type: webhook-url
        secret: slack.webhook_url
  networkPolicy:
    additionalEgress:
      - toCIDR: "10.100.0.0/16"
        ports:
          - "9090/TCP"
`

	// Step 1: Parse YAML
	wf, warnings := spec.Parse([]byte(yamlContent))
	if wf == nil {
		t.Fatalf("spec.Parse failed, warnings: %v", warnings)
	}
	_ = warnings

	// Step 2: Verify contract was parsed
	if wf.Contract == nil {
		t.Fatal("expected contract to be parsed")
	}
	if len(wf.Contract.Dependencies) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(wf.Contract.Dependencies))
	}

	// Step 3: Verify derived secrets
	secrets := spec.DeriveSecrets(wf.Contract)
	if len(secrets) != 3 {
		t.Fatalf("expected 3 derived secrets, got %d: %v", len(secrets), secrets)
	}
	secretSet := make(map[string]bool)
	for _, s := range secrets {
		secretSet[s] = true
	}
	for _, expected := range []string{"github.token", "postgres.password", "slack.webhook_url"} {
		if !secretSet[expected] {
			t.Errorf("expected derived secret %q", expected)
		}
	}

	// Step 4: Verify derived egress rules
	egressRules := spec.DeriveEgressRules(wf.Contract)
	// 2 DNS (UDP+TCP) + 3 dependencies + 1 override = 6
	if len(egressRules) < 6 {
		t.Fatalf("expected at least 6 egress rules, got %d", len(egressRules))
	}

	// Verify DNS present
	hasDNS := false
	for _, r := range egressRules {
		if r.Port == 53 {
			hasDNS = true
			break
		}
	}
	if !hasDNS {
		t.Error("expected DNS egress rule")
	}

	// Verify override CIDR present
	hasOverride := false
	for _, r := range egressRules {
		if r.Host == "10.100.0.0/16" && r.Port == 9090 {
			hasOverride = true
			break
		}
	}
	if !hasOverride {
		t.Error("expected additionalEgress override 10.100.0.0/16:9090")
	}

	// Step 5: Verify derived ingress rules (has webhook trigger)
	ingressRules := spec.DeriveIngressRules(wf)
	if len(ingressRules) != 1 {
		t.Fatalf("expected 1 ingress rule for webhook, got %d", len(ingressRules))
	}
	if ingressRules[0].Port != 8080 {
		t.Errorf("expected webhook ingress port 8080, got %d", ingressRules[0].Port)
	}

	// Step 6: Generate NetworkPolicy
	manifest := GenerateNetworkPolicy(wf, "tentacular-test")
	if manifest == nil {
		t.Fatal("expected non-nil NetworkPolicy manifest")
	}

	// Verify complete manifest structure
	if !strings.Contains(manifest.Content, "kind: NetworkPolicy") {
		t.Error("expected kind: NetworkPolicy")
	}
	if !strings.Contains(manifest.Content, "namespace: tentacular-test") {
		t.Error("expected namespace: tentacular-test")
	}
	if !strings.Contains(manifest.Content, "port: 443") {
		t.Error("expected HTTPS port 443")
	}
	if !strings.Contains(manifest.Content, "port: 5432") {
		t.Error("expected PostgreSQL port 5432")
	}
	if !strings.Contains(manifest.Content, "port: 53") {
		t.Error("expected DNS port 53")
	}
	if !strings.Contains(manifest.Content, "cidr: 10.100.0.0/16") {
		t.Error("expected override CIDR in generated policy")
	}
	if !strings.Contains(manifest.Content, "port: 9090") {
		t.Error("expected override port 9090 in generated policy")
	}
	if !strings.Contains(manifest.Content, "ingress:") {
		t.Error("expected ingress section for webhook trigger")
	}
	if !strings.Contains(manifest.Content, "port: 8080") {
		t.Error("expected webhook ingress port 8080")
	}
}
