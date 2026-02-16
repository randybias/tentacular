package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// --- WI-4: Structured JSON Output Tests ---
// These tests validate the CommandResult envelope and EmitResult behavior.
// Build tag: wi4 -- run with: go test -tags wi4 ./pkg/cli/...

func TestCommandResultEnvelopeFields(t *testing.T) {
	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "5/5 tests passed",
		Hints:   []string{},
		Timing: TimingInfo{
			StartedAt:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: 1234,
		},
	}

	if result.Version != "1" {
		t.Errorf("expected version 1, got %s", result.Version)
	}
	if result.Command != "test" {
		t.Errorf("expected command test, got %s", result.Command)
	}
	if result.Status != "pass" {
		t.Errorf("expected status pass, got %s", result.Status)
	}
}

func TestCommandResultJSONSerialization(t *testing.T) {
	result := CommandResult{
		Version: "1",
		Command: "deploy",
		Status:  "fail",
		Summary: "preflight checks failed",
		Hints:   []string{"check RBAC permissions", "verify namespace exists"},
		Timing: TimingInfo{
			StartedAt:  "2026-01-15T12:00:00Z",
			DurationMs: 567,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal CommandResult: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	for _, field := range []string{`"version":"1"`, `"command":"deploy"`, `"status":"fail"`, `"hints":`} {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("expected JSON to contain %s, got: %s", field, jsonStr)
		}
	}

	// Round-trip
	var decoded CommandResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal CommandResult: %v", err)
	}
	if decoded.Command != "deploy" {
		t.Errorf("expected deploy after round-trip, got %s", decoded.Command)
	}
	if len(decoded.Hints) != 2 {
		t.Errorf("expected 2 hints after round-trip, got %d", len(decoded.Hints))
	}
	if decoded.Timing.DurationMs != 567 {
		t.Errorf("expected duration 567ms, got %d", decoded.Timing.DurationMs)
	}
}

func TestCommandResultEmptyHints(t *testing.T) {
	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "all passed",
		Hints:   []string{},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Empty hints should serialize as [] not null
	if !strings.Contains(string(data), `"hints":[]`) {
		t.Errorf("expected empty hints array in JSON, got: %s", string(data))
	}
}

func TestEmitResultJSONMode(t *testing.T) {
	// Create a command with -o json flag
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format")
	cmd.ParseFlags([]string{"-o", "json"})

	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "5/5 tests passed",
		Hints:   []string{},
		Timing: TimingInfo{
			StartedAt:  "2026-01-15T12:00:00Z",
			DurationMs: 100,
		},
	}

	var buf bytes.Buffer
	err := EmitResult(cmd, result, &buf)
	if err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	output := buf.String()

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("EmitResult JSON output is not valid JSON: %v\nOutput: %s", err, output)
	}

	if parsed["version"] != "1" {
		t.Errorf("expected version 1 in JSON output, got %v", parsed["version"])
	}
	if parsed["command"] != "test" {
		t.Errorf("expected command test in JSON output, got %v", parsed["command"])
	}
}

func TestEmitResultTextMode(t *testing.T) {
	// Create a command with -o text flag (default)
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format")
	cmd.ParseFlags([]string{"-o", "text"})

	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "5/5 tests passed",
		Hints:   []string{"run with --live for integration tests"},
		Timing: TimingInfo{
			StartedAt:  "2026-01-15T12:00:00Z",
			DurationMs: 100,
		},
	}

	var buf bytes.Buffer
	err := EmitResult(cmd, result, &buf)
	if err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	output := buf.String()

	// Text mode should contain the summary
	if !strings.Contains(output, "5/5 tests passed") {
		t.Errorf("expected summary in text output, got: %s", output)
	}

	// Text mode should NOT be valid JSON
	var parsed map[string]interface{}
	if json.Unmarshal([]byte(output), &parsed) == nil {
		t.Error("expected text output to NOT be valid JSON")
	}
}

func TestEmitResultDefaultIsText(t *testing.T) {
	// No -o flag at all -- should default to text
	cmd := &cobra.Command{Use: "deploy"}
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format")

	result := CommandResult{
		Version: "1",
		Command: "deploy",
		Status:  "pass",
		Summary: "deployed successfully",
		Hints:   []string{},
	}

	var buf bytes.Buffer
	err := EmitResult(cmd, result, &buf)
	if err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	// Default mode is text, so output should not be JSON
	var parsed map[string]interface{}
	if json.Unmarshal([]byte(buf.String()), &parsed) == nil {
		t.Error("expected default output to be text, not JSON")
	}
}

func TestCommandResultStatusValues(t *testing.T) {
	// Verify the two valid status values serialize correctly
	for _, status := range []string{"pass", "fail"} {
		result := CommandResult{
			Version: "1",
			Command: "test",
			Status:  status,
			Summary: "test",
			Hints:   []string{},
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("failed to marshal status %s: %v", status, err)
		}
		if !strings.Contains(string(data), `"status":"`+status+`"`) {
			t.Errorf("expected status %s in JSON, got: %s", status, string(data))
		}
	}
}

