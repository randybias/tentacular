package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

func TestGenerateImportMapWithNamespace(t *testing.T) {
	t.Run("returns nil for workflow without contract", func(t *testing.T) {
		wf := &spec.Workflow{Name: "no-contract"}
		got := GenerateImportMapWithNamespace(wf, "default", "")
		if got != nil {
			t.Error("expected nil for contract-less workflow")
		}
	})

	t.Run("returns nil for workflow with no jsr/npm deps", func(t *testing.T) {
		wf := &spec.Workflow{
			Name: "https-only",
			Contract: &spec.Contract{
				Version: "1",
				Dependencies: map[string]spec.Dependency{
					"api": {Protocol: "https", Host: "api.github.com", Port: 443},
				},
			},
		}
		got := GenerateImportMapWithNamespace(wf, "default", "")
		if got != nil {
			t.Error("expected nil for workflow with no jsr/npm deps")
		}
	})

	t.Run("generates ConfigMap for jsr dep", func(t *testing.T) {
		wf := &spec.Workflow{
			Name: "my-wf",
			Contract: &spec.Contract{
				Version: "1",
				Dependencies: map[string]spec.Dependency{
					"postgres": {Protocol: "jsr", Host: "@db/postgres", Version: "^0.4"},
				},
			},
		}
		got := GenerateImportMapWithNamespace(wf, "prod", "http://esm-sh.tentacular-system.svc.cluster.local:8080")
		if got == nil {
			t.Fatal("expected non-nil manifest")
		}
		if got.Kind != "ConfigMap" {
			t.Errorf("Kind = %q, want ConfigMap", got.Kind)
		}
		if got.Name != "my-wf-import-map" {
			t.Errorf("Name = %q, want my-wf-import-map", got.Name)
		}
		if !strings.Contains(got.Content, "namespace: prod") {
			t.Error("expected namespace prod in manifest")
		}
		// Both the versioned key (for code that imports with @version) and the
		// unversioned key (fallback for bare imports) must be present.
		if !strings.Contains(got.Content, "jsr:@db/postgres@^0.4") {
			t.Error("expected versioned jsr specifier key (e.g. jsr:@db/postgres@^0.4) in import map")
		}
		if !strings.Contains(got.Content, "\"jsr:@db/postgres\"") {
			t.Error("expected unversioned jsr specifier key (fallback) in import map")
		}
		if !strings.Contains(got.Content, "/jsr/@db/postgres@^0.4") {
			t.Error("expected proxy path with version in import map")
		}
	})

	t.Run("generates ConfigMap for npm dep", func(t *testing.T) {
		wf := &spec.Workflow{
			Name: "my-wf",
			Contract: &spec.Contract{
				Version: "1",
				Dependencies: map[string]spec.Dependency{
					"zod": {Protocol: "npm", Host: "zod", Version: "^3"},
				},
			},
		}
		got := GenerateImportMapWithNamespace(wf, "default", "")
		if got == nil {
			t.Fatal("expected non-nil manifest")
		}
		// Versioned key for code using "npm:zod@^3"
		if !strings.Contains(got.Content, "npm:zod@^3") {
			t.Error("expected versioned npm specifier key (npm:zod@^3) in import map")
		}
		// Unversioned fallback key for code using "npm:zod"
		if !strings.Contains(got.Content, "\"npm:zod\"") {
			t.Error("expected unversioned npm specifier key (fallback) in import map")
		}
		if !strings.Contains(got.Content, "/zod@^3") {
			t.Error("expected proxy path with version in import map")
		}
	})

	t.Run("omits version when not specified", func(t *testing.T) {
		wf := &spec.Workflow{
			Name: "my-wf",
			Contract: &spec.Contract{
				Version: "1",
				Dependencies: map[string]spec.Dependency{
					"zod": {Protocol: "npm", Host: "zod"},
				},
			},
		}
		got := GenerateImportMapWithNamespace(wf, "default", "")
		if got == nil {
			t.Fatal("expected non-nil manifest")
		}
		if !strings.Contains(got.Content, "\"npm:zod\"") {
			t.Error("expected npm:zod specifier in import map")
		}
		// Should not have a trailing @ with no version
		if strings.Contains(got.Content, "zod@\"") {
			t.Error("unexpected @ suffix when version is empty")
		}
	})

	t.Run("uses default proxy URL when empty", func(t *testing.T) {
		wf := &spec.Workflow{
			Name: "my-wf",
			Contract: &spec.Contract{
				Version: "1",
				Dependencies: map[string]spec.Dependency{
					"pg": {Protocol: "jsr", Host: "@db/postgres"},
				},
			},
		}
		got := GenerateImportMapWithNamespace(wf, "default", "")
		if got == nil {
			t.Fatal("expected non-nil manifest")
		}
		if !strings.Contains(got.Content, DefaultModuleProxyURL) {
			t.Errorf("expected default proxy URL %s in manifest", DefaultModuleProxyURL)
		}
	})

	t.Run("mixed jsr/npm and https deps — only jsr/npm in import map", func(t *testing.T) {
		wf := &spec.Workflow{
			Name: "mixed-wf",
			Contract: &spec.Contract{
				Version: "1",
				Dependencies: map[string]spec.Dependency{
					"api":      {Protocol: "https", Host: "api.github.com", Port: 443},
					"postgres": {Protocol: "jsr", Host: "@db/postgres", Version: "^0.4"},
				},
			},
		}
		got := GenerateImportMapWithNamespace(wf, "default", "")
		if got == nil {
			t.Fatal("expected non-nil manifest")
		}
		if strings.Contains(got.Content, "api.github.com") {
			t.Error("https dep should not appear in import map")
		}
		if !strings.Contains(got.Content, "jsr:@db/postgres") {
			t.Error("jsr dep should appear in import map")
		}
	})
}

