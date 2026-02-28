package builder

import (
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

// TestE2E_DockerfileLockfileIntegrity verifies the Dockerfile has lockfile integrity features.
func TestE2E_DockerfileLockfileIntegrity(t *testing.T) {
	df := GenerateDockerfile()

	// Must COPY deno.lock for integrity verification
	if !strings.Contains(df, "COPY .engine/deno.lock /app/deno.lock") {
		t.Error("expected COPY .engine/deno.lock /app/deno.lock instruction")
	}

	// Cache line must use --lock=deno.lock
	lines := strings.Split(df, "\n")
	foundCache := false
	for _, line := range lines {
		if strings.Contains(line, `"deno", "cache"`) {
			foundCache = true
			if !strings.Contains(line, `"--lock=deno.lock"`) {
				t.Error("expected --lock=deno.lock in deno cache RUN instruction")
			}
			break
		}
	}
	if !foundCache {
		t.Fatal("expected deno cache instruction in Dockerfile")
	}

	// Runtime ENTRYPOINT must use --no-lock (scoped flags override via command/args)
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
		t.Fatal("expected ENTRYPOINT in Dockerfile")
	}
}

// TestE2E_DeploymentSecurityHardeningComplete is a comprehensive check that all security fields
// are present in a generated Deployment manifest.
func TestE2E_DeploymentSecurityHardeningComplete(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "security-check",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 8 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch":   {Path: "./nodes/fetch.ts"},
			"process": {Path: "./nodes/process.ts"},
		},
		Edges: []spec.Edge{
			{From: "fetch", To: "process"},
		},
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"github": {
					Protocol: "https",
					Host:     "api.github.com",
					Port:     443,
					Auth:     &spec.DependencyAuth{Type: "bearer-token", Secret: "github.token"},
				},
			},
		},
	}

	manifests := GenerateK8sManifests(wf, "ghcr.io/randybias/tentacular-engine:latest", "tentacular-dev", DeployOptions{
		RuntimeClassName: "gvisor",
	})

	dep := manifests[0].Content

	// Pod-level security
	checks := map[string]string{
		"automountServiceAccountToken: false": "SA token mount disabled",
		"runAsNonRoot: true":                  "runAsNonRoot",
		"runAsUser: 65534":                    "nobody user (65534)",
		"type: RuntimeDefault":               "seccomp RuntimeDefault",
		"runtimeClassName: gvisor":            "gVisor runtime class",
		// Container-level security
		"readOnlyRootFilesystem: true":    "read-only root filesystem",
		"allowPrivilegeEscalation: false": "no privilege escalation",
		"- ALL":                           "drop ALL capabilities",
		// Volume security
		"readOnly: true":    "read-only volume mounts",
		"sizeLimit: 512Mi":  "emptyDir size limit",
		"optional: true":    "optional secret volume",
		"mountPath: /tmp":   "tmp mount",
		// Resources
		`memory: "64Mi"`:  "memory request",
		`memory: "256Mi"`: "memory limit",
		`cpu: "100m"`:     "cpu request",
		`cpu: "500m"`:     "cpu limit",
		// Probes
		"livenessProbe:":  "liveness probe",
		"readinessProbe:": "readiness probe",
		"path: /health":   "health endpoint",
		// Strategy
		"type: Recreate": "Recreate strategy (no rolling updates)",
	}

	for expected, description := range checks {
		if !strings.Contains(dep, expected) {
			t.Errorf("missing security field: %s (%s)", expected, description)
		}
	}
}

// TestE2E_CronTriggerAnnotationOnDeployment verifies that cron triggers produce
// a tentacular.dev/cron-schedule annotation on the Deployment instead of CronJob manifests.
// Pre-warming is now handled server-side by the MCP server after wf_apply.
func TestE2E_CronTriggerAnnotationOnDeployment(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "cron-sec",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 8 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"process": {Path: "./nodes/process.ts"},
		},
	}

	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})

	// No CronJob manifests should be generated.
	for _, m := range manifests {
		if m.Kind == "CronJob" {
			t.Errorf("unexpected CronJob manifest: cron triggers are now annotations on the Deployment")
		}
	}

	// The cron schedule should appear as an annotation on the Deployment.
	dep := manifests[0].Content
	if !strings.Contains(dep, "tentacular.dev/cron-schedule: 0 8 * * *") {
		t.Error("expected cron-schedule annotation on Deployment with schedule")
	}
}

