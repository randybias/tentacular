// Unit tests for audit.go types and command wiring.
//
// The audit command's RunE depends on an MCP client, so we cannot test the
// full execution path without a running server. These tests cover:
//   - AuditResult JSON structure and marshaling
//   - omitempty behavior for optional fields (Missing, Extra, Details)
//   - SecretsAudit with missing and extra keys
//   - CronJobsAudit detail formatting
//   - NewAuditCommand flag existence and argument validation

package cli

import (
	"encoding/json"
	"testing"
)

// --- AuditResult JSON Structure Tests ---

// TestAuditResultJSONStructure verifies that a fully populated AuditResult
// can be marshaled to JSON without error. This catches struct tag issues.
func TestAuditResultJSONStructure(t *testing.T) {
	// Verify AuditResult struct can be marshaled to JSON
	result := AuditResult{
		NetworkPolicy: NetworkPolicyAudit{
			Expected: map[string]any{
				"egressRuleCount":  3,
				"ingressRuleCount": 1,
			},
			Actual: map[string]any{
				"rules": "mock",
			},
			Status:  "match",
			Details: []string{},
		},
		Secrets: SecretsAudit{
			ExpectedKeys: []string{"postgres.password", "github.token"},
			ActualKeys:   []string{"postgres.password", "github.token"},
			Missing:      []string{},
			Extra:        []string{},
			Status:       "match",
		},
		CronJobs: CronJobsAudit{
			ExpectedCount: 2,
			ActualCount:   2,
			Status:        "match",
			Details:       []string{},
		},
		Overall: "pass",
	}

	// Should marshal without error
	_, err := json.Marshal(result)
	if err != nil {
		t.Errorf("failed to marshal AuditResult: %v", err)
	}
}

// TestAuditResultFailureState verifies that failure states are properly
// represented in the struct. Each sub-audit can independently report
// its own status (missing, mismatch) while Overall summarizes.
func TestAuditResultFailureState(t *testing.T) {
	// Verify failure states are properly represented
	result := AuditResult{
		NetworkPolicy: NetworkPolicyAudit{
			Status:  "missing",
			Details: []string{"NetworkPolicy not found"},
		},
		Secrets: SecretsAudit{
			ExpectedKeys: []string{"postgres.password"},
			ActualKeys:   []string{},
			Missing:      []string{"postgres.password"},
			Status:       "mismatch",
		},
		CronJobs: CronJobsAudit{
			ExpectedCount: 1,
			ActualCount:   0,
			Status:        "mismatch",
			Details:       []string{"Expected 1 CronJobs, found 0"},
		},
		Overall: "fail",
	}

	if result.Overall != "fail" {
		t.Error("expected overall status to be 'fail'")
	}
	if result.NetworkPolicy.Status != "missing" {
		t.Error("expected NetworkPolicy status to be 'missing'")
	}
	if result.Secrets.Status != "mismatch" {
		t.Error("expected Secrets status to be 'mismatch'")
	}
	if result.CronJobs.Status != "mismatch" {
		t.Error("expected CronJobs status to be 'mismatch'")
	}
}

// --- AuditResult JSON round-trip ---

