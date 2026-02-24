package cli

import (
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestIsBootstrapHost verifies bootstrap host detection.
func TestIsBootstrapHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"jsr.io", true},
		{"deno.land", true},
		{"cdn.deno.land", true},
		{"registry.npmjs.org", true},
		// case-insensitive
		{"JSR.IO", true},
		{"Deno.Land", true},
		// whitespace-tolerant
		{"  jsr.io  ", true},
		// non-bootstrap
		{"api.github.com", false},
		{"api.openai.com", false},
		{"hooks.slack.com", false},
		{"postgres.internal.svc.cluster.local", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			got := isBootstrapHost(tc.host)
			if got != tc.want {
				t.Errorf("isBootstrapHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

// TestLiveEgressHosts verifies parsing of the intended-hosts annotation.
func TestLiveEgressHosts(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
	}{
		{
			name:        "nil netpol returns nil",
			annotations: nil,
			want:        nil,
		},
		{
			name:        "missing annotation returns nil",
			annotations: map[string]string{},
			want:        nil,
		},
		{
			name:        "empty annotation returns nil",
			annotations: map[string]string{"tentacular.dev/intended-hosts": ""},
			want:        nil,
		},
		{
			name:        "single host",
			annotations: map[string]string{"tentacular.dev/intended-hosts": "api.github.com"},
			want:        []string{"api.github.com"},
		},
		{
			name:        "multiple hosts comma-separated",
			annotations: map[string]string{"tentacular.dev/intended-hosts": "api.github.com,jsr.io,api.openai.com"},
			want:        []string{"api.github.com", "jsr.io", "api.openai.com"},
		},
		{
			name:        "hosts with whitespace are trimmed",
			annotations: map[string]string{"tentacular.dev/intended-hosts": " api.github.com , jsr.io "},
			want:        []string{"api.github.com", "jsr.io"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var netpol *unstructured.Unstructured
			if tc.annotations != nil {
				netpol = &unstructured.Unstructured{}
				netpol.SetAnnotations(tc.annotations)
			}

			got := liveEgressHosts(netpol)
			if len(got) != len(tc.want) {
				t.Fatalf("liveEgressHosts() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("liveEgressHosts()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestFilterBootstrapDeps verifies that bootstrap deps are removed from a workflow copy.
func TestFilterBootstrapDeps(t *testing.T) {
	wf := &spec.Workflow{
		Name: "test-wf",
		Contract: &spec.Contract{
			Version: "v1",
			Dependencies: map[string]spec.Dependency{
				"github": {Protocol: "https", Host: "api.github.com", Port: 443},
				"deno":   {Protocol: "https", Host: "jsr.io", Port: 443},
				"npm":    {Protocol: "https", Host: "registry.npmjs.org", Port: 443},
				"openai": {Protocol: "https", Host: "api.openai.com", Port: 443},
			},
		},
	}

	clean := filterBootstrapDeps(wf)

	// Original should be untouched
	if len(wf.Contract.Dependencies) != 4 {
		t.Errorf("original workflow deps modified: got %d, want 4", len(wf.Contract.Dependencies))
	}

	// Clean copy should only have non-bootstrap deps
	if len(clean.Contract.Dependencies) != 2 {
		t.Errorf("clean deps count = %d, want 2", len(clean.Contract.Dependencies))
	}
	if _, ok := clean.Contract.Dependencies["github"]; !ok {
		t.Error("expected github dep to be retained")
	}
	if _, ok := clean.Contract.Dependencies["openai"]; !ok {
		t.Error("expected openai dep to be retained")
	}
	if _, ok := clean.Contract.Dependencies["deno"]; ok {
		t.Error("expected jsr.io dep to be removed")
	}
	if _, ok := clean.Contract.Dependencies["npm"]; ok {
		t.Error("expected registry.npmjs.org dep to be removed")
	}
}

// TestContractEgressHosts verifies that DNS and cluster-internal hosts are excluded.
func TestContractEgressHosts(t *testing.T) {
	wf := &spec.Workflow{
		Name: "test-wf",
		Contract: &spec.Contract{
			Version: "v1",
			Dependencies: map[string]spec.Dependency{
				"github": {Protocol: "https", Host: "api.github.com", Port: 443},
			},
		},
	}

	hosts := contractEgressHosts(wf)

	// Should have api.github.com but not DNS entries
	found := false
	for _, h := range hosts {
		if h == "api.github.com:443" {
			found = true
		}
		if h == "kube-dns.kube-system.svc.cluster.local:53" {
			t.Error("DNS entry should be excluded from contractEgressHosts")
		}
	}
	if !found {
		t.Errorf("expected api.github.com:443 in hosts, got %v", hosts)
	}
}
