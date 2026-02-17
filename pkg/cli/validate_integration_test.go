package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Integration tests that run the actual tntc binary

func TestValidateIntegrationEmptyContract(t *testing.T) {
	// Skip if tntc binary doesn't exist
	if _, err := exec.LookPath("tntc"); err != nil {
		if _, err := os.Stat("../../cmd/tntc/main.go"); err != nil {
			t.Skip("tntc binary or source not available")
		}
	}

	yamlContent := `
name: test-workflow
version: "1.0"
triggers:
  - type: manual
nodes:
  handler:
    path: ./handler.ts
contract:
  version: "1"
  dependencies: {}
`
	dir := t.TempDir()
	workflowPath := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Run validate with JSON output
	cmd := exec.Command("go", "run", "../../cmd/tntc", "validate", dir, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	// Parse JSON
	var result ValidateResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

	// Verify
	if result.Workflow != "test-workflow" {
		t.Errorf("expected workflow test-workflow, got %s", result.Workflow)
	}
	if !result.HasContract {
		t.Error("expected hasContract to be true")
	}
	if len(result.EgressRules) < 2 {
		t.Errorf("expected at least 2 DNS egress rules, got %d", len(result.EgressRules))
	}

	t.Logf("✅ Empty contract validated: egress=%d", len(result.EgressRules))
}

func TestValidateIntegrationFullContract(t *testing.T) {
	if _, err := exec.LookPath("tntc"); err != nil {
		if _, err := os.Stat("../../cmd/tntc/main.go"); err != nil {
			t.Skip("tntc binary or source not available")
		}
	}

	yamlContent := `
name: test-workflow
version: "1.0"
triggers:
  - type: webhook
    path: /hook
nodes:
  handler:
    path: ./handler.ts
contract:
  version: "1"
  dependencies:
    github:
      protocol: https
      host: api.github.com
      port: 443
      auth:
        type: bearer-token
        secret: github.token
    postgres:
      protocol: postgresql
      host: postgres.svc.cluster.local
      port: 5432
      database: appdb
      user: postgres
      auth:
        type: password
        secret: postgres.password
`
	dir := t.TempDir()
	workflowPath := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Run validate
	cmd := exec.Command("go", "run", "../../cmd/tntc", "validate", dir, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	var result ValidateResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

	// Verify secrets
	if len(result.Secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d: %v", len(result.Secrets), result.Secrets)
	}

	// Verify egress (DNS + 2 deps)
	if len(result.EgressRules) < 4 {
		t.Errorf("expected at least 4 egress rules, got %d", len(result.EgressRules))
	}

	// Verify ingress
	if len(result.IngressRules) < 1 {
		t.Errorf("expected at least 1 ingress rule, got %d", len(result.IngressRules))
	}

	t.Logf("✅ Full contract: secrets=%d, egress=%d, ingress=%d",
		len(result.Secrets), len(result.EgressRules), len(result.IngressRules))
}

func TestValidateIntegrationTextOutput(t *testing.T) {
	if _, err := exec.LookPath("tntc"); err != nil {
		if _, err := os.Stat("../../cmd/tntc/main.go"); err != nil {
			t.Skip("tntc binary or source not available")
		}
	}

	yamlContent := `
name: test-workflow
version: "1.0"
triggers:
  - type: manual
nodes:
  handler:
    path: ./handler.ts
`
	dir := t.TempDir()
	workflowPath := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(workflowPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	// Run validate (text mode)
	cmd := exec.Command("go", "run", "../../cmd/tntc", "validate", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "✓") {
		t.Errorf("expected checkmark in output: %s", outputStr)
	}
	if !strings.Contains(outputStr, "valid") {
		t.Errorf("expected 'valid' in output: %s", outputStr)
	}

	t.Logf("✅ Text output validated")
}
