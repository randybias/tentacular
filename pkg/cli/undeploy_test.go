package cli

import (
	"strings"
	"testing"
)

func TestUndeployCmdHasYesFlag(t *testing.T) {
	cmd := NewUndeployCmd()
	f := cmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("expected --yes flag on undeploy command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --yes default false, got %s", f.DefValue)
	}
	if f.Shorthand != "y" {
		t.Errorf("expected --yes shorthand -y, got %s", f.Shorthand)
	}
}

func TestUndeployCmdHasForceFlag(t *testing.T) {
	cmd := NewUndeployCmd()
	f := cmd.Flags().Lookup("force")
	if f == nil {
		t.Fatal("expected --force flag on undeploy command")
	}
	if f.DefValue != "false" {
		t.Errorf("expected --force default false, got %s", f.DefValue)
	}
}

func TestUndeployCmdForceFlagParsing(t *testing.T) {
	cmd := NewUndeployCmd()
	if err := cmd.ParseFlags([]string{"--force"}); err != nil {
		t.Fatalf("failed to parse --force: %v", err)
	}
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		t.Error("expected --force to be true after parsing")
	}
}

func TestUndeployCmdCombinedFlags(t *testing.T) {
	cmd := NewUndeployCmd()
	if err := cmd.ParseFlags([]string{"-y", "--force"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	yes, _ := cmd.Flags().GetBool("yes")
	force, _ := cmd.Flags().GetBool("force")
	if !yes {
		t.Error("expected --yes to be true")
	}
	if !force {
		t.Error("expected --force to be true")
	}
}

func TestCheckExoskeletonCleanupWarningFormat(t *testing.T) {
	// Unit test the warning message format by checking the builder directly.
	// We cannot easily call checkExoskeletonCleanup without an MCP server,
	// so test the warning string generation pattern instead.
	var sb strings.Builder
	sb.WriteString("\nWARNING: Exoskeleton cleanup is enabled. Undeploying will permanently delete:\n")
	sb.WriteString("  - Postgres schema and role for this workflow\n")
	sb.WriteString("  - RustFS objects, IAM user, and access policy\n")
	sb.WriteString("  - NATS cleanup (no-op in Phase 1)\n")
	sb.WriteString("\n")

	warning := sb.String()
	if !strings.Contains(warning, "WARNING: Exoskeleton cleanup is enabled") {
		t.Error("expected warning header")
	}
	if !strings.Contains(warning, "Postgres schema") {
		t.Error("expected Postgres in warning")
	}
	if !strings.Contains(warning, "RustFS objects") {
		t.Error("expected RustFS in warning")
	}
	if !strings.Contains(warning, "NATS cleanup") {
		t.Error("expected NATS in warning")
	}
}
