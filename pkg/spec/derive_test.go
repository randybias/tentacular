package spec

import (
	"strings"
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
	// Expect 2 rules: webhook ingress + MCP health probe ingress
	if len(rules) != 2 {
		t.Fatalf("expected 2 ingress rules for webhook (webhook + MCP), got %d", len(rules))
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
	// Expect 2 rules: trigger ingress + MCP health probe ingress
	if len(rules) != 2 {
		t.Errorf("expected 2 ingress rules (trigger + MCP), got %d", len(rules))
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
	// OTel telemetry requires flags even for nil contract — never returns nil.
	flags := DeriveDenoFlags(nil, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags for nil contract (OTel requires allow-net)")
	}
	// Collector endpoint must be present
	allowNet := ""
	for _, f := range flags {
		if strings.HasPrefix(f, "--allow-net=") {
			allowNet = f
			break
		}
	}
	if !strings.Contains(allowNet, "otel-collector.tentacular-observability.svc.cluster.local:4318") {
		t.Errorf("expected OTel collector in --allow-net, got %q", allowNet)
	}
}

func TestDeriveDenoFlagsEmptyDependencies(t *testing.T) {
	// OTel telemetry requires flags even for empty dependencies — never returns nil.
	contract := &Contract{
		Dependencies: map[string]Dependency{},
	}
	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags for empty dependencies (OTel requires allow-net)")
	}
	// Collector endpoint must be present
	allowNet := ""
	for _, f := range flags {
		if strings.HasPrefix(f, "--allow-net=") {
			allowNet = f
			break
		}
	}
	if !strings.Contains(allowNet, "otel-collector.tentacular-observability.svc.cluster.local:4318") {
		t.Errorf("expected OTel collector in --allow-net, got %q", allowNet)
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

	flags := DeriveDenoFlags(contract, nil, "")
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

	// Should include api.github.com:443, 0.0.0.0:8080, and OTel collector
	if allowNetFlag != "--allow-net=0.0.0.0:8080,api.github.com:443,otel-collector.tentacular-observability.svc.cluster.local:4318" {
		t.Errorf("expected --allow-net with OTel collector, got %s", allowNetFlag)
	}

	// Should include scoped --allow-env with OTel vars
	if allowEnvFlag != "--allow-env=DENO_DIR,HOME,OTEL_DENO,OTEL_EXPORTER_OTLP_ENDPOINT,OTEL_EXPORTER_OTLP_PROTOCOL,OTEL_RESOURCE_ATTRIBUTES,OTEL_SERVICE_NAME,SPIFFE_ENDPOINT_SOCKET,SPIFFE_ID,SPIFFE_ID_PATH,SVID_CERT_PATH,TELEMETRY_SINK" {
		t.Errorf("expected --allow-env with OTel vars, got %s", allowEnvFlag)
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

	flags := DeriveDenoFlags(contract, nil, "")
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

	flags := DeriveDenoFlags(contract, nil, "")
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

	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags for fixed-host dependencies")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	// Should include default ports and OTel collector
	if allowNetFlag != "--allow-net=0.0.0.0:8080,api.github.com:443,otel-collector.tentacular-observability.svc.cluster.local:4318,postgres.svc:5432" {
		t.Errorf("expected ports resolved with defaults plus OTel collector, got %s", allowNetFlag)
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

	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags for multiple fixed-host dependencies")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	// Hosts should be sorted alphabetically, including OTel collector
	expected := "--allow-net=0.0.0.0:8080,a.example.com:443,m.example.com:5432,otel-collector.tentacular-observability.svc.cluster.local:4318,z.example.com:443"
	if allowNetFlag != expected {
		t.Errorf("expected sorted hosts with OTel collector, got %s, want %s", allowNetFlag, expected)
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

	flags := DeriveDenoFlags(contract, nil, "")
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

	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	// Should include scoped --allow-env with OTel vars
	foundAllowEnv := false
	for _, flag := range flags {
		if flag == "--allow-env=DENO_DIR,HOME,OTEL_DENO,OTEL_EXPORTER_OTLP_ENDPOINT,OTEL_EXPORTER_OTLP_PROTOCOL,OTEL_RESOURCE_ATTRIBUTES,OTEL_SERVICE_NAME,SPIFFE_ENDPOINT_SOCKET,SPIFFE_ID,SPIFFE_ID_PATH,SVID_CERT_PATH,TELEMETRY_SINK" {
			foundAllowEnv = true
			break
		}
	}

	if !foundAllowEnv {
		t.Errorf("expected --allow-env with OTel vars in derived flags, got %v", flags)
	}
}

// --- Exoskeleton Phase 1 Tests ---

func TestEgressRulesSkipExoskeletonDeps(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"tentacular-postgres": {
				Protocol: "postgresql",
				// No host/port — MCP provisions these at deploy time
			},
			"tentacular-nats": {
				Protocol: "nats",
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// Should only have DNS rules (2), no rules for tentacular-* deps
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules (DNS only), got %d: %+v", len(rules), rules)
	}
	for _, r := range rules {
		if r.Host != "kube-dns.kube-system.svc.cluster.local" {
			t.Errorf("unexpected non-DNS egress rule: %+v", r)
		}
	}
}

func TestDenoFlagsSkipExoskeletonDeps(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"tentacular-postgres": {
				Protocol: "postgresql",
			},
		},
	}

	// With only exoskeleton deps (no host/port), DeriveDenoFlags should still
	// return flags but with no dependency hosts in --allow-net
	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	// Should contain 0.0.0.0:8080 and OTel collector (no postgres host — exoskeleton-managed)
	if allowNetFlag != "--allow-net=0.0.0.0:8080,otel-collector.tentacular-observability.svc.cluster.local:4318" {
		t.Errorf("expected --allow-net with OTel collector only, got %s", allowNetFlag)
	}
}

func TestMixedDepsEgressRules(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"tentacular-postgres": {
				Protocol: "postgresql",
			},
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// Should have DNS (2) + github (1) = 3 rules, no rule for tentacular-postgres
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules (DNS + github), got %d: %+v", len(rules), rules)
	}

	foundGithub := false
	for _, r := range rules {
		if r.Host == "api.github.com" && r.Port == 443 {
			foundGithub = true
		}
		// Ensure no postgresql host leaked through
		if r.Port == 5432 {
			t.Errorf("unexpected postgresql egress rule from tentacular-* dep: %+v", r)
		}
	}
	if !foundGithub {
		t.Error("expected egress rule for github:443")
	}
}

func TestMixedDepsDenoFlags(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"tentacular-postgres": {
				Protocol: "postgresql",
			},
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}

	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	// Should include github but not any tentacular-* host
	if !strings.Contains(allowNetFlag, "api.github.com:443") {
		t.Errorf("expected api.github.com:443 in allow-net, got %s", allowNetFlag)
	}
	// Verify no postgres host leaked (tentacular-postgres has no host anyway, but
	// confirm the flag looks correct, including OTel collector)
	expected := "--allow-net=0.0.0.0:8080,api.github.com:443,otel-collector.tentacular-observability.svc.cluster.local:4318"
	if allowNetFlag != expected {
		t.Errorf("expected %s, got %s", expected, allowNetFlag)
	}
}

func TestExoskeletonDepsWithDynamicTargetDoNotTriggerBroadNet(t *testing.T) {
	// Ensure tentacular-* deps don't trigger hasDynamic even if they had type set
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"tentacular-external": {
				Protocol: "https",
				Type:     "dynamic-target",
				CIDR:     "0.0.0.0/0",
				DynPorts: []string{"443/TCP"},
			},
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}

	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	// tentacular-external should be skipped, so hasDynamic should be false
	// and we should get scoped --allow-net
	for _, flag := range flags {
		if flag == "--allow-net" {
			t.Error("expected scoped --allow-net, got broad --allow-net (tentacular-* dynamic dep should be skipped)")
		}
	}
}

