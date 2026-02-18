package builder

import (
	"strings"
	"testing"
)

func TestGenerateDockerfile_NoWorkflowCopy(t *testing.T) {
	df := GenerateDockerfile()
	if strings.Contains(df, "COPY workflow.yaml") {
		t.Error("expected NO COPY workflow.yaml instruction (engine-only image)")
	}
	if strings.Contains(df, "COPY nodes/") {
		t.Error("expected NO COPY nodes/ instruction (engine-only image)")
	}
}

func TestGenerateDockerfile_EngineOnly(t *testing.T) {
	df := GenerateDockerfile()
	if !strings.Contains(df, "COPY .engine/") {
		t.Error("expected COPY .engine/ instruction")
	}
	if !strings.Contains(df, "COPY .engine/deno.json") {
		t.Error("expected COPY .engine/deno.json instruction")
	}
	// Assert no COPY instructions for workflow/nodes
	if strings.Contains(df, "COPY workflow.yaml") {
		t.Error("expected NO COPY workflow.yaml instruction in engine-only image")
	}
	if strings.Contains(df, "COPY nodes/") {
		t.Error("expected NO COPY nodes/ instruction in engine-only image")
	}
}

func TestGenerateDockerfile_Entrypoint(t *testing.T) {
	df := GenerateDockerfile()
	// ENTRYPOINT includes --workflow and --port (matches ConfigMap mount path)
	requiredFlags := []string{
		"--workflow",
		"/app/workflow/workflow.yaml",
		"--port",
		"8080",
		"--allow-net",
		"--allow-env",
		"--allow-read=/app,/var/run/secrets",
		"--allow-write=/tmp",
		"engine/main.ts",
	}
	for _, flag := range requiredFlags {
		if !strings.Contains(df, flag) {
			t.Errorf("expected ENTRYPOINT to contain %s", flag)
		}
	}
}

func TestGenerateDockerfileNoLockOnCache(t *testing.T) {
	df := GenerateDockerfile()
	// The deno cache line must include --lock=deno.lock for integrity verification
	// Find the line that has "deno", "cache" and verify it has "--lock=deno.lock" and NOT "--no-lock"
	lines := strings.Split(df, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, `"deno", "cache"`) {
			found = true
			if !strings.Contains(line, `"--lock=deno.lock"`) {
				t.Error("expected --lock=deno.lock flag in deno cache RUN instruction")
			}
			if strings.Contains(line, `"--no-lock"`) {
				t.Error("expected NO --no-lock flag in deno cache RUN instruction (should use --lock=deno.lock)")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected to find a deno cache instruction in Dockerfile")
	}
}

func TestGenerateDockerfile_NoDenoDirOverride(t *testing.T) {
	df := GenerateDockerfile()
	if strings.Contains(df, "ENV DENO_DIR") {
		t.Error("expected no ENV DENO_DIR override — engine deps use distroless default /deno-dir/")
	}
}

func TestGenerateDockerfile_CopyDenoLock(t *testing.T) {
	df := GenerateDockerfile()
	// Should COPY deno.lock file for integrity verification
	if !strings.Contains(df, "COPY .engine/deno.lock /app/deno.lock") {
		t.Error("expected COPY .engine/deno.lock /app/deno.lock instruction")
	}
}

func TestGenerateDockerfile_RuntimeEntrypointNoLock(t *testing.T) {
	df := GenerateDockerfile()
	// Runtime ENTRYPOINT should still have --no-lock (scoped flags override via command/args in k8s.go)
	lines := strings.Split(df, "\n")
	foundEntrypoint := false
	for _, line := range lines {
		if strings.Contains(line, "ENTRYPOINT") {
			foundEntrypoint = true
			if !strings.Contains(line, "--no-lock") {
				t.Error("expected --no-lock in runtime ENTRYPOINT")
			}
			break
		}
	}
	if !foundEntrypoint {
		t.Fatal("expected to find ENTRYPOINT in Dockerfile")
	}
}
