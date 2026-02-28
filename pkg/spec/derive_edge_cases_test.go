package spec

import (
	"testing"
)

// --- Additional Secret Derivation Tests ---

func TestDeriveSecretsDeduplication(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github-api": {
				Protocol: "https",
				Host:     "api.github.com",
				Auth: &DependencyAuth{
					Secret: "github.token",
				},
			},
			"github-raw": {
				Protocol: "https",
				Host:     "raw.githubusercontent.com",
				Auth: &DependencyAuth{
					Secret: "github.token", // Same secret
				},
			},
		},
	}

	secrets := DeriveSecrets(contract)
	if len(secrets) != 1 {
		t.Errorf("expected 1 deduplicated secret, got %d: %v", len(secrets), secrets)
	}
	if secrets[0] != "github.token" {
		t.Errorf("expected github.token, got %s", secrets[0])
	}
}

func TestDeriveSecretsEmptyContract(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{},
	}

	secrets := DeriveSecrets(contract)
	if len(secrets) != 0 {
		t.Errorf("expected empty secrets for empty dependencies, got %v", secrets)
	}
}

func TestDeriveSecretsMixedAuthAndNoAuth(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"auth-api": {
				Protocol: "https",
				Host:     "api.auth.com",
				Auth: &DependencyAuth{
					Secret: "auth.key",
				},
			},
			"public-api": {
				Protocol: "https",
				Host:     "api.public.com",
				// No auth
			},
		},
	}

	secrets := DeriveSecrets(contract)
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	if secrets[0] != "auth.key" {
		t.Errorf("expected auth.key, got %s", secrets[0])
	}
}

// --- Additional Egress Rule Tests ---

func TestDeriveEgressRulesNilContract(t *testing.T) {
	rules := DeriveEgressRules(nil)

	// Should still have DNS rules even with nil contract
	if len(rules) != 2 {
		t.Fatalf("expected 2 DNS rules for nil contract, got %d", len(rules))
	}

	dnsCount := 0
	for _, r := range rules {
		if r.Port == 53 && r.Host == "kube-dns.kube-system.svc.cluster.local" {
			dnsCount++
		}
	}
	if dnsCount != 2 {
		t.Errorf("expected 2 DNS rules (UDP + TCP), got %d", dnsCount)
	}
}

func TestDeriveEgressRulesEmptyDependencies(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{},
	}

	rules := DeriveEgressRules(contract)

	// Should only have DNS rules
	if len(rules) != 2 {
		t.Errorf("expected 2 DNS rules for empty dependencies, got %d", len(rules))
	}
}

func TestDeriveEgressRulesMixedDefaultAndExplicitPorts(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"api-default": {
				Protocol: "https",
				Host:     "api.default.com",
				// No port specified
			},
			"api-custom": {
				Protocol: "https",
				Host:     "api.custom.com",
				Port:     8443, // Custom port
			},
		},
	}

	rules := DeriveEgressRules(contract)

	foundDefault := false
	foundCustom := false
	for _, r := range rules {
		if r.Host == "api.default.com" && r.Port == 443 {
			foundDefault = true
		}
		if r.Host == "api.custom.com" && r.Port == 8443 {
			foundCustom = true
		}
	}

	if !foundDefault {
		t.Error("expected default port 443 to be applied for api-default")
	}
	if !foundCustom {
		t.Error("expected custom port 8443 for api-custom")
	}
}

func TestDeriveEgressRulesDependencyMissingHost(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"incomplete": {
				Protocol: "https",
				Port:     443,
				// Missing host - should be skipped in egress rules
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// Should only have DNS rules, not the incomplete dependency
	for _, r := range rules {
		if r.Host != "kube-dns.kube-system.svc.cluster.local" {
			t.Errorf("expected only DNS rules, got rule for host %s", r.Host)
		}
	}
}

func TestDeriveEgressRulesAllProtocols(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"https-api": {
				Protocol: "https",
				Host:     "api.example.com",
			},
			"postgres-db": {
				Protocol: "postgresql",
				Host:     "postgres.svc",
			},
			"nats-queue": {
				Protocol: "nats",
				Host:     "nats.svc",
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// DNS (2) + https (1) + postgresql (1) + nats (1) = 5 rules
	if len(rules) < 5 {
		t.Fatalf("expected at least 5 rules, got %d", len(rules))
	}

	foundHTTPS := false
	foundPostgres := false
	foundNATS := false

	for _, r := range rules {
		if r.Host == "api.example.com" && r.Port == 443 {
			foundHTTPS = true
		}
		if r.Host == "postgres.svc" && r.Port == 5432 {
			foundPostgres = true
		}
		if r.Host == "nats.svc" && r.Port == 4222 {
			foundNATS = true
		}
	}

	if !foundHTTPS {
		t.Error("expected https egress rule with default port 443")
	}
	if !foundPostgres {
		t.Error("expected postgresql egress rule with default port 5432")
	}
	if !foundNATS {
		t.Error("expected nats egress rule with default port 4222")
	}
}

// --- Additional Ingress Rule Tests ---

func TestDeriveIngressRulesMultipleTriggerTypes(t *testing.T) {
	wf := &Workflow{
		Name:    "test",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "manual"},
			{Type: "cron", Schedule: "0 * * * *"},
			{Type: "queue", Subject: "events.test"},
		},
		Nodes: map[string]NodeSpec{
			"handler": {Path: "./handler.ts"},
		},
	}

	rules := DeriveIngressRules(wf)
	// Expect 2 rules: trigger ingress + MCP health probe ingress
	if len(rules) != 2 {
		t.Errorf("expected 2 ingress rules (trigger + MCP), got %d", len(rules))
	}
}