func TestDeriveDenoFlagsSidecarLocalhost(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}
	sidecars := []SidecarSpec{
		{Name: "ffmpeg", Image: "ffmpeg:latest", Port: 9000},
	}

	flags := DeriveDenoFlags(contract, sidecars, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	if !strings.Contains(allowNetFlag, "localhost:9000") {
		t.Errorf("expected localhost:9000 in --allow-net, got: %s", allowNetFlag)
	}
}

func TestDeriveDenoFlagsSidecarSharedVolume(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}
	sidecars := []SidecarSpec{
		{Name: "ffmpeg", Image: "ffmpeg:latest", Port: 9000},
	}

	flags := DeriveDenoFlags(contract, sidecars, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowReadFlag := ""
	allowWriteFlag := ""
	for _, flag := range flags {
		if len(flag) > 13 && flag[:13] == "--allow-read=" {
			allowReadFlag = flag
		}
		if len(flag) > 14 && flag[:14] == "--allow-write=" {
			allowWriteFlag = flag
		}
	}

	if allowReadFlag != "--allow-read=/app,/shared" {
		t.Errorf("expected --allow-read=/app,/shared, got: %s", allowReadFlag)
	}
	if allowWriteFlag != "--allow-write=/tmp,/shared" {
		t.Errorf("expected --allow-write=/tmp,/shared, got: %s", allowWriteFlag)
	}
}