func TestGenerateImportMapContainsEngineEntries(t *testing.T) {
	// The generated deno.json must include engine import entries so that
	// engine deps (std/path, tentacular, etc.) still resolve when the ConfigMap
	// overrides /app/deno.json. This prevents --import-map from breaking the engine.
	wf := &spec.Workflow{
		Name: "my-wf",
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"pg": {Protocol: "jsr", Host: "@db/postgres", Version: "^0.4"},
			},
		},
	}
	got := GenerateImportMapWithNamespace(wf, "default", "")
	if got == nil {
		t.Fatal("expected non-nil manifest")
	}

	// Engine entries must be present
	for _, entry := range []string{"std/path", "std/yaml", "tentacular", "@nats-io/transport-deno"} {
		if !strings.Contains(got.Content, entry) {
			t.Errorf("expected engine entry %q in merged deno.json, missing from:\n%s", entry, got.Content)
		}
	}

	// Workflow entry must also be present
	if !strings.Contains(got.Content, "jsr:@db/postgres") {
		t.Error("expected workflow jsr entry in merged deno.json")
	}
}

func TestHasModuleProxyDeps(t *testing.T) {
	tests := []struct {
		name string
		wf   *spec.Workflow
		want bool
	}{
		{
			name: "nil contract",
			wf:   &spec.Workflow{Name: "x"},
			want: false,
		},
		{
			name: "https only",
			wf: &spec.Workflow{
				Contract: &spec.Contract{Dependencies: map[string]spec.Dependency{
					"api": {Protocol: "https", Host: "api.example.com"},
				}},
			},
			want: false,
		},
		{
			name: "has jsr dep",
			wf: &spec.Workflow{
				Contract: &spec.Contract{Dependencies: map[string]spec.Dependency{
					"pg": {Protocol: "jsr", Host: "@db/postgres"},
				}},
			},
			want: true,
		},
		{
			name: "has npm dep",
			wf: &spec.Workflow{
				Contract: &spec.Contract{Dependencies: map[string]spec.Dependency{
					"zod": {Protocol: "npm", Host: "zod"},
				}},
			},
			want: true,
		},
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

func TestGenerateModuleProxyManifests(t *testing.T) {
	t.Run("emptydir produces 3 manifests", func(t *testing.T) {
		manifests := GenerateModuleProxyManifests("", "tentacular-system", "emptydir", "")
		if len(manifests) != 3 {
			t.Errorf("got %d manifests, want 3 (Deployment+Service+NetworkPolicy)", len(manifests))
		}
		kinds := map[string]bool{}
		for _, m := range manifests {
			kinds[m.Kind] = true
		}
		for _, expected := range []string{"Deployment", "Service", "NetworkPolicy"} {
			if !kinds[expected] {
				t.Errorf("missing manifest kind: %s", expected)
			}
		}
	})

	t.Run("pvc produces 4 manifests", func(t *testing.T) {
		manifests := GenerateModuleProxyManifests("", "tentacular-system", "pvc", "10Gi")
		if len(manifests) != 4 {
			t.Errorf("got %d manifests, want 4 (Deployment+Service+NetworkPolicy+PVC)", len(manifests))
		}
		hasPVC := false
		for _, m := range manifests {
			if m.Kind == "PersistentVolumeClaim" {
				hasPVC = true
				if !strings.Contains(m.Content, "10Gi") {
					t.Error("expected 10Gi in PVC manifest")
				}
			}
		}
		if !hasPVC {
			t.Error("expected PersistentVolumeClaim manifest")
		}
	})

	t.Run("NetworkPolicy allows egress to jsr.io and npm on 443", func(t *testing.T) {
		manifests := GenerateModuleProxyManifests("", "tentacular-system", "", "")
		for _, m := range manifests {
			if m.Kind == "NetworkPolicy" {
				if !strings.Contains(m.Content, "port: 443") {
					t.Error("expected port 443 egress in NetworkPolicy")
				}
				return
			}
		}
		t.Error("no NetworkPolicy manifest found")
	})

	t.Run("uses default image when empty", func(t *testing.T) {
		manifests := GenerateModuleProxyManifests("", "tentacular-system", "", "")
		for _, m := range manifests {
			if m.Kind == "Deployment" {
				if !strings.Contains(m.Content, "esm-dev/esm.sh") {
					t.Error("expected default esm.sh image in Deployment")
				}
				return
			}
		}
		t.Error("no Deployment manifest found")
	})

	t.Run("Deployment uses /esmd mount (no leading dot) and no runAsNonRoot", func(t *testing.T) {
		manifests := GenerateModuleProxyManifests("", "tentacular-system", "", "")
		for _, m := range manifests {
			if m.Kind == "Deployment" {
				if !strings.Contains(m.Content, "mountPath: /esmd") {
					t.Error("expected mountPath: /esmd (no leading dot) in Deployment")
				}
				if strings.Contains(m.Content, "/.esmd") {
					t.Error("unexpected /.esmd (with leading dot) in Deployment")
				}
				if strings.Contains(m.Content, "runAsNonRoot") {
					t.Error("unexpected runAsNonRoot in Deployment — esm-sh needs to run as root")
				}
				return
			}
		}
		t.Error("no Deployment manifest found")
	})
}

func TestScanNodeImports(t *testing.T) {
	t.Run("returns nil for non-existent dir", func(t *testing.T) {
		deps, err := ScanNodeImports("/no/such/dir")
		if err != nil {
			t.Errorf("expected nil error for missing dir, got %v", err)
		}
		if deps != nil {
			t.Errorf("expected nil deps for missing dir, got %v", deps)
		}
	})

	t.Run("detects jsr and npm imports from TypeScript", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.Mkdir(nodesDir, 0755); err != nil {
			t.Fatal(err)
		}
		src := `import { Client } from "jsr:@db/postgres@0.19.5";
import { z } from "npm:zod@3.22.0";
import { join } from "std/path"; // not jsr/npm, should be ignored
const x = await import("jsr:@std/encoding@1.0.0/base64");
`
		if err := os.WriteFile(filepath.Join(nodesDir, "main.ts"), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}

		deps, err := ScanNodeImports(nodesDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deps) != 3 {
			t.Fatalf("expected 3 deps, got %d: %+v", len(deps), deps)
		}

		byHost := make(map[string]spec.Dependency)
		for _, d := range deps {
			byHost[d.Host] = d
		}

		pg := byHost["@db/postgres"]
		if pg.Protocol != "jsr" || pg.Version != "0.19.5" {
			t.Errorf("@db/postgres: got protocol=%s version=%s", pg.Protocol, pg.Version)
		}
		zod := byHost["zod"]
		if zod.Protocol != "npm" || zod.Version != "3.22.0" {
			t.Errorf("zod: got protocol=%s version=%s", zod.Protocol, zod.Version)
		}
		enc := byHost["@std/encoding"]
		if enc.Protocol != "jsr" || enc.Version != "1.0.0/base64" {
			t.Errorf("@std/encoding: got protocol=%s version=%s", enc.Protocol, enc.Version)
		}
	})

	t.Run("deduplicates repeated imports across files", func(t *testing.T) {
		dir := t.TempDir()
		nodesDir := filepath.Join(dir, "nodes")
		if err := os.Mkdir(nodesDir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"a.ts", "b.ts"} {
			if err := os.WriteFile(filepath.Join(nodesDir, name),
				[]byte(`import { x } from "jsr:@db/postgres@0.19.5";`), 0644); err != nil {
				t.Fatal(err)
			}
		}
		deps, err := ScanNodeImports(nodesDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(deps) != 1 {
			t.Errorf("expected 1 dep (deduped), got %d", len(deps))
		}
	})
}
