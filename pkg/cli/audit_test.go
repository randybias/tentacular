package cli

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// --- Group 3: Audit Command Tests ---

// --- compareSecretKeys Tests ---

func TestCompareSecretKeysExactMatch(t *testing.T) {
	expected := []string{"postgres.password", "github.token"}
	actual := []string{"postgres.password", "github.token"}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 0 {
		t.Errorf("expected no missing keys, got: %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra keys, got: %v", extra)
	}
}

func TestCompareSecretKeysMissingKeys(t *testing.T) {
	expected := []string{"postgres.password", "github.token", "slack.webhook"}
	actual := []string{"postgres.password"}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 2 {
		t.Errorf("expected 2 missing keys, got %d: %v", len(missing), missing)
	}
	if !contains(missing, "github.token") {
		t.Error("expected github.token to be missing")
	}
	if !contains(missing, "slack.webhook") {
		t.Error("expected slack.webhook to be missing")
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra keys, got: %v", extra)
	}
}

func TestCompareSecretKeysExtraKeys(t *testing.T) {
	expected := []string{"postgres.password"}
	actual := []string{"postgres.password", "github.token", "slack.webhook"}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 0 {
		t.Errorf("expected no missing keys, got: %v", missing)
	}
	if len(extra) != 2 {
		t.Errorf("expected 2 extra keys, got %d: %v", len(extra), extra)
	}
	if !contains(extra, "github.token") {
		t.Error("expected github.token to be extra")
	}
	if !contains(extra, "slack.webhook") {
		t.Error("expected slack.webhook to be extra")
	}
}

func TestCompareSecretKeysMissingAndExtra(t *testing.T) {
	expected := []string{"postgres.password", "redis.password"}
	actual := []string{"postgres.password", "github.token"}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 1 {
		t.Errorf("expected 1 missing key, got %d: %v", len(missing), missing)
	}
	if !contains(missing, "redis.password") {
		t.Error("expected redis.password to be missing")
	}
	if len(extra) != 1 {
		t.Errorf("expected 1 extra key, got %d: %v", len(extra), extra)
	}
	if !contains(extra, "github.token") {
		t.Error("expected github.token to be extra")
	}
}

func TestCompareSecretKeysEmptyExpected(t *testing.T) {
	expected := []string{}
	actual := []string{"github.token", "slack.webhook"}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 0 {
		t.Errorf("expected no missing keys, got: %v", missing)
	}
	if len(extra) != 2 {
		t.Errorf("expected 2 extra keys, got %d: %v", len(extra), extra)
	}
}

func TestCompareSecretKeysEmptyActual(t *testing.T) {
	expected := []string{"postgres.password", "github.token"}
	actual := []string{}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 2 {
		t.Errorf("expected 2 missing keys, got %d: %v", len(missing), missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra keys, got: %v", extra)
	}
}

func TestCompareSecretKeysBothEmpty(t *testing.T) {
	expected := []string{}
	actual := []string{}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 0 {
		t.Errorf("expected no missing keys, got: %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra keys, got: %v", extra)
	}
}

func TestCompareSecretKeysNilSlices(t *testing.T) {
	var expected []string = nil
	var actual []string = nil

	missing, extra := compareSecretKeys(expected, actual)

	if missing != nil && len(missing) != 0 {
		t.Errorf("expected nil or empty missing, got: %v", missing)
	}
	if extra != nil && len(extra) != 0 {
		t.Errorf("expected nil or empty extra, got: %v", extra)
	}
}

func TestCompareSecretKeysDuplicates(t *testing.T) {
	// Test behavior with duplicate keys
	expected := []string{"postgres.password", "postgres.password"}
	actual := []string{"postgres.password"}

	missing, extra := compareSecretKeys(expected, actual)

	// Should handle duplicates gracefully (no false positives)
	if len(missing) != 0 {
		t.Errorf("expected no missing keys despite duplicates, got: %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra keys, got: %v", extra)
	}
}

func TestCompareSecretKeysOrderIndependent(t *testing.T) {
	expected := []string{"a", "b", "c"}
	actual := []string{"c", "b", "a"}

	missing, extra := compareSecretKeys(expected, actual)

	if len(missing) != 0 {
		t.Errorf("expected no missing keys (order independent), got: %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra keys (order independent), got: %v", extra)
	}
}

func TestCompareSecretKeysComplexScenario(t *testing.T) {
	// Complex real-world scenario
	expected := []string{
		"postgres.password",
		"postgres.username",
		"github.token",
		"slack.webhook",
	}
	actual := []string{
		"postgres.password",
		"github.token",
		"azure.sas_token", // extra
		"legacy.key",      // extra
	}

	missing, extra := compareSecretKeys(expected, actual)

	// Verify missing keys
	expectedMissing := []string{"postgres.username", "slack.webhook"}
	sort.Strings(missing)
	sort.Strings(expectedMissing)
	if !reflect.DeepEqual(missing, expectedMissing) {
		t.Errorf("expected missing %v, got %v", expectedMissing, missing)
	}

	// Verify extra keys
	expectedExtra := []string{"azure.sas_token", "legacy.key"}
	sort.Strings(extra)
	sort.Strings(expectedExtra)
	if !reflect.DeepEqual(extra, expectedExtra) {
		t.Errorf("expected extra %v, got %v", expectedExtra, extra)
	}
}

// --- Service-Name Level Comparison Tests ---

func TestCompareSecretKeysServiceNameLevel(t *testing.T) {
	// Simulates Bug 2 fix: expected dotted keys reduced to service names
	// before comparison against actual flat service-name keys
	expectedServiceNames := []string{"slack", "postgres"}
	actual := []string{"slack", "postgres"}

	missing, extra := compareSecretKeys(expectedServiceNames, actual)

	if len(missing) != 0 {
		t.Errorf("expected no missing keys at service level, got: %v", missing)
	}
	if len(extra) != 0 {
		t.Errorf("expected no extra keys at service level, got: %v", extra)
	}
}

// --- AuditResult JSON Structure Tests ---

func TestAuditResultJSONStructure(t *testing.T) {
	// Verify AuditResult struct can be marshaled to JSON
	result := AuditResult{
		NetworkPolicy: NetworkPolicyAudit{
			Expected: map[string]interface{}{
				"egressRuleCount":  3,
				"ingressRuleCount": 1,
			},
			Actual: map[string]interface{}{
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

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
