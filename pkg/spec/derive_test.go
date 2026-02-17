package spec

import (
	"testing"
)

func TestDeriveSecretsFromContract(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Auth: &DependencyAuth{
					Secret: "github.token",
				},
			},
			"postgres": {
				Protocol: "postgresql",
				Host:     "postgres.svc",
				Database: "appdb",
				User:     "postgres",
				Auth: &DependencyAuth{
					Secret: "postgres.password",
				},
			},
		},
	}

	secrets := DeriveSecrets(contract)
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(secrets))
	}

	// Check that both secrets are present (order may vary)
	secretMap := make(map[string]bool)
	for _, s := range secrets {
		secretMap[s] = true
	}

	if !secretMap["github.token"] {
		t.Error("expected github.token in derived secrets")
	}
	if !secretMap["postgres.password"] {
		t.Error("expected postgres.password in derived secrets")
	}
}

func TestDeriveSecretsNilContract(t *testing.T) {
	secrets := DeriveSecrets(nil)
	if secrets != nil {
		t.Errorf("expected nil secrets for nil contract, got %v", secrets)
	}
}

func TestDeriveSecretsNoAuth(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"public-api": {
				Protocol: "https",
				Host:     "api.example.com",
			},
		},
	}

	secrets := DeriveSecrets(contract)
	if len(secrets) != 0 {
		t.Errorf("expected no secrets for dependencies without auth, got %v", secrets)
	}
}

func TestDeriveEgressRulesFromContract(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
			"postgres": {
				Protocol: "postgresql",
				Host:     "postgres.svc.cluster.local",
				Port:     5432,
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// Should have DNS (2 rules) + github + postgres = 4 rules
	if len(rules) < 4 {
		t.Fatalf("expected at least 4 egress rules (DNS + 2 deps), got %d", len(rules))
	}

	// Check DNS rules are present
	dnsCount := 0
	for _, r := range rules {
		if r.Port == 53 && r.Host == "kube-dns.kube-system.svc.cluster.local" {
			dnsCount++
		}
	}
	if dnsCount != 2 {
		t.Errorf("expected 2 DNS egress rules (UDP + TCP), got %d", dnsCount)
	}

	// Check github rule
	foundGithub := false
	for _, r := range rules {
		if r.Host == "api.github.com" && r.Port == 443 && r.Protocol == "TCP" {
			foundGithub = true
		}
	}
	if !foundGithub {
		t.Error("expected egress rule for github:443")
	}

	// Check postgres rule
	foundPostgres := false
	for _, r := range rules {
		if r.Host == "postgres.svc.cluster.local" && r.Port == 5432 && r.Protocol == "TCP" {
			foundPostgres = true
		}
	}
	if !foundPostgres {
		t.Error("expected egress rule for postgres:5432")
	}
}

func TestDeriveEgressRulesWithDefaultPorts(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				// Port omitted, should default to 443
			},
			"nats": {
				Protocol: "nats",
				Host:     "nats.svc",
				// Port omitted, should default to 4222
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// Check that default ports were applied
	foundHTTPS := false
	foundNATS := false
	for _, r := range rules {
		if r.Host == "api.github.com" && r.Port == 443 {
			foundHTTPS = true
		}
		if r.Host == "nats.svc" && r.Port == 4222 {
			foundNATS = true
		}
	}

	if !foundHTTPS {
		t.Error("expected https default port 443 to be applied")
	}
	if !foundNATS {
		t.Error("expected nats default port 4222 to be applied")
	}
}

func TestDeriveIngressRulesWebhook(t *testing.T) {
	wf := &Workflow{
		Name:    "test",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "webhook", Path: "/hook"},
		},
		Nodes: map[string]NodeSpec{
			"handler": {Path: "./handler.ts"},
		},
	}

	rules := DeriveIngressRules(wf)
	if len(rules) != 1 {
		t.Fatalf("expected 1 ingress rule for webhook, got %d", len(rules))
	}

	if rules[0].Port != 8080 || rules[0].Protocol != "TCP" {
		t.Errorf("expected ingress on port 8080/TCP, got %d/%s", rules[0].Port, rules[0].Protocol)
	}
}

func TestDeriveIngressRulesNoWebhook(t *testing.T) {
	wf := &Workflow{
		Name:    "test",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "cron", Schedule: "0 * * * *"},
			{Type: "manual"},
		},
		Nodes: map[string]NodeSpec{
			"handler": {Path: "./handler.ts"},
		},
	}

	rules := DeriveIngressRules(wf)
	if len(rules) != 1 {
		t.Errorf("expected 1 ingress rule (namespace-local port 8080), got %d", len(rules))
	}
	if len(rules) > 0 && rules[0].Port != 8080 {
		t.Errorf("expected ingress port 8080, got %d", rules[0].Port)
	}
}

func TestApplyDefaultPorts(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
			},
			"postgres": {
				Protocol: "postgresql",
				Host:     "postgres.svc",
				Port:     9999, // Explicit port should not be overridden
			},
		},
	}

	ApplyDefaultPorts(contract)

	if contract.Dependencies["github"].Port != 443 {
		t.Errorf("expected default port 443 for https, got %d", contract.Dependencies["github"].Port)
	}
	if contract.Dependencies["postgres"].Port != 9999 {
		t.Errorf("expected explicit port 9999 to be preserved, got %d", contract.Dependencies["postgres"].Port)
	}
}

func TestGetSecretServiceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.token", "github"},
		{"postgres.password", "postgres"},
		{"azure.sas_token", "azure"},
		{"invalid", ""},
	}

	for _, tt := range tests {
		result := GetSecretServiceName(tt.input)
		if result != tt.expected {
			t.Errorf("GetSecretServiceName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetSecretKeyName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.token", "token"},
		{"postgres.password", "password"},
		{"azure.sas_token", "sas_token"},
		{"invalid", ""},
	}

	for _, tt := range tests {
		result := GetSecretKeyName(tt.input)
		if result != tt.expected {
			t.Errorf("GetSecretKeyName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
