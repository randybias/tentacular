package k8s

import (
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

// TestE2E_MixedDepsFullPipeline tests the full pipeline with mixed dependency types
// (fixed-host + dynamic-target), matching the ai-news-roundup pattern.
func TestE2E_MixedDepsFullPipeline(t *testing.T) {
	yamlContent := `
name: ai-news-roundup
version: "1.0"
description: "Fetches news feeds, filters recent articles, summarizes with LLM, notifies Slack"

triggers:
  - type: cron
    schedule: "0 8 * * *"

nodes:
  fetch-feeds:
    path: ./nodes/fetch-feeds.ts
  filter-24h:
    path: ./nodes/filter-24h.ts
  summarize-llm:
    path: ./nodes/summarize-llm.ts
  notify-slack:
    path: ./nodes/notify-slack.ts

edges:
  - from: fetch-feeds
    to: filter-24h
  - from: filter-24h
    to: summarize-llm
  - from: summarize-llm
    to: notify-slack

contract:
  version: "1"
  dependencies:
    openai-api:
      protocol: https
      host: api.openai.com
      port: 443
      auth:
        type: bearer-token
        secret: openai.api_key
    slack:
      protocol: https
      host: hooks.slack.com
      port: 443
      auth:
        type: webhook-url
        secret: slack.webhook_url
    news-sources:
      protocol: https
      type: dynamic-target
      cidr: "0.0.0.0/0"
      dynPorts:
        - "443/TCP"
`

	// Step 1: Parse YAML
	wf, warnings := spec.Parse([]byte(yamlContent))
	if wf == nil {
		t.Fatalf("spec.Parse failed, warnings: %v", warnings)
	}

	// Step 2: Verify contract was parsed with 3 dependencies
	if wf.Contract == nil {
		t.Fatal("expected contract to be parsed")
	}
	if len(wf.Contract.Dependencies) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(wf.Contract.Dependencies))
	}

	// Step 3: Verify DeriveDenoFlags returns broad --allow-net (dynamic-target present)
	denoFlags := spec.DeriveDenoFlags(wf.Contract, "")
	if denoFlags == nil {
		t.Fatal("expected non-nil DeriveDenoFlags result for contract with dependencies")
	}

	// Should contain broad --allow-net (not scoped with =)
	foundBroadNet := false
	foundScopedNet := false
	for _, flag := range denoFlags {
		if flag == "--allow-net" {
			foundBroadNet = true
		}
		if strings.HasPrefix(flag, "--allow-net=") {
			foundScopedNet = true
		}
	}
	if !foundBroadNet {
		t.Errorf("expected broad --allow-net for mixed deps (dynamic-target present), got %v", denoFlags)
	}
	if foundScopedNet {
		t.Error("expected NO scoped --allow-net= when dynamic-target is present")
	}

	// Should have scoped --allow-env
	foundScopedEnv := false
	for _, flag := range denoFlags {
		if flag == "--allow-env=DENO_DIR,HOME" {
			foundScopedEnv = true
		}
	}
	if !foundScopedEnv {
		t.Error("expected --allow-env=DENO_DIR,HOME in derived flags")
	}

	// Step 4: Verify derived secrets
	secrets := spec.DeriveSecrets(wf.Contract)
	if len(secrets) != 2 {
		t.Fatalf("expected 2 derived secrets (openai + slack), got %d: %v", len(secrets), secrets)
	}
	secretSet := make(map[string]bool)
	for _, s := range secrets {
		secretSet[s] = true
	}
	if !secretSet["openai.api_key"] {
		t.Error("expected derived secret openai.api_key")
	}
	if !secretSet["slack.webhook_url"] {
		t.Error("expected derived secret slack.webhook_url")
	}

	// Step 5: Verify egress rules
	egressRules := spec.DeriveEgressRules(wf.Contract)
	// 2 DNS (UDP+TCP) + 2 fixed hosts + 1 dynamic-target CIDR = 5
	if len(egressRules) < 5 {
		t.Fatalf("expected at least 5 egress rules, got %d", len(egressRules))
	}

	hasDNS := false
	hasOpenAI := false
	hasSlack := false
	hasDynamicCIDR := false
	for _, r := range egressRules {
		if r.Port == 53 {
			hasDNS = true
		}
		if r.Host == "api.openai.com" && r.Port == 443 {
			hasOpenAI = true
		}
		if r.Host == "hooks.slack.com" && r.Port == 443 {
			hasSlack = true
		}
		if r.Host == "0.0.0.0/0" && r.Port == 443 {
			hasDynamicCIDR = true
		}
	}
	if !hasDNS {
		t.Error("expected DNS egress rule")
	}
	if !hasOpenAI {
		t.Error("expected egress rule for api.openai.com:443")
	}
	if !hasSlack {
		t.Error("expected egress rule for hooks.slack.com:443")
	}
	if !hasDynamicCIDR {
		t.Error("expected egress rule for dynamic-target CIDR 0.0.0.0/0:443")
	}

	// Step 6: Generate NetworkPolicy
	manifest := GenerateNetworkPolicy(wf, "tentacular-dev", "")
	if manifest == nil {
		t.Fatal("expected non-nil NetworkPolicy manifest")
	}

	content := manifest.Content
	if !strings.Contains(content, "kind: NetworkPolicy") {
		t.Error("expected kind: NetworkPolicy")
	}
	if !strings.Contains(content, "namespace: tentacular-dev") {
		t.Error("expected namespace: tentacular-dev")
	}
	if !strings.Contains(content, "port: 53") {
		t.Error("expected DNS port 53 in NetworkPolicy")
	}
	if !strings.Contains(content, "port: 443") {
		t.Error("expected HTTPS port 443 in NetworkPolicy")
	}
	// Dynamic-target generates CIDR egress
	if !strings.Contains(content, "cidr: 0.0.0.0/0") {
		t.Error("expected CIDR 0.0.0.0/0 in NetworkPolicy for dynamic-target")
	}

	// Step 7: Verify ingress (cron trigger = label-scoped)
	ingressRules := spec.DeriveIngressRules(wf)
	if len(ingressRules) != 1 {
		t.Fatalf("expected 1 ingress rule, got %d", len(ingressRules))
	}
	if ingressRules[0].FromLabels == nil {
		t.Error("expected label-scoped ingress for cron trigger, got open (nil FromLabels)")
	}
	if ingressRules[0].FromLabels["tentacular.dev/role"] != "trigger" {
		t.Error("expected FromLabels tentacular.dev/role: trigger")
	}
}