// TestAuditResultJSONRoundTrip verifies that a fully populated AuditResult
// survives marshal → unmarshal without data loss. This catches field name
// mismatches between json tags and Go struct fields.
func TestAuditResultJSONRoundTrip(t *testing.T) {
	original := AuditResult{
		NetworkPolicy: NetworkPolicyAudit{
			Expected: map[string]any{
				"egressRuleCount":  float64(3),
				"ingressRuleCount": float64(1),
			},
			Actual: map[string]any{
				"egressRuleCount":  float64(3),
				"ingressRuleCount": float64(1),
			},
			Status:  "match",
			Details: []string{"all rules verified"},
		},
		Secrets: SecretsAudit{
			ExpectedKeys: []string{"db.password", "api.token"},
			ActualKeys:   []string{"db.password", "api.token"},
			Missing:      []string{},
			Extra:        []string{},
			Status:       "match",
		},
		CronJobs: CronJobsAudit{
			ExpectedCount: 2,
			ActualCount:   2,
			Status:        "match",
			Details:       []string{"2 CronJobs verified"},
		},
		Overall: "pass",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded AuditResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Overall != "pass" {
		t.Errorf("Overall: got %q, want %q", decoded.Overall, "pass")
	}
	if decoded.NetworkPolicy.Status != "match" {
		t.Errorf("NetworkPolicy.Status: got %q, want %q", decoded.NetworkPolicy.Status, "match")
	}
	if len(decoded.Secrets.ExpectedKeys) != 2 {
		t.Errorf("Secrets.ExpectedKeys: got %d, want 2", len(decoded.Secrets.ExpectedKeys))
	}
	if decoded.CronJobs.ExpectedCount != 2 {
		t.Errorf("CronJobs.ExpectedCount: got %d, want 2", decoded.CronJobs.ExpectedCount)
	}
	if decoded.CronJobs.ActualCount != 2 {
		t.Errorf("CronJobs.ActualCount: got %d, want 2", decoded.CronJobs.ActualCount)
	}
}

// --- omitempty behavior ---

// TestAuditResultOmitsEmptyOptionalFields verifies that fields tagged with
// omitempty (Missing, Extra, Details) are omitted from JSON when empty/nil.
// This keeps the JSON output clean for passing audits.
func TestAuditResultOmitsEmptyOptionalFields(t *testing.T) {
	result := AuditResult{
		NetworkPolicy: NetworkPolicyAudit{
			Status: "match",
			// Details is nil — should be omitted from JSON
		},
		Secrets: SecretsAudit{
			ExpectedKeys: []string{"db.password"},
			ActualKeys:   []string{"db.password"},
			Status:       "match",
			// Missing and Extra are nil — should be omitted
		},
		CronJobs: CronJobsAudit{
			ExpectedCount: 1,
			ActualCount:   1,
			Status:        "match",
			// Details is nil — should be omitted
		},
		Overall: "pass",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	jsonStr := string(data)

	// Fields with omitempty should not appear when nil/empty
	if containsJSONKey(jsonStr, "details") {
		t.Errorf("expected 'details' to be omitted when nil, got: %s", jsonStr)
	}
	if containsJSONKey(jsonStr, "missing") {
		t.Errorf("expected 'missing' to be omitted when nil, got: %s", jsonStr)
	}
	if containsJSONKey(jsonStr, "extra") {
		t.Errorf("expected 'extra' to be omitted when nil, got: %s", jsonStr)
	}
}

// containsJSONKey checks if a JSON key (e.g. "details") appears in the raw JSON string.
// This is a simple helper — it looks for `"key":` pattern in the serialized output.
func containsJSONKey(jsonStr, key string) bool {
	return len(jsonStr) > 0 && (len(key) > 0 && (jsonStr != "" && contains(jsonStr, `"`+key+`"`)))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- SecretsAudit fields ---

// TestSecretsAuditMissingAndExtraKeys verifies that the Missing and Extra
// fields correctly represent the gap between expected and actual secret keys.
// This is the core data that the audit text output uses to report drift.
func TestSecretsAuditMissingAndExtraKeys(t *testing.T) {
	audit := SecretsAudit{
		ExpectedKeys: []string{"db.password", "api.token", "cache.key"},
		ActualKeys:   []string{"db.password", "legacy.key"},
		Missing:      []string{"api.token", "cache.key"},
		Extra:        []string{"legacy.key"},
		Status:       "mismatch",
	}

	if len(audit.Missing) != 2 {
		t.Errorf("Missing: got %d keys, want 2", len(audit.Missing))
	}
	if len(audit.Extra) != 1 {
		t.Errorf("Extra: got %d keys, want 1", len(audit.Extra))
	}

	// Verify JSON round-trip preserves the arrays
	data, _ := json.Marshal(audit)
	var decoded SecretsAudit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Missing[0] != "api.token" {
		t.Errorf("Missing[0]: got %q, want %q", decoded.Missing[0], "api.token")
	}
	if decoded.Extra[0] != "legacy.key" {
		t.Errorf("Extra[0]: got %q, want %q", decoded.Extra[0], "legacy.key")
	}
}

// --- CronJobsAudit fields ---

// TestCronJobsAuditDetailFormatting verifies that CronJobsAudit Details
// can hold descriptive mismatch messages and survive JSON serialization.
func TestCronJobsAuditDetailFormatting(t *testing.T) {
	audit := CronJobsAudit{
		ExpectedCount: 3,
		ActualCount:   1,
		Status:        "mismatch",
		Details:       []string{"Expected 3 CronJobs, found 1", "Missing: backup-daily, cleanup-weekly"},
	}

	if len(audit.Details) != 2 {
		t.Errorf("Details: got %d entries, want 2", len(audit.Details))
	}

	// Verify JSON round-trip
	data, _ := json.Marshal(audit)
	var decoded CronJobsAudit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ExpectedCount != 3 {
		t.Errorf("ExpectedCount: got %d, want 3", decoded.ExpectedCount)
	}
	if decoded.ActualCount != 1 {
		t.Errorf("ActualCount: got %d, want 1", decoded.ActualCount)
	}
}

// --- NewAuditCommand wiring ---

// TestAuditCommandHasOutputFlag verifies that --output (-o) is registered
// with "text" as the default. The audit command supports "text" and "json".
func TestAuditCommandHasOutputFlag(t *testing.T) {
	cmd := NewAuditCommand()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("expected --output flag on audit command")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected -o shorthand, got %q", f.Shorthand)
	}
	if f.DefValue != "text" {
		t.Errorf("expected default 'text' for --output, got %q", f.DefValue)
	}
}

// TestAuditCommandRequiresExactlyOneArg verifies that the audit command
// rejects invocations with zero or multiple arguments. The single required
// argument is the workflow directory path.
func TestAuditCommandRequiresExactlyOneArg(t *testing.T) {
	// Zero args should fail
	cmd := NewAuditCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when no args provided")
	}

	// Two args should fail
	cmd = NewAuditCommand()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"dir1", "dir2"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when two args provided")
	}
}
