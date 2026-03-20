package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// NewPermissionsCmd creates the "permissions" parent command with subcommands.
func NewPermissionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permissions",
		Short: "Manage workflow ownership and access permissions",
	}
	cmd.AddCommand(newPermissionsGetCmd())
	cmd.AddCommand(newPermissionsSetCmd())
	cmd.AddCommand(newPermissionsChmodCmd())
	cmd.AddCommand(newPermissionsChgrpCmd())
	return cmd
}

// newPermissionsGetCmd creates "tntc permissions get <namespace> [<name>]".
// With 1 arg, shows namespace-level permissions. With 2 args, shows workflow permissions.
func newPermissionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <namespace> [<name>]",
		Short: "Show ownership and permissions for a namespace or workflow",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runPermissionsGet,
	}
}

func runPermissionsGet(cmd *cobra.Command, args []string) error {
	namespace := args[0]
	name := ""
	if len(args) == 2 {
		name = args[1]
	}
	outputFormat := flagString(cmd, "output")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.PermissionsGet(cmd.Context(), namespace, name)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("getting permissions: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("getting permissions: %w", err)
	}

	if outputFormat == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	out := cmd.OutOrStdout()
	if name == "" {
		fmt.Fprintf(out, "Namespace:    %s\n", namespace)
	} else {
		fmt.Fprintf(out, "Workflow:     %s/%s\n", namespace, name)
	}
	fmt.Fprintf(out, "Owner:        %s", result.OwnerEmail)
	if result.OwnerName != "" {
		fmt.Fprintf(out, " (%s)", result.OwnerName)
	}
	fmt.Fprintln(out)
	if result.OwnerSub != "" {
		fmt.Fprintf(out, "Subject:      %s\n", result.OwnerSub)
	}
	fmt.Fprintf(out, "Group:        %s\n", result.Group)
	fmt.Fprintf(out, "Mode:         %s\n", result.Mode)
	if result.AuthProvider != "" {
		fmt.Fprintf(out, "Auth:         %s\n", result.AuthProvider)
	}
	return nil
}

// newPermissionsSetCmd creates "tntc permissions set <namespace> [<name>] [--group <g>] [--mode <m>]".
// With 1 arg, updates namespace-level permissions. With 2 args, updates workflow permissions.
func newPermissionsSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <namespace> [<name>]",
		Short: "Update group or mode for a namespace or workflow",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runPermissionsSet,
	}
	cmd.Flags().String("group", "", "New group name")
	cmd.Flags().String("mode", "", "New permissions mode (preset e.g. group-read, or raw e.g. rwxr-x---)")
	return cmd
}

func runPermissionsSet(cmd *cobra.Command, args []string) error {
	namespace := args[0]
	name := ""
	if len(args) == 2 {
		name = args[1]
	}
	group, _ := cmd.Flags().GetString("group")
	mode, _ := cmd.Flags().GetString("mode")
	outputFormat := flagString(cmd, "output")

	if group == "" && mode == "" {
		return fmt.Errorf("at least one of --group or --mode must be specified")
	}

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.PermissionsSet(cmd.Context(), namespace, name, group, mode)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w\n  hint: only the owner can change permissions", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("setting permissions: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("setting permissions: %w", err)
	}

	if outputFormat == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	out := cmd.OutOrStdout()
	if result.Updated {
		if name == "" {
			fmt.Fprintf(out, "Updated namespace %s\n", namespace)
		} else {
			fmt.Fprintf(out, "Updated %s/%s\n", namespace, name)
		}
		if result.Group != "" {
			fmt.Fprintf(out, "  Group: %s\n", result.Group)
		}
		if result.Mode != "" {
			fmt.Fprintf(out, "  Mode:  %s\n", result.Mode)
		}
	} else {
		if name == "" {
			fmt.Fprintf(out, "No changes for namespace %s\n", namespace)
		} else {
			fmt.Fprintf(out, "No changes for %s/%s\n", namespace, name)
		}
	}
	return nil
}

// newPermissionsChmodCmd creates "tntc permissions chmod <mode> <namespace> [<name>]".
// With 2 args, sets mode on the namespace. With 3 args, sets mode on the workflow.
func newPermissionsChmodCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chmod <mode> <namespace> [<name>]",
		Short: "Set permissions mode for a namespace or workflow (POSIX-style shorthand)",
		Args:  cobra.RangeArgs(2, 3),
		RunE:  runPermissionsChmod,
	}
}

func runPermissionsChmod(cmd *cobra.Command, args []string) error {
	mode := args[0]
	namespace := args[1]
	name := ""
	if len(args) == 3 {
		name = args[2]
	}

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.PermissionsSet(cmd.Context(), namespace, name, "", mode)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w\n  hint: only the owner can change permissions", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("chmod: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("chmod: %w", err)
	}

	if result.Updated {
		if name == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "mode %s → namespace %s\n", result.Mode, namespace)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "mode %s → %s/%s\n", result.Mode, namespace, name)
		}
	} else {
		if name == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "No changes for namespace %s\n", namespace)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "No changes for %s/%s\n", namespace, name)
		}
	}
	return nil
}

// newPermissionsChgrpCmd creates "tntc permissions chgrp <group> <namespace> [<name>]".
// With 2 args, sets group on the namespace. With 3 args, sets group on the workflow.
func newPermissionsChgrpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chgrp <group> <namespace> [<name>]",
		Short: "Set group ownership for a namespace or workflow (POSIX-style shorthand)",
		Args:  cobra.RangeArgs(2, 3),
		RunE:  runPermissionsChgrp,
	}
}

func runPermissionsChgrp(cmd *cobra.Command, args []string) error {
	group := args[0]
	namespace := args[1]
	name := ""
	if len(args) == 3 {
		name = args[2]
	}

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.PermissionsSet(cmd.Context(), namespace, name, group, "")
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w\n  hint: only the owner can change permissions", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("chgrp: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("chgrp: %w", err)
	}

	if result.Updated {
		if name == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "group %s → namespace %s\n", result.Group, namespace)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "group %s → %s/%s\n", result.Group, namespace, name)
		}
	} else {
		if name == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "No changes for namespace %s\n", namespace)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "No changes for %s/%s\n", namespace, name)
		}
	}
	return nil
}
