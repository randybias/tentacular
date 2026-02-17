package spec

import (
	"testing"
)

// --- Group 2.4: Dynamic Target Egress Generation Tests ---

func TestDynamicTargetGeneratesCIDREgress(t *testing.T) {
	contract := &Contract{
		Version: "1",
		Dependencies: map[string]Dependency{
			"probe-targets": {
				Type:     "dynamic-target",
				Protocol: "https",
				CIDR:     "0.0.0.0/0",
				DynPorts: []string{"443/TCP", "80/TCP"},
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// Should have DNS (2) + CIDR egress (2 ports)
	expectedRuleCount := 4
	if len(rules) != expectedRuleCount {
		t.Errorf("expected %d rules (DNS + 2 CIDR), got %d rules", expectedRuleCount, len(rules))
		for i, rule := range rules {
			t.Logf("  Rule %d: Host=%s Port=%d Protocol=%s", i+1, rule.Host, rule.Port, rule.Protocol)
		}
	}

	// Verify CIDR rules are generated
	foundCIDR443 := false
	foundCIDR80 := false
	for _, rule := range rules {
		if rule.Host == "0.0.0.0/0" && rule.Port == 443 && rule.Protocol == "TCP" {
			foundCIDR443 = true
		}
		if rule.Host == "0.0.0.0/0" && rule.Port == 80 && rule.Protocol == "TCP" {
			foundCIDR80 = true
		}
	}

	if !foundCIDR443 {
		t.Error("expected CIDR egress rule for port 443")
	}
	if !foundCIDR80 {
		t.Error("expected CIDR egress rule for port 80")
	}
}

func TestDynamicTargetMixedWithStaticDependencies(t *testing.T) {
	contract := &Contract{
		Version: "1",
		Dependencies: map[string]Dependency{
			"probe-targets": {
				Type:     "dynamic-target",
				Protocol: "https",
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

	rules := DeriveEgressRules(contract)

	// Should have DNS (2) + github (1) + CIDR (1)
	expectedRuleCount := 4
	if len(rules) != expectedRuleCount {
		t.Errorf("expected %d rules (DNS + github + CIDR), got %d rules", expectedRuleCount, len(rules))
		for i, rule := range rules {
			t.Logf("  Rule %d: Host=%s Port=%d Protocol=%s", i+1, rule.Host, rule.Port, rule.Protocol)
		}
	}

	// Verify both github and CIDR rules exist
	foundGithub := false
	foundCIDR := false
	for _, rule := range rules {
		if rule.Host == "api.github.com" && rule.Port == 443 {
			foundGithub = true
		}
		if rule.Host == "0.0.0.0/0" && rule.Port == 443 {
			foundCIDR = true
		}
	}

	if !foundGithub {
		t.Error("expected github egress rule")
	}
	if !foundCIDR {
		t.Error("expected CIDR egress rule for dynamic-target")
	}
}

func TestDynamicTargetNoCIDRSkipsEgress(t *testing.T) {
	contract := &Contract{
		Version: "1",
		Dependencies: map[string]Dependency{
			"probe-targets": {
				Type:     "dynamic-target",
				Protocol: "https",
				// No CIDR or DynPorts — should generate no extra egress
			},
		},
	}

	rules := DeriveEgressRules(contract)

	// Should only have DNS rules
	if len(rules) != 2 {
		t.Errorf("expected 2 rules (DNS only) when dynamic-target has no CIDR, got %d", len(rules))
	}
}