// TestE2E_NoCronNoAnnotation verifies that workflows without cron triggers
// do NOT have a cron-schedule annotation on the Deployment.
func TestE2E_NoCronNoAnnotation(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "manual-only",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "manual"},
		},
		Nodes: map[string]spec.NodeSpec{
			"process": {Path: "./nodes/process.ts"},
		},
	}

	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})

	dep := manifests[0].Content
	if strings.Contains(dep, "tentacular.dev/cron-schedule") {
		t.Error("expected no cron-schedule annotation for manual-only workflow")
	}

	for _, m := range manifests {
		if m.Kind == "CronJob" {
			t.Errorf("unexpected CronJob manifest for manual-only workflow: %s", m.Name)
		}
	}
}

// TestE2E_DenoFlagsCommandStructure verifies the exact order and structure of command/args
// entries in the generated Deployment YAML.
func TestE2E_DenoFlagsCommandStructure(t *testing.T) {
	t.Run("scoped flags for fixed-host deps", func(t *testing.T) {
		wf := &spec.Workflow{
			Name:    "scoped-test",
			Version: "1.0",
			Triggers: []spec.Trigger{
				{Type: "manual"},
			},
			Nodes: map[string]spec.NodeSpec{
				"fetch": {Path: "./nodes/fetch.ts"},
			},
			Contract: &spec.Contract{
				Dependencies: map[string]spec.Dependency{
					"github": {
						Protocol: "https",
						Host:     "api.github.com",
						Port:     443,
					},
					"slack": {
						Protocol: "https",
						Host:     "hooks.slack.com",
						Port:     443,
					},
				},
			},
		}

		manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
		dep := manifests[0].Content

		// Verify command: ["deno"]
		if !strings.Contains(dep, "command:\n            - deno") {
			t.Error("expected command: with deno entry")
		}

		// Verify args ordering
		expectedArgs := []string{
			"- run",
			"- --no-lock",
			"- --unstable-net",
			"- --allow-net=0.0.0.0:8080,api.github.com:443,hooks.slack.com:443",
			"- --allow-read=/app",
			"- --allow-write=/tmp",
			"- --allow-env=DENO_DIR,HOME",
			"- engine/main.ts",
			"- --workflow",
			"- /app/workflow/workflow.yaml",
			"- --port",
			`- "8080"`,
		}

		for _, arg := range expectedArgs {
			if !strings.Contains(dep, arg) {
				t.Errorf("expected arg %q in Deployment args", arg)
			}
		}
	})

	t.Run("broad flags for dynamic-target deps", func(t *testing.T) {
		wf := &spec.Workflow{
			Name:    "broad-test",
			Version: "1.0",
			Triggers: []spec.Trigger{
				{Type: "manual"},
			},
			Nodes: map[string]spec.NodeSpec{
				"fetch": {Path: "./nodes/fetch.ts"},
			},
			Contract: &spec.Contract{
				Dependencies: map[string]spec.Dependency{
					"openai": {
						Protocol: "https",
						Host:     "api.openai.com",
						Port:     443,
					},
					"news": {
						Protocol: "https",
						Type:     "dynamic-target",
						CIDR:     "0.0.0.0/0",
						DynPorts: []string{"443/TCP"},
					},
				},
			},
		}

		manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
		dep := manifests[0].Content

		// Verify broad --allow-net (no =)
		if !strings.Contains(dep, "- --allow-net\n") {
			t.Error("expected broad --allow-net flag (no = sign) for dynamic-target")
		}

		// Should NOT have scoped form
		lines := strings.Split(dep, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- --allow-net=") {
				t.Errorf("expected NO scoped --allow-net= for dynamic-target, found: %s", trimmed)
			}
		}
	})

	t.Run("no command/args without contract", func(t *testing.T) {
		wf := &spec.Workflow{
			Name:    "no-contract",
			Version: "1.0",
			Triggers: []spec.Trigger{
				{Type: "manual"},
			},
			Nodes: map[string]spec.NodeSpec{
				"process": {Path: "./nodes/process.ts"},
			},
		}

		manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
		dep := manifests[0].Content

		if strings.Contains(dep, "command:") {
			t.Error("expected NO command field when contract is nil (ENTRYPOINT fallback)")
		}
		if strings.Contains(dep, "args:") {
			t.Error("expected NO args field when contract is nil (ENTRYPOINT fallback)")
		}
	})
}
