package cli

import (
	"testing"
)

// TestClusterCmdHasNoInstallSubcommand verifies that the cluster install
// subcommand has been removed (Phase 1).
func TestClusterCmdHasNoInstallSubcommand(t *testing.T) {
	cmd := NewClusterCmd()
	for _, sub := range cmd.Commands() {
		if sub.Use == "install" || sub.Name() == "install" {
			t.Error("cluster install subcommand should not exist after Phase 1 removal")
		}
	}
}

// TestClusterCmdHasCheckSubcommand verifies the check subcommand remains.
func TestClusterCmdHasCheckSubcommand(t *testing.T) {
	cmd := NewClusterCmd()
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "check" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster check subcommand to exist")
	}
}

// TestClusterCmdHasProfileSubcommand verifies the profile subcommand remains.
func TestClusterCmdHasProfileSubcommand(t *testing.T) {
	cmd := NewClusterCmd()
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "profile" {
			found = true
		}
	}
	if !found {
		t.Error("expected cluster profile subcommand to exist")
	}
}

// TestClusterCmdSubcommandCount verifies there are exactly 2 subcommands
// (check and profile) -- no install.
func TestClusterCmdSubcommandCount(t *testing.T) {
	cmd := NewClusterCmd()
	subs := cmd.Commands()
	if len(subs) != 2 {
		names := make([]string, len(subs))
		for i, s := range subs {
			names[i] = s.Name()
		}
		t.Errorf("expected 2 cluster subcommands (check, profile), got %d: %v", len(subs), names)
	}
}
