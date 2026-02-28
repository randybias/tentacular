package spec

import (
	"strings"
	"testing"
)

// TestJSRNPMProtocolValidation verifies that jsr/npm protocol deps are validated correctly.
func TestJSRNPMProtocolValidation(t *testing.T) {
	validYAML := `name: test-wf
version: "1.0"
triggers:
  - type: manual
    name: trigger
nodes:
  step1:
    path: nodes/step1.ts
edges: []
contract:
  version: "1"
  dependencies:
    postgres:
      protocol: jsr
      host: "@db/postgres"
      version: "^0.4"
    zod:
      protocol: npm
      host: "zod"
      version: "^3"
`
	wf, errs := Parse([]byte(validYAML))
	if len(errs) > 0 {
		t.Fatalf("expected valid spec, got errors: %v", errs)
	}
	if wf.Contract == nil {
		t.Fatal("expected contract to be parsed")
	}
	pgDep := wf.Contract.Dependencies["postgres"]
	if pgDep.Protocol != "jsr" {
		t.Errorf("postgres dep protocol = %q, want jsr", pgDep.Protocol)
	}
	if pgDep.Host != "@db/postgres" {
		t.Errorf("postgres dep host = %q, want @db/postgres", pgDep.Host)
	}
	if pgDep.Version != "^0.4" {
		t.Errorf("postgres dep version = %q, want ^0.4", pgDep.Version)
	}

	zod := wf.Contract.Dependencies["zod"]
	if zod.Protocol != "npm" {
		t.Errorf("zod dep protocol = %q, want npm", zod.Protocol)
	}
}

// TestJSRMissingHostValidation verifies that jsr/npm deps without a host are rejected.
func TestJSRMissingHostValidation(t *testing.T) {
	yaml := `name: test-wf
version: "1.0"
triggers:
  - type: manual
    name: trigger
nodes:
  step1:
    path: nodes/step1.ts
edges: []
contract:
  version: "1"
  dependencies:
    nohost:
      protocol: jsr
`
	_, errs := Parse([]byte(yaml))
	found := false
	for _, e := range errs {
		if strings.Contains(e, "jsr requires host") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'jsr requires host' error, got: %v", errs)
	}
}

// TestHasModuleProxyDepsSpec verifies HasModuleProxyDeps in the spec package.
func TestHasModuleProxyDepsSpec(t *testing.T) {
	tests := []struct {
		name string
		wf   *Workflow
		want bool
	}{
		{"nil workflow", nil, false},
		{"no contract", &Workflow{Name: "x"}, false},
		{"https only", &Workflow{Contract: &Contract{Dependencies: map[string]Dependency{
			"api": {Protocol: "https"},
		}}}, false},
		{"jsr dep", &Workflow{Contract: &Contract{Dependencies: map[string]Dependency{
			"pg": {Protocol: "jsr", Host: "@db/postgres"},
		}}}, true},
		{"npm dep", &Workflow{Contract: &Contract{Dependencies: map[string]Dependency{
			"zod": {Protocol: "npm", Host: "zod"},
		}}}, true},
		{"mixed — has jsr", &Workflow{Contract: &Contract{Dependencies: map[string]Dependency{
			"api": {Protocol: "https"},
			"pg":  {Protocol: "jsr", Host: "@db/postgres"},
		}}}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HasModuleProxyDeps(tc.wf)
			if got != tc.want {
				t.Errorf("HasModuleProxyDeps() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDeriveEgressRulesSkipsJSRNPM verifies jsr/npm deps produce no NetworkPolicy egress rules.
func TestDeriveEgressRulesSkipsJSRNPM(t *testing.T) {
	c := &Contract{
		Version: "1",
		Dependencies: map[string]Dependency{
			"api":      {Protocol: "https", Host: "api.github.com", Port: 443},
			"postgres": {Protocol: "jsr", Host: "@db/postgres", Version: "^0.4"},
			"zod":      {Protocol: "npm", Host: "zod", Version: "^3"},
		},
	}

	rules := DeriveEgressRules(c)

	for _, r := range rules {
		if r.Host == "@db/postgres" || r.Host == "jsr.io" {
			t.Errorf("unexpected jsr egress rule: %+v", r)
		}
		if r.Host == "zod" || r.Host == "registry.npmjs.org" {
			t.Errorf("unexpected npm egress rule: %+v", r)
		}
	}

	// https dep should still be present
	found := false
	for _, r := range rules {
		if r.Host == "api.github.com" && r.Port == 443 {
			found = true
		}
	}
	if !found {
		t.Error("expected https dep egress rule for api.github.com:443")
	}
}

// TestDeriveDenoFlagsModuleProxy verifies proxy host, --allow-import, and no --import-map for jsr/npm deps.
func TestDeriveDenoFlagsModuleProxy(t *testing.T) {
	const proxyHost = "esm-sh.tentacular-support.svc.cluster.local:8080"
	c := &Contract{
		Version: "1",
		Dependencies: map[string]Dependency{
			"api":      {Protocol: "https", Host: "api.github.com", Port: 443},
			"postgres": {Protocol: "jsr", Host: "@db/postgres", Version: "^0.4"},
		},
	}

	flags := DeriveDenoFlags(c, proxyHost)
	if flags == nil {
		t.Fatal("expected non-nil flags")
	}

	flagStr := strings.Join(flags, " ")

	// Should NOT include --import-map flag — the merged deno.json is auto-discovered
	// at /app/engine/deno.json via ConfigMap mount, no flag needed.
	if strings.Contains(flagStr, "--import-map") {
		t.Errorf("expected no --import-map flag (auto-discovered deno.json), got: %s", flagStr)
	}

	// Should include proxy host in --allow-net
	if !strings.Contains(flagStr, proxyHost) {
		t.Errorf("expected module proxy host %s in --allow-net, got: %s", proxyHost, flagStr)
	}

	// --allow-import needs both deno.land:443 (transitive imports within deno_std
	// use absolute URLs that bypass the import map) and the proxy host.
	wantImport := "--allow-import=deno.land:443," + proxyHost
	if !strings.Contains(flagStr, wantImport) {
		t.Errorf("expected %s in flags, got: %s", wantImport, flagStr)
	}

	// Should still include the https dep host
	if !strings.Contains(flagStr, "api.github.com:443") {
		t.Errorf("expected api.github.com:443 in --allow-net, got: %s", flagStr)
	}

	// jsr host should NOT be in --allow-net directly
	if strings.Contains(flagStr, "@db/postgres") {
		t.Errorf("jsr host should not appear in flags, got: %s", flagStr)
	}
}

// TestDeriveDenoFlagsAlwaysIncludesProxy verifies proxy host and --allow-import
// are always present because the engine itself has jsr: deps that route through esm.sh.
func TestDeriveDenoFlagsAlwaysIncludesProxy(t *testing.T) {
	c := &Contract{
		Version: "1",
		Dependencies: map[string]Dependency{
			"api": {Protocol: "https", Host: "api.github.com", Port: 443},
		},
	}

	flags := DeriveDenoFlags(c, "some-proxy:8080")
	flagStr := strings.Join(flags, " ")

	if !strings.Contains(flagStr, "some-proxy:8080") {
		t.Errorf("expected proxy host always present (engine jsr deps), got: %s", flagStr)
	}
	if !strings.Contains(flagStr, "--allow-import") {
		t.Errorf("expected --allow-import always present (engine jsr deps), got: %s", flagStr)
	}
	if strings.Contains(flagStr, "--import-map") {
		t.Errorf("expected no --import-map flag (Deno auto-discovers deno.json), got: %s", flagStr)
	}
}