func TestDeriveDenoFlagsMultipleSidecars(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"github": {
				Protocol: "https",
				Host:     "api.github.com",
				Port:     443,
			},
		},
	}
	sidecars := []SidecarSpec{
		{Name: "ffmpeg", Image: "ffmpeg:latest", Port: 9000},
		{Name: "chrome", Image: "chrome:latest", Port: 9001},
	}

	flags := DeriveDenoFlags(contract, sidecars, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowNetFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
	}

	if !strings.Contains(allowNetFlag, "localhost:9000") {
		t.Errorf("expected localhost:9000 in --allow-net, got: %s", allowNetFlag)
	}
	if !strings.Contains(allowNetFlag, "localhost:9001") {
		t.Errorf("expected localhost:9001 in --allow-net, got: %s", allowNetFlag)
	}
}

func TestDeriveDenoFlagsSidecarsNoContract(t *testing.T) {
	sidecars := []SidecarSpec{
		{Name: "ffmpeg", Image: "ffmpeg:latest", Port: 9000},
	}

	flags := DeriveDenoFlags(nil, sidecars, "")
	if flags == nil {
		t.Fatal("expected non-nil flags for sidecars without contract")
	}

	allowNetFlag := ""
	allowReadFlag := ""
	allowWriteFlag := ""
	for _, flag := range flags {
		if len(flag) > 11 && flag[:11] == "--allow-net" {
			allowNetFlag = flag
		}
		if len(flag) > 13 && flag[:13] == "--allow-read=" {
			allowReadFlag = flag
		}
		if len(flag) > 14 && flag[:14] == "--allow-write=" {
			allowWriteFlag = flag
		}
	}

	if !strings.Contains(allowNetFlag, "localhost:9000") {
		t.Errorf("expected localhost:9000 in --allow-net, got: %s", allowNetFlag)
	}
	if allowReadFlag != "--allow-read=/app,/shared" {
		t.Errorf("expected --allow-read=/app,/shared, got: %s", allowReadFlag)
	}
	if allowWriteFlag != "--allow-write=/tmp,/shared" {
		t.Errorf("expected --allow-write=/tmp,/shared, got: %s", allowWriteFlag)
	}
}

func TestDeriveDenoFlagsZeroDepWithProxyHost(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{},
	}

	proxyHost := "esm-sh.tentacular-support.svc.cluster.local:8080"
	flags := DeriveDenoFlags(contract, nil, proxyHost)
	if flags == nil {
		t.Fatal("expected non-nil flags for zero-dep contract with proxy host")
	}

	allowImportFlag := ""
	allowReadFlag := ""
	allowWriteFlag := ""
	allowNetFlag := ""
	for _, flag := range flags {
		if strings.HasPrefix(flag, "--allow-import=") {
			allowImportFlag = flag
		}
		if strings.HasPrefix(flag, "--allow-read=") {
			allowReadFlag = flag
		}
		if strings.HasPrefix(flag, "--allow-write=") {
			allowWriteFlag = flag
		}
		if strings.HasPrefix(flag, "--allow-net=") {
			allowNetFlag = flag
		}
	}

	expectedImport := "--allow-import=deno.land:443,esm-sh.tentacular-support.svc.cluster.local:8080"
	if allowImportFlag != expectedImport {
		t.Errorf("expected %s, got %s", expectedImport, allowImportFlag)
	}

	// No sidecars: read should be /app only, write should be /tmp only
	if allowReadFlag != "--allow-read=/app" {
		t.Errorf("expected --allow-read=/app, got %s", allowReadFlag)
	}
	if allowWriteFlag != "--allow-write=/tmp" {
		t.Errorf("expected --allow-write=/tmp, got %s", allowWriteFlag)
	}

	// allow-net should include the proxy host and 0.0.0.0:8080
	if !strings.Contains(allowNetFlag, proxyHost) {
		t.Errorf("expected proxy host in --allow-net, got %s", allowNetFlag)
	}
}