func TestTimingInfoFields(t *testing.T) {
	timing := TimingInfo{
		StartedAt:  "2026-02-16T10:30:00Z",
		DurationMs: 42,
	}

	data, err := json.Marshal(timing)
	if err != nil {
		t.Fatalf("failed to marshal TimingInfo: %v", err)
	}

	var decoded TimingInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TimingInfo: %v", err)
	}
	if decoded.StartedAt != "2026-02-16T10:30:00Z" {
		t.Errorf("expected startedAt preserved, got %s", decoded.StartedAt)
	}
	if decoded.DurationMs != 42 {
		t.Errorf("expected durationMs 42, got %d", decoded.DurationMs)
	}
}

func TestCommandResultWithResultsField(t *testing.T) {
	// Test that command-specific Results field serializes correctly
	type TestNodeResult struct {
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
	}
	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "3/3 tests passed",
		Hints:   []string{},
		Results: []TestNodeResult{
			{Name: "fetch-seps", Passed: true},
			{Name: "store-report", Passed: true},
			{Name: "notify", Passed: true},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"results"`) {
		t.Errorf("expected results field in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, "fetch-seps") {
		t.Errorf("expected fetch-seps in results, got: %s", jsonStr)
	}
}

func TestCommandResultOmitsEmptyOptionalFields(t *testing.T) {
	// Optional fields (Results, Phases, Execution, Manifests) should be omitted
	// when nil (omitempty tag)
	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "ok",
		Hints:   []string{},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	for _, field := range []string{`"results"`, `"phases"`, `"execution"`, `"manifests"`} {
		if strings.Contains(jsonStr, field) {
			t.Errorf("expected %s to be omitted when nil, got: %s", field, jsonStr)
		}
	}
}

func TestEmitResultTextModeWithHints(t *testing.T) {
	cmd := &cobra.Command{Use: "deploy"}
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format")
	cmd.ParseFlags([]string{"-o", "text"})

	result := CommandResult{
		Version: "1",
		Command: "deploy",
		Status:  "fail",
		Summary: "preflight checks failed",
		Hints:   []string{"check RBAC permissions", "verify namespace exists"},
		Timing: TimingInfo{
			StartedAt:  "2026-01-15T12:00:00Z",
			DurationMs: 250,
		},
	}

	var buf bytes.Buffer
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	output := buf.String()

	// Should contain FAIL prefix
	if !strings.Contains(output, "FAIL") {
		t.Errorf("expected FAIL in text output for fail status, got: %s", output)
	}

	// Should contain hints
	if !strings.Contains(output, "check RBAC permissions") {
		t.Errorf("expected first hint in output, got: %s", output)
	}
	if !strings.Contains(output, "verify namespace exists") {
		t.Errorf("expected second hint in output, got: %s", output)
	}

	// Should contain timing
	if !strings.Contains(output, "250ms") {
		t.Errorf("expected timing in output, got: %s", output)
	}
}

func TestEmitResultTextModePassPrefix(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format")
	cmd.ParseFlags([]string{"-o", "text"})

	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "all good",
		Hints:   []string{},
	}

	var buf bytes.Buffer
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	if !strings.Contains(buf.String(), "PASS") {
		t.Errorf("expected PASS in text output for pass status, got: %s", buf.String())
	}
}

func TestEmitResultTextModeZeroDurationOmitted(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format")
	cmd.ParseFlags([]string{"-o", "text"})

	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "quick",
		Hints:   []string{},
		Timing: TimingInfo{
			DurationMs: 0,
		},
	}

	var buf bytes.Buffer
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	// Zero duration should not appear in text output
	if strings.Contains(buf.String(), "0ms") {
		t.Errorf("expected zero duration to be omitted in text, got: %s", buf.String())
	}
}

func TestEmitResultJSONModeEndsWithNewline(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().StringP("output", "o", "text", "Output format")
	cmd.ParseFlags([]string{"-o", "json"})

	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  "pass",
		Summary: "ok",
		Hints:   []string{},
	}

	var buf bytes.Buffer
	if err := EmitResult(cmd, result, &buf); err != nil {
		t.Fatalf("EmitResult failed: %v", err)
	}

	// JSON output should end with newline for clean piping
	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("expected JSON output to end with newline, got: %q", output)
	}
}

func TestCommandResultWithManifestsField(t *testing.T) {
	type ManifestAction struct {
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Action string `json:"action"`
	}
	result := CommandResult{
		Version: "1",
		Command: "deploy",
		Status:  "pass",
		Summary: "deployed successfully",
		Hints:   []string{},
		Manifests: []ManifestAction{
			{Kind: "ConfigMap", Name: "sep-tracker-code", Action: "created"},
			{Kind: "Deployment", Name: "sep-tracker", Action: "updated"},
			{Kind: "Service", Name: "sep-tracker", Action: "unchanged"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"manifests"`) {
		t.Errorf("expected manifests field in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, "sep-tracker-code") {
		t.Errorf("expected manifest name in JSON, got: %s", jsonStr)
	}
}
