// Integration tests for tntc scaffold params show and validate.
//
// These commands read params.schema.yaml and workflow.yaml from the CWD.
// Tests use os.Chdir to a temp directory with test files.
//
// Covers 6 cases from design doc Section 12.7:
//   - Params show with schema (table with current values)
//   - Params show no schema ("no params.schema.yaml found")
//   - Params show value mismatch (shows value with type indicator)
//   - Params validate all good (non-example values)
//   - Params validate stale (warns about example values)
//   - Params validate no schema ("nothing to validate" / error)

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupParamsTentacle creates a temp directory with workflow.yaml and
// optionally params.schema.yaml, and changes into it for the duration of the test.
func setupParamsTentacle(t *testing.T, workflowYAML, schemaYAML string) {
	t.Helper()
	dir := t.TempDir()
	if workflowYAML != "" {
		if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"),
			[]byte(workflowYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if schemaYAML != "" {
		if err := os.WriteFile(filepath.Join(dir, "params.schema.yaml"),
			[]byte(schemaYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
}

const paramsTestWorkflowYAML = `name: my-tentacle
version: "1.0"
triggers:
  - type: manual
  - type: cron
    name: check-endpoints
    schedule: "*/10 * * * *"
config:
  timeout: 60s
  endpoints:
    - url: "https://mysite.com"
nodes:
  probe:
    path: ./nodes/probe.ts
`

const paramsTestWorkflowExampleYAML = `name: my-tentacle
version: "1.0"
triggers:
  - type: manual
  - type: cron
    name: check-endpoints
    schedule: "*/5 * * * *"
config:
  timeout: 120s
  endpoints:
    - url: "https://example.com"
nodes:
  probe:
    path: ./nodes/probe.ts
`

const paramsTestSchemaYAML = `version: "1"
parameters:
  probe_schedule:
    path: triggers[name=check-endpoints].schedule
    type: string
    description: "Cron schedule"
    required: false
    default: "*/5 * * * *"
  endpoints:
    path: config.endpoints
    type: list
    description: "Endpoints"
    required: true
`

// runParamsShowCmd runs tntc scaffold params show from the CWD and returns stdout+error.
func runParamsShowCmd(t *testing.T) (string, error) {
	t.Helper()
	cmd := newScaffoldParamsShowCmd()
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	return out, runErr
}

// runParamsValidateCmd runs tntc scaffold params validate from CWD.
func runParamsValidateCmd(t *testing.T) (string, error) {
	t.Helper()
	cmd := newScaffoldParamsValidateCmd()
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	return out, runErr
}

// TestScaffoldParamsShowWithSchema verifies that "params show" prints a table
// of parameter names and current values when params.schema.yaml exists.
func TestScaffoldParamsShowWithSchema(t *testing.T) {
	setupParamsTentacle(t, paramsTestWorkflowYAML, paramsTestSchemaYAML)

	out, err := runParamsShowCmd(t)
	if err != nil {
		t.Fatalf("params show: %v", err)
	}
	// Should show parameter names
	if !strings.Contains(out, "probe_schedule") && !strings.Contains(out, "endpoints") {
		t.Errorf("expected parameter names in output, got:\n%s", out)
	}
}

// TestScaffoldParamsShowNoSchema verifies that "params show" returns an error
// when no params.schema.yaml exists.
func TestScaffoldParamsShowNoSchema(t *testing.T) {
	setupParamsTentacle(t, paramsTestWorkflowYAML, "") // no schema

	_, err := runParamsShowCmd(t)
	if err == nil {
		t.Fatal("expected error for missing schema, got nil")
	}
	if !strings.Contains(err.Error(), "params.schema.yaml") {
		t.Errorf("expected error message about params.schema.yaml, got: %v", err)
	}
}

// TestScaffoldParamsShowValueResolution verifies that the current value from
// workflow.yaml is resolved and shown (not just the default or example).
func TestScaffoldParamsShowValueResolution(t *testing.T) {
	setupParamsTentacle(t, paramsTestWorkflowYAML, paramsTestSchemaYAML)

	out, err := runParamsShowCmd(t)
	if err != nil {
		t.Fatalf("params show: %v", err)
	}
	// The workflow has schedule */10 * * * * -- verify it's shown
	if !strings.Contains(out, "*/10 * * * *") {
		t.Errorf("expected resolved schedule value '*/10 * * * *' in output, got:\n%s", out)
	}
}

// TestScaffoldParamsValidateAllGood verifies that validate returns success when
// no example values remain.
func TestScaffoldParamsValidateAllGood(t *testing.T) {
	setupParamsTentacle(t, paramsTestWorkflowYAML, paramsTestSchemaYAML)

	out, err := runParamsValidateCmd(t)
	if err != nil {
		t.Fatalf("params validate (all good): %v\nOutput:\n%s", err, out)
	}
	if !strings.Contains(out, "non-example") {
		t.Errorf("expected 'non-example' success message, got:\n%s", out)
	}
}

// TestScaffoldParamsValidateStale verifies that validate warns when example
// values remain in workflow.yaml.
func TestScaffoldParamsValidateStale(t *testing.T) {
	setupParamsTentacle(t, paramsTestWorkflowExampleYAML, paramsTestSchemaYAML)

	out, err := runParamsValidateCmd(t)
	if err == nil {
		t.Fatalf("expected validation error for example values, got nil\nOutput:\n%s", out)
	}
	// Should mention the example value
	if !strings.Contains(out, "example.com") && !strings.Contains(err.Error(), "example") {
		t.Errorf("expected warning about example values, got:\n%s\nerr: %v", out, err)
	}
}

// TestScaffoldParamsValidateNoSchema verifies that validate returns an error
// when no params.schema.yaml exists.
func TestScaffoldParamsValidateNoSchema(t *testing.T) {
	setupParamsTentacle(t, paramsTestWorkflowYAML, "") // no schema

	_, err := runParamsValidateCmd(t)
	if err == nil {
		t.Fatal("expected error when no schema exists, got nil")
	}
}
