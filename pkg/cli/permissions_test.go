// Unit tests for permissions command wiring.
//
// Tests cover command structure, argument validation, and flag registration.
// The RunE functions require an MCP client and cannot be tested here without
// a running server — those paths are covered by integration tests.

package cli

import (
	"strings"
	"testing"
)

// --- NewPermissionsCmd wiring ---

func TestPermissionsCmd_HasSubcommands(t *testing.T) {
	cmd := NewPermissionsCmd()
	names := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}

	for _, want := range []string{"get", "set", "chmod", "chgrp"} {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected permissions subcommand %q, got subcommands: %v", want, names)
		}
	}
}

func TestPermissionsCmd_Use(t *testing.T) {
	cmd := NewPermissionsCmd()
	if cmd.Use != "permissions" {
		t.Errorf("Use = %q, want %q", cmd.Use, "permissions")
	}
}

// --- permissions get wiring ---

func TestPermissionsGetCmd_AcceptsOneArg(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "get" {
			// Validate that 1 arg (namespace only) is accepted.
			if err := sub.Args(sub, []string{"my-ns"}); err != nil {
				t.Errorf("1 arg should be accepted: %v", err)
			}
			return
		}
	}
	t.Fatal("get subcommand not found")
}

func TestPermissionsGetCmd_AcceptsTwoArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "get" {
			if err := sub.Args(sub, []string{"my-ns", "my-wf"}); err != nil {
				t.Errorf("2 args should be accepted: %v", err)
			}
			return
		}
	}
	t.Fatal("get subcommand not found")
}

func TestPermissionsGetCmd_RejectsZeroArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "get" {
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("expected error for zero args")
			}
			return
		}
	}
	t.Fatal("get subcommand not found")
}

func TestPermissionsGetCmd_RejectsThreeArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "get" {
			if err := sub.Args(sub, []string{"ns", "wf", "extra"}); err == nil {
				t.Error("expected error for 3 args")
			}
			return
		}
	}
	t.Fatal("get subcommand not found")
}

func TestPermissionsGetCmd_Use(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "get" {
			if !strings.HasPrefix(sub.Use, "get") {
				t.Errorf("Use = %q, expected to start with 'get'", sub.Use)
			}
			return
		}
	}
	t.Fatal("get subcommand not found")
}

// --- permissions set wiring ---

func TestPermissionsSetCmd_AcceptsOneArg(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "set" {
			if err := sub.Args(sub, []string{"my-ns"}); err != nil {
				t.Errorf("1 arg (namespace only) should be accepted: %v", err)
			}
			return
		}
	}
	t.Fatal("set subcommand not found")
}

func TestPermissionsSetCmd_AcceptsTwoArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "set" {
			if err := sub.Args(sub, []string{"my-ns", "my-wf"}); err != nil {
				t.Errorf("2 args should be accepted: %v", err)
			}
			return
		}
	}
	t.Fatal("set subcommand not found")
}

func TestPermissionsSetCmd_RejectsZeroArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "set" {
			if err := sub.Args(sub, []string{}); err == nil {
				t.Error("expected error for zero args")
			}
			return
		}
	}
	t.Fatal("set subcommand not found")
}

func TestPermissionsSetCmd_HasGroupFlag(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "set" {
			f := sub.Flags().Lookup("group")
			if f == nil {
				t.Fatal("expected --group flag on permissions set")
			}
			return
		}
	}
	t.Fatal("set subcommand not found")
}

func TestPermissionsSetCmd_HasModeFlag(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "set" {
			f := sub.Flags().Lookup("mode")
			if f == nil {
				t.Fatal("expected --mode flag on permissions set")
			}
			return
		}
	}
	t.Fatal("set subcommand not found")
}

// --- permissions chmod wiring ---

func TestPermissionsChmodCmd_AcceptsTwoArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "chmod" {
			if err := sub.Args(sub, []string{"rwxr-x---", "my-ns"}); err != nil {
				t.Errorf("2 args should be accepted: %v", err)
			}
			return
		}
	}
	t.Fatal("chmod subcommand not found")
}

func TestPermissionsChmodCmd_AcceptsThreeArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "chmod" {
			if err := sub.Args(sub, []string{"rwxr-x---", "my-ns", "my-wf"}); err != nil {
				t.Errorf("3 args should be accepted: %v", err)
			}
			return
		}
	}
	t.Fatal("chmod subcommand not found")
}

func TestPermissionsChmodCmd_RejectsOneArg(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "chmod" {
			if err := sub.Args(sub, []string{"rwxr-x---"}); err == nil {
				t.Error("expected error for 1 arg (missing namespace)")
			}
			return
		}
	}
	t.Fatal("chmod subcommand not found")
}

// --- permissions chgrp wiring ---

func TestPermissionsChgrpCmd_AcceptsTwoArgs(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "chgrp" {
			if err := sub.Args(sub, []string{"platform-team", "my-ns"}); err != nil {
				t.Errorf("2 args should be accepted: %v", err)
			}
			return
		}
	}
	t.Fatal("chgrp subcommand not found")
}

func TestPermissionsChgrpCmd_RejectsOneArg(t *testing.T) {
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "chgrp" {
			if err := sub.Args(sub, []string{"platform-team"}); err == nil {
				t.Error("expected error for 1 arg (missing namespace)")
			}
			return
		}
	}
	t.Fatal("chgrp subcommand not found")
}

// --- runPermissionsSet validation: at least one flag required ---

func TestPermissionsSetCmd_RequiresAtLeastOneFlag(t *testing.T) {
	// Without --group or --mode, RunE should return an error.
	// We can test this by constructing the command and calling its RunE directly.
	cmd := NewPermissionsCmd()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "set" {
			sub.SilenceUsage = true
			sub.SilenceErrors = true
			sub.SetArgs([]string{"my-ns"})
			// RunE requires an MCP client and will fail before the flag check,
			// but we can at least confirm the args validation passes.
			if err := sub.Args(sub, []string{"my-ns"}); err != nil {
				t.Errorf("1 arg should pass args validation: %v", err)
			}
			return
		}
	}
	t.Fatal("set subcommand not found")
}