func TestDeriveDenoFlagsZeroDepWithSidecarsAndProxy(t *testing.T) {
	contract := &Contract{
		Dependencies: map[string]Dependency{},
	}
	sidecars := []SidecarSpec{
		{Name: "postgres", Image: "postgres:16", Port: 5432},
	}

	proxyHost := "esm-sh.tentacular-support.svc.cluster.local:8080"
	flags := DeriveDenoFlags(contract, sidecars, proxyHost)
	if flags == nil {
		t.Fatal("expected non-nil flags for zero-dep contract with sidecars and proxy")
	}

	allowReadFlag := ""
	allowWriteFlag := ""
	allowNetFlag := ""
	for _, flag := range flags {
		if strings.HasPrefix(flag, "--allow-read=") {
			allowReadFlag = flag
		}
		if strings.HasPrefix(flag, "--allow-write=") {
			allowWriteFlag = flag
		}
		if strings.HasPrefix(flag, "--allow-net=") {
			allowNetFlag = flag
		}
	}

	// Sidecars present: /shared should be included
	if allowReadFlag != "--allow-read=/app,/shared" {
		t.Errorf("expected --allow-read=/app,/shared, got %s", allowReadFlag)
	}
	if allowWriteFlag != "--allow-write=/tmp,/shared" {
		t.Errorf("expected --allow-write=/tmp,/shared, got %s", allowWriteFlag)
	}

	// allow-net should include both localhost:5432 and the proxy host
	if !strings.Contains(allowNetFlag, "localhost:5432") {
		t.Errorf("expected localhost:5432 in --allow-net, got %s", allowNetFlag)
	}
	if !strings.Contains(allowNetFlag, proxyHost) {
		t.Errorf("expected proxy host in --allow-net, got %s", allowNetFlag)
	}
}

func TestDeriveDenoFlagsNilContractWithProxy(t *testing.T) {
	proxyHost := "esm-sh.tentacular-support.svc.cluster.local:8080"
	flags := DeriveDenoFlags(nil, nil, proxyHost)
	if flags == nil {
		t.Fatal("expected non-nil flags for nil contract with proxy host")
	}

	allowImportFlag := ""
	allowReadFlag := ""
	allowWriteFlag := ""
	for _, flag := range flags {
		if strings.HasPrefix(flag, "--allow-import=") {
			allowImportFlag = flag
		}
		if strings.HasPrefix(flag, "--allow-read=") {
			allowReadFlag = flag
		}
		if strings.HasPrefix(flag, "--allow-write=") {
			allowWriteFlag = flag
		}
	}

	// Should behave same as empty deps case
	expectedImport := "--allow-import=deno.land:443,esm-sh.tentacular-support.svc.cluster.local:8080"
	if allowImportFlag != expectedImport {
		t.Errorf("expected %s, got %s", expectedImport, allowImportFlag)
	}
	if allowReadFlag != "--allow-read=/app" {
		t.Errorf("expected --allow-read=/app, got %s", allowReadFlag)
	}
	if allowWriteFlag != "--allow-write=/tmp" {
		t.Errorf("expected --allow-write=/tmp, got %s", allowWriteFlag)
	}
}

