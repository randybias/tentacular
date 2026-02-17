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

	if len(rules) != 1 {
		t.Fatalf("expected 1 ingress rule for webhook, got %d", len(rules))
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

	if len(rules) != 1 {
		t.Fatalf("expected 1 ingress rule for cron, got %d", len(rules))
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

	// If ANY trigger is webhook, should use open ingress (podSelector: {})
	if len(rules) != 1 {
		t.Fatalf("expected 1 ingress rule, got %d", len(rules))
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

	if len(rules) != 1 {
		t.Fatalf("expected 1 ingress rule for manual, got %d", len(rules))
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

	if len(rules) != 1 {
		t.Fatalf("expected 1 ingress rule for queue, got %d", len(rules))
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