// TestE2E_FixedHostOnlyFullPipeline tests the full pipeline with only fixed-host dependencies.
func TestE2E_FixedHostOnlyFullPipeline(t *testing.T) {
	yamlContent := `
name: github-digest
version: "1.0"

triggers:
  - type: cron
    schedule: "0 9 * * *"

nodes:
  fetch-repos:
    path: ./nodes/fetch-repos.ts
  summarize:
    path: ./nodes/summarize.ts
  notify:
    path: ./nodes/notify.ts

edges:
  - from: fetch-repos
    to: summarize
  - from: summarize
    to: notify

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
    slack:
      protocol: https
      host: hooks.slack.com
      port: 443
      auth:
        type: webhook-url
        secret: slack.webhook_url
`

	wf, warnings := spec.Parse([]byte(yamlContent))
	if wf == nil {
		t.Fatalf("spec.Parse failed, warnings: %v", warnings)
	}

	// DeriveDenoFlags should return scoped --allow-net= with specific hosts
	denoFlags := spec.DeriveDenoFlags(wf.Contract, "")
	if denoFlags == nil {
		t.Fatal("expected non-nil DeriveDenoFlags result")
	}

	allowNetFlag := ""
	for _, flag := range denoFlags {
		if strings.HasPrefix(flag, "--allow-net") {
			allowNetFlag = flag
		}
	}

	// Must be scoped (with =), sorted alphabetically, 0.0.0.0:8080 always first
	expected := "--allow-net=0.0.0.0:8080,api.github.com:443,hooks.slack.com:443"
	if allowNetFlag != expected {
		t.Errorf("expected %s, got %s", expected, allowNetFlag)
	}

	// NetworkPolicy should have host-specific egress only (no CIDR wildcard)
	manifest := GenerateNetworkPolicy(wf, "tentacular-dev", "")
	if manifest == nil {
		t.Fatal("expected non-nil NetworkPolicy manifest")
	}

	content := manifest.Content
	// Fixed-host deps use 0.0.0.0/0 with RFC1918 exclusions (v1 pragmatic approach)
	if !strings.Contains(content, "cidr: 0.0.0.0/0") {
		t.Error("expected cidr: 0.0.0.0/0 for external host egress")
	}
	// Must have RFC1918 exclusions for external hosts
	if !strings.Contains(content, "10.0.0.0/8") {
		t.Error("expected RFC1918 exclusion 10.0.0.0/8 for external host egress")
	}
	if !strings.Contains(content, "172.16.0.0/12") {
		t.Error("expected RFC1918 exclusion 172.16.0.0/12 for external host egress")
	}
	// Must have host annotations
	if !strings.Contains(content, "tentacular.dev/intended-hosts") {
		t.Error("expected intended-hosts annotation for fixed-host deps")
	}
	if !strings.Contains(content, "port: 443") {
		t.Error("expected port 443 in NetworkPolicy")
	}
	if !strings.Contains(content, "port: 53") {
		t.Error("expected DNS port 53 in NetworkPolicy")
	}
}