func TestDeriveIngressRulesWebhookWithOthers(t *testing.T) {
	wf := &Workflow{
		Name:    "test",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "manual"},
			{Type: "webhook", Path: "/hook"},
			{Type: "cron", Schedule: "0 * * * *"},
		},
		Nodes: map[string]NodeSpec{
			"handler": {Path: "./handler.ts"},
		},
	}

	rules := DeriveIngressRules(wf)
	// Expect 2 rules: webhook ingress + MCP health probe ingress
	if len(rules) != 2 {
		t.Fatalf("expected 2 ingress rules for webhook (webhook + MCP), got %d", len(rules))
	}
	if rules[0].Port != 8080 {
		t.Errorf("expected webhook ingress on port 8080, got %d", rules[0].Port)
	}
}

func TestDeriveIngressRulesMultipleWebhooks(t *testing.T) {
	wf := &Workflow{
		Name:    "test",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "webhook", Path: "/hook1"},
			{Type: "webhook", Path: "/hook2"},
		},
		Nodes: map[string]NodeSpec{
			"handler": {Path: "./handler.ts"},
		},
	}

	rules := DeriveIngressRules(wf)
	// Should create 2 ingress rules: one for webhook, one for MCP health probe
	if len(rules) != 2 {
		t.Errorf("expected 2 ingress rules for multiple webhooks (webhook + MCP), got %d", len(rules))
	}
}

// --- ApplyDefaultPorts Tests ---

func TestApplyDefaultPortsNilContract(t *testing.T) {
	// Should not panic
	ApplyDefaultPorts(nil)
}

func TestApplyDefaultPortsPreservesExplicitPorts(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"custom-https": {
				Protocol: "https",
				Host:     "api.example.com",
				Port:     8443,
			},
			"custom-postgres": {
				Protocol: "postgresql",
				Host:     "postgres.svc",
				Port:     15432,
			},
		},
	}

	ApplyDefaultPorts(contract)

	if contract.Dependencies["custom-https"].Port != 8443 {
		t.Error("expected explicit port 8443 to be preserved")
	}
	if contract.Dependencies["custom-postgres"].Port != 15432 {
		t.Error("expected explicit port 15432 to be preserved")
	}
}

func TestApplyDefaultPortsOnlyMissing(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"default-port": {
				Protocol: "https",
				Host:     "api.example.com",
			},
			"explicit-port": {
				Protocol: "https",
				Host:     "api.custom.com",
				Port:     8443,
			},
		},
	}

	ApplyDefaultPorts(contract)

	if contract.Dependencies["default-port"].Port != 443 {
		t.Errorf("expected default port 443, got %d", contract.Dependencies["default-port"].Port)
	}
	if contract.Dependencies["explicit-port"].Port != 8443 {
		t.Errorf("expected explicit port 8443 to be preserved, got %d", contract.Dependencies["explicit-port"].Port)
	}
}

func TestApplyDefaultPortsUnsupportedProtocol(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"blob": {
				Protocol: "blob",
				Host:     "storage.blob.core.windows.net",
				// blob protocol doesn't have default port in protocolDefaultPorts
			},
		},
	}

	ApplyDefaultPorts(contract)

	// Port should remain 0 since blob is not in protocolDefaultPorts
	if contract.Dependencies["blob"].Port != 0 {
		t.Errorf("expected port 0 for unsupported protocol, got %d", contract.Dependencies["blob"].Port)
	}
}

// --- Secret Parsing Helper Tests ---

func TestGetSecretServiceNameEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"service.key", "service"},
		{"nested.service.key", "nested"}, // Only splits on first dot
		{"no-dot", ""},
		{"", ""},
		{"service.", "service"},
		{".key", ""},
	}

	for _, tt := range tests {
		result := GetSecretServiceName(tt.input)
		if result != tt.expected {
			t.Errorf("GetSecretServiceName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetSecretKeyNameEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"service.key", "key"},
		{"nested.service.key", "service.key"}, // Gets everything after first dot
		{"no-dot", ""},
		{"", ""},
		{"service.", ""},
		{".key", "key"},
	}

	for _, tt := range tests {
		result := GetSecretKeyName(tt.input)
		if result != tt.expected {
			t.Errorf("GetSecretKeyName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
