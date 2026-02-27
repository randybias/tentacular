package cli

import (
	"encoding/json"
	"testing"
)

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
