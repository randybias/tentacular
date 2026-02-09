package k8s

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCheckResultsJSONWithWarning(t *testing.T) {
	results := []CheckResult{
		{Name: "gVisor RuntimeClass", Passed: true, Warning: "gVisor not found"},
	}
	output := CheckResultsJSON(results)

	if !strings.Contains(output, `"warning"`) {
		t.Error("expected warning field in JSON output")
	}
	if !strings.Contains(output, "gVisor not found") {
		t.Error("expected warning message in JSON output")
	}
}

func TestCheckResultsJSONWithoutWarning(t *testing.T) {
	results := []CheckResult{
		{Name: "K8s API reachable", Passed: true},
	}
	output := CheckResultsJSON(results)

	if strings.Contains(output, `"warning"`) {
		t.Error("expected warning field to be omitted (omitempty) when empty")
	}
}

func TestCheckResultsJSONAllFields(t *testing.T) {
	results := []CheckResult{
		{
			Name:        "test-check",
			Passed:      false,
			Warning:     "something is off",
			Remediation: "fix it",
		},
	}
	output := CheckResultsJSON(results)

	// Round-trip: parse JSON back and verify fields
	var parsed []CheckResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 result, got %d", len(parsed))
	}
	r := parsed[0]
	if r.Name != "test-check" {
		t.Errorf("expected name test-check, got %s", r.Name)
	}
	if r.Passed != false {
		t.Error("expected passed to be false")
	}
	if r.Warning != "something is off" {
		t.Errorf("expected warning 'something is off', got %s", r.Warning)
	}
	if r.Remediation != "fix it" {
		t.Errorf("expected remediation 'fix it', got %s", r.Remediation)
	}
}
