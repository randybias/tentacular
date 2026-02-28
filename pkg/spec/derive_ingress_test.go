package spec

import (
	"testing"
)

// --- Ingress Rule Derivation Tests ---

func TestDeriveIngressRulesWebhookTrigger(t *testing.T) {
	wf := &Workflow{
		Name:    "webhook-workflow",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "webhook", Path: "/api/v1/hook"},
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

	rule := rules[0]
	if rule.Port != 8080 {
		t.Errorf("expected port 8080, got %d", rule.Port)
	}
	if rule.Protocol != "TCP" {
		t.Errorf("expected protocol TCP, got %s", rule.Protocol)
	}

	// CRITICAL: Webhook triggers should use podSelector: {} (no label restriction)
	// This allows external traffic via Ingress/LoadBalancer to reach the pod
	if rule.FromLabels != nil {
		t.Errorf("expected FromLabels=nil for webhook trigger (podSelector: {}), got %v", rule.FromLabels)
		t.Logf("Bug: Webhook ingress is label-scoped, preventing external traffic")
	}

	// Second rule: MCP health probe ingress from tentacular-system
	mcpRule := rules[1]
	if mcpRule.Port != 8080 {
		t.Errorf("expected MCP ingress port 8080, got %d", mcpRule.Port)
	}
	if mcpRule.FromNamespaceLabels["kubernetes.io/metadata.name"] != "tentacular-system" {
		t.Errorf("expected MCP ingress from tentacular-system, got %v", mcpRule.FromNamespaceLabels)
	}
}

func TestDeriveIngressRulesNonWebhookTrigger(t *testing.T) {
	wf := &Workflow{
		Name:    "cron-workflow",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "cron", Schedule: "*/5 * * * *"},
		},
		Nodes: map[string]NodeSpec{
			"task": {Path: "./task.ts"},
		},
	}

	rules := DeriveIngressRules(wf)

	// Expect 2 rules: trigger ingress + MCP health probe ingress
	if len(rules) != 2 {
		t.Fatalf("expected 2 ingress rules for cron (trigger + MCP), got %d", len(rules))
	}

	rule := rules[0]
	if rule.Port != 8080 {
		t.Errorf("expected port 8080, got %d", rule.Port)
	}

	// Non-webhook triggers should use label-scoped ingress (internal trigger pods only)
	if rule.FromLabels == nil {
		t.Error("expected FromLabels to be non-nil for non-webhook trigger (label-scoped)")
	} else {
		if rule.FromLabels["tentacular.dev/role"] != "trigger" {
			t.Errorf("expected role=trigger label, got %v", rule.FromLabels)
		}
	}

	// Second rule: MCP health probe ingress from tentacular-system
	mcpRule := rules[1]
	if mcpRule.Port != 8080 {
		t.Errorf("expected MCP ingress port 8080, got %d", mcpRule.Port)
	}
	if mcpRule.FromNamespaceLabels["kubernetes.io/metadata.name"] != "tentacular-system" {
		t.Errorf("expected MCP ingress from tentacular-system, got %v", mcpRule.FromNamespaceLabels)
	}
}

func TestDeriveIngressRulesMixedTriggers(t *testing.T) {
	wf := &Workflow{
		Name:    "mixed-workflow",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "cron", Schedule: "0 * * * *"},
			{Type: "webhook", Path: "/hook"},
			{Type: "manual"},
		},
		Nodes: map[string]NodeSpec{
			"task": {Path: "./task.ts"},
		},
	}

	rules := DeriveIngressRules(wf)

	// If ANY trigger is webhook, should use open ingress (podSelector: {}) + MCP ingress
	if len(rules) != 2 {
		t.Fatalf("expected 2 ingress rules (webhook + MCP), got %d", len(rules))
	}

	rule := rules[0]
	if rule.FromLabels != nil {
		t.Errorf("expected FromLabels=nil when webhook trigger present, got %v", rule.FromLabels)
	}
}

func TestDeriveIngressRulesManualTriggerOnly(t *testing.T) {
	wf := &Workflow{
		Name:    "manual-workflow",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "manual"},
		},
		Nodes: map[string]NodeSpec{
			"task": {Path: "./task.ts"},
		},
	}

	rules := DeriveIngressRules(wf)

	// Expect 2 rules: trigger ingress + MCP health probe ingress
	if len(rules) != 2 {
		t.Fatalf("expected 2 ingress rules for manual (trigger + MCP), got %d", len(rules))
	}

	rule := rules[0]
	// Manual triggers use label-scoped ingress (runner Job needs access)
	if rule.FromLabels == nil {
		t.Error("expected FromLabels to be non-nil for manual trigger (label-scoped)")
	} else {
		if rule.FromLabels["tentacular.dev/role"] != "trigger" {
			t.Errorf("expected role=trigger label, got %v", rule.FromLabels)
		}
	}
}

func TestDeriveIngressRulesQueueTrigger(t *testing.T) {
	wf := &Workflow{
		Name:    "queue-workflow",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "queue", Subject: "events.workflow"},
		},
		Nodes: map[string]NodeSpec{
			"handler": {Path: "./handler.ts"},
		},
	}

	rules := DeriveIngressRules(wf)

	// Expect 2 rules: trigger ingress + MCP health probe ingress
	if len(rules) != 2 {
		t.Fatalf("expected 2 ingress rules for queue (trigger + MCP), got %d", len(rules))
	}

	rule := rules[0]
	// Queue triggers use label-scoped ingress (queue consumer Job needs access)
	if rule.FromLabels == nil {
		t.Error("expected FromLabels to be non-nil for queue trigger (label-scoped)")
	} else {
		if rule.FromLabels["tentacular.dev/role"] != "trigger" {
			t.Errorf("expected role=trigger label, got %v", rule.FromLabels)
		}
	}
}

func TestDeriveIngressRulesMCPHealthProbe(t *testing.T) {
	wf := &Workflow{
		Name:    "any-workflow",
		Version: "1.0",
		Triggers: []Trigger{
			{Type: "manual"},
		},
		Nodes: map[string]NodeSpec{
			"task": {Path: "./task.ts"},
		},
	}

	rules := DeriveIngressRules(wf)

	// Always expect MCP health probe rule as last rule
	if len(rules) < 1 {
		t.Fatal("expected at least 1 ingress rule")
	}
	mcpRule := rules[len(rules)-1]
	if mcpRule.Port != 8080 {
		t.Errorf("expected MCP ingress port 8080, got %d", mcpRule.Port)
	}
	if mcpRule.Protocol != "TCP" {
		t.Errorf("expected MCP ingress protocol TCP, got %s", mcpRule.Protocol)
	}
	if mcpRule.FromNamespaceLabels == nil {
		t.Error("expected MCP ingress to have FromNamespaceLabels")
	} else if mcpRule.FromNamespaceLabels["kubernetes.io/metadata.name"] != "tentacular-system" {
		t.Errorf("expected MCP ingress from tentacular-system, got %v", mcpRule.FromNamespaceLabels)
	}
	// Belt-and-suspenders: MCP probe also scopes to the tentacular-mcp pod label
	if mcpRule.FromLabels == nil {
		t.Error("expected MCP ingress FromLabels to be set (app.kubernetes.io/name: tentacular-mcp)")
	} else if mcpRule.FromLabels["app.kubernetes.io/name"] != "tentacular-mcp" {
		t.Errorf("expected MCP ingress FromLabels app.kubernetes.io/name=tentacular-mcp, got %v", mcpRule.FromLabels)
	}
}