func TestDeriveDenoFlagsDynamicTargetWithSidecars(t *testing.T) {
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
	sidecars := []SidecarSpec{
		{Name: "ffmpeg", Image: "ffmpeg:latest", Port: 9000},
	}

	flags := DeriveDenoFlags(contract, sidecars, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	// Dynamic target should still use broad --allow-net
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

// --- OTel Phase 1 Tests ---

func TestDeriveDenoFlagsOTelAllowEnvScoped(t *testing.T) {
	// Both scoped and dynamic paths must include OTel vars in --allow-env.
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"api": {Protocol: "https", Host: "api.example.com", Port: 443},
		},
	}
	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	wantEnv := "--allow-env=DENO_DIR,HOME,OTEL_DENO,OTEL_EXPORTER_OTLP_ENDPOINT,OTEL_EXPORTER_OTLP_PROTOCOL,OTEL_RESOURCE_ATTRIBUTES,OTEL_SERVICE_NAME,SPIFFE_ENDPOINT_SOCKET,SPIFFE_ID,SPIFFE_ID_PATH,SVID_CERT_PATH,TELEMETRY_SINK"
	found := false
	for _, f := range flags {
		if f == wantEnv {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in flags, got %v", wantEnv, flags)
	}
}

func TestDeriveDenoFlagsOTelAllowEnvDynamic(t *testing.T) {
	// Dynamic path must also include OTel vars in --allow-env.
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"ext": {Protocol: "https", Type: "dynamic-target", CIDR: "0.0.0.0/0", DynPorts: []string{"443/TCP"}},
		},
	}
	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	wantEnv := "--allow-env=DENO_DIR,HOME,OTEL_DENO,OTEL_EXPORTER_OTLP_ENDPOINT,OTEL_EXPORTER_OTLP_PROTOCOL,OTEL_RESOURCE_ATTRIBUTES,OTEL_SERVICE_NAME,SPIFFE_ENDPOINT_SOCKET,SPIFFE_ID,SPIFFE_ID_PATH,SVID_CERT_PATH,TELEMETRY_SINK"
	found := false
	for _, f := range flags {
		if f == wantEnv {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in dynamic flags, got %v", wantEnv, flags)
	}
}

func TestDeriveDenoFlagsOTelCollectorInAllowNet(t *testing.T) {
	// OTel collector endpoint must appear in --allow-net for the scoped path.
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"api": {Protocol: "https", Host: "api.example.com", Port: 443},
		},
	}
	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowNet := ""
	for _, f := range flags {
		if strings.HasPrefix(f, "--allow-net=") {
			allowNet = f
			break
		}
	}
	if !strings.Contains(allowNet, "otel-collector.tentacular-observability.svc.cluster.local:4318") {
		t.Errorf("expected OTel collector in --allow-net, got %q", allowNet)
	}
}

func TestDeriveDenoFlagsOTelCollectorNoDependencies(t *testing.T) {
	// OTel collector must be in --allow-net even when there are no contract dependencies.
	flags := DeriveDenoFlags(nil, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags for nil contract (OTel requires flags)")
	}

	allowNet := ""
	for _, f := range flags {
		if strings.HasPrefix(f, "--allow-net=") {
			allowNet = f
			break
		}
	}
	if !strings.Contains(allowNet, "otel-collector.tentacular-observability.svc.cluster.local:4318") {
		t.Errorf("expected OTel collector in --allow-net for no-dep workflow, got %q", allowNet)
	}
}

func TestDeriveDenoFlagsOTelSpiffeVarsPreserved(t *testing.T) {
	// Existing SPIFFE and TELEMETRY_SINK vars must still be present alongside OTel vars.
	contract := &Contract{
		Dependencies: map[string]Dependency{
			"api": {Protocol: "https", Host: "api.example.com", Port: 443},
		},
	}
	flags := DeriveDenoFlags(contract, nil, "")
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	allowEnv := ""
	for _, f := range flags {
		if strings.HasPrefix(f, "--allow-env=") {
			allowEnv = f
			break
		}
	}

	existingVars := []string{
		"DENO_DIR", "HOME", "SPIFFE_ENDPOINT_SOCKET", "SPIFFE_ID",
		"SPIFFE_ID_PATH", "SVID_CERT_PATH", "TELEMETRY_SINK",
	}
	for _, v := range existingVars {
		if !strings.Contains(allowEnv, v) {
			t.Errorf("expected existing var %q preserved in --allow-env, got %q", v, allowEnv)
		}
	}
}
