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

func TestDeriveDenoFlagsNilContract(t *testing.T) {
	flags := DeriveDenoFlags(nil)
	if flags != nil {
		t.Errorf("expected nil flags for nil contract, got %v", flags)
	}
}

func TestDeriveDenoFlagsEmptyDependencies(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{},
	}
	flags := DeriveDenoFlags(contract)
	if flags != nil {
		t.Errorf("expected nil flags for empty dependencies, got %v", flags)
	}
}

func TestDeriveDenoFlagsFixedHostScoped(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}

	flags := DeriveDenoFlags(contract)
	if flags == nil {
		t.Fatal("expected non-nil flags for fixed-host dependency")
	}

	// Should return scoped --allow-net with specific hosts
	allowNetFlag := ""
	allowEnvFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
		if len(flag) > 11 && flag[:11] == "--allow-env" {
			allowEnvFlag = flag
		}
	}

	if allowNetFlag == "" {
		t.Error("expected --allow-net flag in derived flags")
	}

	// Should include api.github.com:443 and 0.0.0.0:8080
	if allowNetFlag != "--allow-net=0.0.0.0:8080,api.github.com:443" {
		t.Errorf("expected --allow-net=0.0.0.0:8080,api.github.com:443, got %s", allowNetFlag)
	}

	// Should include scoped --allow-env
	if allowEnvFlag != "--allow-env=DENO_DIR,HOME" {
		t.Errorf("expected --allow-env=DENO_DIR,HOME, got %s", allowEnvFlag)
	}
}

func TestDeriveDenoFlagsDynamicTargetBroad(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"external-api": {
				Protocol: "https",
				Type:     "dynamic-target",
				CIDR:     "0.0.0.0/0",
				DynPorts: []string{"443/TCP"},
			},
		},
	}

	flags := DeriveDenoFlags(contract)
	if flags == nil {
		t.Fatal("expected non-nil flags for dynamic-target dependency")
	}

	// Should return broad --allow-net (no host restrictions)
	allowNetFlag := ""
	for _, flag := range flags {
		if flag == "--allow-net" {
			allowNetFlag = flag
		}
	}

	if allowNetFlag != "--allow-net" {
		t.Errorf("expected broad --allow-net for dynamic-target, got %v", flags)
	}
}

func TestDeriveDenoFlagsMixedDependenciesBroad(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
			"external-api": {
				Protocol: "https",
				Type:     "dynamic-target",
				CIDR:     "0.0.0.0/0",
				DynPorts: []string{"443/TCP"},
			},
		},
	}

	flags := DeriveDenoFlags(contract)
	if flags == nil {
		t.Fatal("expected non-nil flags for mixed dependencies")
	}

	// Mixed (fixed + dynamic-target) should return broad --allow-net
	allowNetFlag := ""
	for _, flag := range flags {
		if flag == "--allow-net" {
			allowNetFlag = flag
		}
	}

	if allowNetFlag != "--allow-net" {
		t.Errorf("expected broad --allow-net for mixed dependencies, got %v", flags)
	}
}

func TestDeriveDenoFlagsDefaultPortResolution(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				// Port omitted, should default to 443
			},
			"postgres": {
				Protocol: "postgresql",
				Host:     "postgres.svc",
				// Port omitted, should default to 5432
			},
		},
	}

	flags := DeriveDenoFlags(contract)
	if flags == nil {
		t.Fatal("expected non-nil flags for fixed-host dependencies")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	// Should include default ports
	if allowNetFlag != "--allow-net=0.0.0.0:8080,api.github.com:443,postgres.svc:5432" {
		t.Errorf("expected ports resolved with defaults, got %s", allowNetFlag)
	}
}

func TestDeriveDenoFlagsMultipleFixedSorted(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"z-service": {
				Protocol: "https",
				Host:     "z.example.com",
				Port:     443,
			},
			"a-service": {
				Protocol: "https",
				Host:     "a.example.com",
				Port:     443,
			},
			"m-service": {
				Protocol: "postgresql",
				Host:     "m.example.com",
				Port:     5432,
			},
		},
	}

	flags := DeriveDenoFlags(contract)
	if flags == nil {
		t.Fatal("expected non-nil flags for multiple fixed-host dependencies")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	// Hosts should be sorted alphabetically
	expected := "--allow-net=0.0.0.0:8080,a.example.com:443,m.example.com:5432,z.example.com:443"
	if allowNetFlag != expected {
		t.Errorf("expected sorted hosts, got %s, want %s", allowNetFlag, expected)
	}
}

func TestDeriveDenoFlagsAlwaysIncludesLocalhost(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}

	flags := DeriveDenoFlags(contract)
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	// Should always include 0.0.0.0:8080 as first entry
	if len(allowNetFlag) < 23 || allowNetFlag[:23] != "--allow-net=0.0.0.0:808" {
		t.Errorf("expected 0.0.0.0:8080 to be included first, got %s", allowNetFlag)
	}
}

func TestDeriveDenoFlagsScopedAllowEnv(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}

	flags := DeriveDenoFlags(contract)
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	// Should include scoped --allow-env=DENO_DIR,HOME
	foundAllowEnv := false
	for _, flag := range flags {
		if flag == "--allow-env=DENO_DIR,HOME" {
			foundAllowEnv = true
			break
		}
	}

	if !foundAllowEnv {
		t.Errorf("expected --allow-env=DENO_DIR,HOME in derived flags, got %v", flags)
	}
}
