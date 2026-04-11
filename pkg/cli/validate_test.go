// Unit tests for validate.go functions.
//
// validate_integration_test.go covers end-to-end tests that run the tntc binary.
// This file adds unit-level coverage for:
//   - outputValidateJSON: JSON output with and without contract
//   - runValidate: text/json output modes, missing/invalid workflow errors
//   - ValidateResult / EgressRuleJSON / IngressRuleJSON: JSON round-trip

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

// minimalWorkflowYAML is a valid workflow spec for unit tests.
// It has a single manual trigger and one node — the minimum required
// by spec.Parse to produce a valid Workflow without a contract section.
const minimalWorkflowYAML = `name: test-workflow
version: "1.0"
triggers:
  - type: manual
nodes:
  handler:
    path: ./handler.ts
    description: "Test node"
`

// contractWorkflowYAML includes a contract section with two dependencies.
// Each dependency uses a different protocol (postgresql, https) and has an
// auth secret with the required auth.type field. The postgresql dependency
// also includes the required database and user fields. This exercises the
// full egress/secrets extraction paths in outputValidateJSON.
const contractWorkflowYAML = `name: contract-workflow
version: "2.0"
triggers:
  - type: cron
    schedule: "*/5 * * * *"
  - type: webhook
    path: /ingest
    port: 8080
nodes:
  fetch:
    path: ./fetch.ts
    description: "Test node"
  store:
    path: ./store.ts
    description: "Test node"
edges:
  - from: fetch
    to: store
contract:
  version: "1"
  dependencies:
    postgres:
      protocol: postgresql
      host: db.example.com
      port: 5432
      database: appdb
      user: postgres
      auth:
        type: password
        secret: postgres.password
    github:
      protocol: https
      host: api.github.com
      port: 443
      auth:
        type: bearer-token
        secret: github.token
`

// --- outputValidateJSON ---
//
// These tests call outputValidateJSON directly (bypassing cobra) to verify
// the JSON structure produced for workflows with and without a contract.

// TestOutputValidateJSONWithoutContract verifies that a workflow without a
// contract section produces HasContract=false and empty secrets/egress arrays.
func TestOutputValidateJSONWithoutContract(t *testing.T) {
	data := []byte(minimalWorkflowYAML)
	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		t.Fatalf("spec.Parse: %v", errs)
	}

	var buf bytes.Buffer
	if err := outputValidateJSON(wf, &buf); err != nil {
		t.Fatalf("outputValidateJSON: %v", err)
	}

	var result ValidateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.Workflow != "test-workflow" {
		t.Errorf("Workflow: got %q, want %q", result.Workflow, "test-workflow")
	}
	if result.Version != "1.0" {
		t.Errorf("Version: got %q, want %q", result.Version, "1.0")
	}
	if result.HasContract {
		t.Error("expected HasContract=false for workflow without contract")
	}
	if len(result.Secrets) != 0 {
		t.Errorf("expected no secrets, got %v", result.Secrets)
	}
	if len(result.EgressRules) != 0 {
		t.Errorf("expected no egress rules, got %v", result.EgressRules)
	}
}

// TestOutputValidateJSONWithContract verifies that a workflow with a contract
// section correctly populates HasContract, Secrets, and EgressRules from the
// contract dependencies.
func TestOutputValidateJSONWithContract(t *testing.T) {
	data := []byte(contractWorkflowYAML)
	wf, errs := spec.Parse(data)
	if len(errs) > 0 {
		t.Fatalf("spec.Parse: %v", errs)
	}

	var buf bytes.Buffer
	if err := outputValidateJSON(wf, &buf); err != nil {
		t.Fatalf("outputValidateJSON: %v", err)
	}

	var result ValidateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.Workflow != "contract-workflow" {
		t.Errorf("Workflow: got %q, want %q", result.Workflow, "contract-workflow")
	}
	if !result.HasContract {
		t.Error("expected HasContract=true")
	}
	if len(result.Secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d: %v", len(result.Secrets), result.Secrets)
	}
	// DeriveEgressRules produces 4 rules: 2 dependency rules + 2 DNS rules
	if len(result.EgressRules) != 4 {
		t.Errorf("expected 4 egress rules (2 deps + 2 DNS), got %d", len(result.EgressRules))
	}
}

// --- runValidate ---
//
// These tests exercise the full cobra command path (NewValidateCmd → Execute)
// with workflow.yaml files in temporary directories.

// TestRunValidateMissingWorkflowYAML verifies that the validate command fails
// gracefully when workflow.yaml does not exist in the target directory.
func TestRunValidateMissingWorkflowYAML(t *testing.T) {
	dir := t.TempDir()
	cmd := NewValidateCmd()
	cmd.SetArgs([]string{dir})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing workflow.yaml")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("expected 'reading' in error, got: %v", err)
	}
}

// TestRunValidateInvalidSpec verifies that malformed YAML in workflow.yaml
// causes a parse/validation error rather than a panic.
func TestRunValidateInvalidSpec(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(":::invalid"), 0o644)

	cmd := NewValidateCmd()
	cmd.SetArgs([]string{dir})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid workflow spec")
	}
}

// TestRunValidateTextOutput verifies the default text output mode contains
// "is valid" for a correct workflow spec.
func TestRunValidateTextOutput(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(minimalWorkflowYAML), 0o644)

	cmd := NewValidateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{dir})
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runValidate: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "is valid") {
		t.Errorf("expected 'is valid' in text output, got: %s", output)
	}
}

// TestRunValidateJSONOutput verifies that -o json produces parseable JSON
// with the workflow name populated.
func TestRunValidateJSONOutput(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(minimalWorkflowYAML), 0o644)

	cmd := NewValidateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{dir, "-o", "json"})
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("runValidate: %v", err)
	}

	var result ValidateResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, buf.String())
	}
	if result.Workflow != "test-workflow" {
		t.Errorf("expected workflow name in JSON, got %q", result.Workflow)
	}
}

// --- JSON round-trip ---

// TestValidateResultJSONRoundTrip verifies that ValidateResult with all fields
// populated (including EgressRuleJSON and IngressRuleJSON) survives a marshal →
// unmarshal cycle without data loss. This catches struct tag mismatches and
// serialization issues.
func TestValidateResultJSONRoundTrip(t *testing.T) {
	result := ValidateResult{
		Workflow:    "my-wf",
		Version:     "1.0",
		Nodes:       3,
		Edges:       2,
		Triggers:    1,
		HasContract: true,
		Secrets:     []string{"db.password"},
		EgressRules: []EgressRuleJSON{
			{Host: "api.example.com", Port: 443, Protocol: "https"},
		},
		IngressRules: []IngressRuleJSON{
			{Port: 8080, Protocol: "tcp", FromLabels: map[string]string{"app": "frontend"}},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ValidateResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Workflow != "my-wf" {
		t.Errorf("Workflow: got %q, want %q", decoded.Workflow, "my-wf")
	}
	if len(decoded.EgressRules) != 1 {
		t.Errorf("EgressRules: got %d, want 1", len(decoded.EgressRules))
	}
	if len(decoded.IngressRules) != 1 {
		t.Errorf("IngressRules: got %d, want 1", len(decoded.IngressRules))
	}
	if decoded.IngressRules[0].FromLabels["app"] != "frontend" {
		t.Errorf("FromLabels: got %v", decoded.IngressRules[0].FromLabels)
	}
}