// TestE2E_NoContractFullPipeline tests that workflows without a contract use ENTRYPOINT defaults.
func TestE2E_NoContractFullPipeline(t *testing.T) {
	yamlContent := `
name: word-counter
version: "1.0"

triggers:
  - type: manual

nodes:
  count:
    path: ./nodes/count.ts

edges: []
`

	wf, warnings := spec.Parse([]byte(yamlContent))
	if wf == nil {
		t.Fatalf("spec.Parse failed, warnings: %v", warnings)
	}

	// No contract
	if wf.Contract != nil {
		t.Fatal("expected nil contract for word-counter")
	}

	// DeriveDenoFlags should return nil
	denoFlags := spec.DeriveDenoFlags(wf.Contract, "")
	if denoFlags != nil {
		t.Errorf("expected nil DeriveDenoFlags for nil contract, got %v", denoFlags)
	}

	// NetworkPolicy should not be generated
	manifest := GenerateNetworkPolicy(wf, "tentacular-dev", "")
	if manifest != nil {
		t.Error("expected nil NetworkPolicy for workflow without contract")
	}
}

// TestE2E_WebhookTriggerIngressVariant tests webhook trigger generates open ingress (podSelector: {}).
func TestE2E_WebhookTriggerIngressVariant(t *testing.T) {
	yamlContent := `
name: webhook-handler
version: "1.0"

triggers:
  - type: webhook
    path: /hook
  - type: cron
    schedule: "0 * * * *"

nodes:
  handle:
    path: ./nodes/handle.ts
  process:
    path: ./nodes/process.ts

edges:
  - from: handle
    to: process

contract:
  version: "1"
  dependencies:
    slack:
      protocol: https
      host: hooks.slack.com
      port: 443
      auth:
        type: webhook-url
        secret: slack.webhook_url
`

	wf, warnings := spec.Parse([]byte(yamlContent))
	if wf == nil {
		t.Fatalf("spec.Parse failed, warnings: %v", warnings)
	}

	// Webhook trigger should produce open ingress
	ingressRules := spec.DeriveIngressRules(wf)
	if len(ingressRules) != 1 {
		t.Fatalf("expected 1 ingress rule, got %d", len(ingressRules))
	}
	if ingressRules[0].Port != 8080 {
		t.Errorf("expected webhook ingress port 8080, got %d", ingressRules[0].Port)
	}
	if ingressRules[0].FromLabels != nil {
		t.Error("expected nil FromLabels (open podSelector: {}) for webhook trigger")
	}

	// Generate NetworkPolicy and verify ingress section
	manifest := GenerateNetworkPolicy(wf, "tentacular-dev", "")
	if manifest == nil {
		t.Fatal("expected non-nil NetworkPolicy manifest")
	}

	content := manifest.Content
	if !strings.Contains(content, "ingress:") {
		t.Error("expected ingress section in NetworkPolicy")
	}
	if !strings.Contains(content, "port: 8080") {
		t.Error("expected ingress port 8080")
	}
}
