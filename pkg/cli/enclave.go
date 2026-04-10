package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/mcp"
)

// NewEnclaveCmd creates the "enclave" parent command with subcommands.
func NewEnclaveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enclave",
		Short: "Manage enclaves (collaboration workspaces)",
	}
	cmd.AddCommand(newEnclaveListCmd())
	cmd.AddCommand(newEnclaveInfoCmd())
	cmd.AddCommand(newEnclaveProvisionCmd())
	cmd.AddCommand(newEnclaveSyncCmd())
	cmd.AddCommand(newEnclaveDeprovisionCmd())
	return cmd
}

// --- enclave list ---

func newEnclaveListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List enclaves",
		Args:  cobra.NoArgs,
		RunE:  runEnclaveList,
	}
	cmd.Flags().Bool("mine", false, "Only show enclaves you belong to")
	return cmd
}

func runEnclaveList(cmd *cobra.Command, _ []string) error {
	mine, _ := cmd.Flags().GetBool("mine")
	outputFormat := flagString(cmd, "output")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	// When --mine is set, pass the caller's email to filter server-side.
	// The server resolves the caller identity from the auth token; we pass
	// an empty string and let the server handle the --mine logic via the
	// caller_email field when populated.
	callerEmail := ""
	if mine {
		// Resolve caller email from whoami if available; otherwise pass sentinel.
		callerEmail = resolveCallerEmail(cmd, mcpClient)
	}

	items, err := mcpClient.EnclaveList(cmd.Context(), callerEmail)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("listing enclaves: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("listing enclaves: %w", err)
	}

	if outputFormat == "json" {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	out := cmd.OutOrStdout()
	if len(items) == 0 {
		fmt.Fprintln(out, "No enclaves found.")
		return nil
	}
	for _, e := range items {
		status := e.Status
		if status == "" {
			status = "active"
		}
		platform := e.Platform
		if platform == "" {
			platform = "-"
		}
		fmt.Fprintf(out, "%-30s  owner=%-30s  status=%-8s  platform=%s\n",
			e.Name, e.Owner, status, platform)
	}
	return nil
}

// --- enclave info ---

func newEnclaveInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show details for an enclave",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnclaveInfo,
	}
}

func runEnclaveInfo(cmd *cobra.Command, args []string) error {
	name := args[0]
	outputFormat := flagString(cmd, "output")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.EnclaveInfo(cmd.Context(), name)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("getting enclave info: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("getting enclave info: %w", err)
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
	fmt.Fprintf(out, "Enclave:     %s\n", result.Name)
	fmt.Fprintf(out, "Owner:       %s\n", result.Owner)
	if len(result.Members) > 0 {
		fmt.Fprintf(out, "Members:     %s\n", strings.Join(result.Members, ", "))
	}
	if result.Platform != "" {
		fmt.Fprintf(out, "Platform:    %s\n", result.Platform)
	}
	if result.ChannelID != "" {
		fmt.Fprintf(out, "Channel ID:  %s\n", result.ChannelID)
	}
	if result.ChannelName != "" {
		fmt.Fprintf(out, "Channel:     %s\n", result.ChannelName)
	}
	if result.QuotaPreset != "" {
		fmt.Fprintf(out, "Quota:       %s\n", result.QuotaPreset)
	}
	if result.Status != "" {
		fmt.Fprintf(out, "Status:      %s\n", result.Status)
	}
	if result.TentacleCount > 0 {
		fmt.Fprintf(out, "Tentacles:   %d\n", result.TentacleCount)
	}
	return nil
}

// --- enclave provision ---

func newEnclaveProvisionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision <name>",
		Short: "Provision a new enclave",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnclaveProvision,
	}
	cmd.Flags().String("owner", "", "Email of the enclave owner")
	cmd.Flags().String("members", "", "Comma-separated list of member emails")
	cmd.Flags().String("platform", "", "Platform type (e.g. slack)")
	cmd.Flags().String("channel-id", "", "Platform channel ID")
	cmd.Flags().String("channel-name", "", "Platform channel name")
	cmd.Flags().String("quota", "", "Resource quota preset (small|medium|large)")
	cmd.Flags().String("default-mode", "", "Default permission mode for new tentacles (e.g. rwxrwx---)")
	return cmd
}

func runEnclaveProvision(cmd *cobra.Command, args []string) error {
	name := args[0]
	outputFormat := flagString(cmd, "output")

	owner, _ := cmd.Flags().GetString("owner")
	membersRaw, _ := cmd.Flags().GetString("members")
	platform, _ := cmd.Flags().GetString("platform")
	channelID, _ := cmd.Flags().GetString("channel-id")
	channelName, _ := cmd.Flags().GetString("channel-name")
	quota, _ := cmd.Flags().GetString("quota")
	defaultMode, _ := cmd.Flags().GetString("default-mode")

	var members []string
	if membersRaw != "" {
		for _, m := range strings.Split(membersRaw, ",") {
			if trimmed := strings.TrimSpace(m); trimmed != "" {
				members = append(members, trimmed)
			}
		}
	}

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	params := mcp.EnclaveProvisionParams{
		Name:        name,
		Owner:       owner,
		Members:     members,
		Platform:    platform,
		ChannelID:   channelID,
		ChannelName: channelName,
		Quota:       quota,
		DefaultMode: defaultMode,
	}

	result, err := mcpClient.EnclaveProvision(cmd.Context(), params)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("provisioning enclave: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("provisioning enclave: %w", err)
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
	fmt.Fprintf(out, "Enclave %s (status: %s)\n", result.Name, result.Status)
	fmt.Fprintf(out, "  Owner:      %s\n", result.Owner)
	if len(result.Members) > 0 {
		fmt.Fprintf(out, "  Members:    %s\n", strings.Join(result.Members, ", "))
	}
	if result.QuotaPreset != "" {
		fmt.Fprintf(out, "  Quota:      %s\n", result.QuotaPreset)
	}
	return nil
}

// --- enclave sync ---

func newEnclaveSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync <name>",
		Short: "Update enclave membership or metadata",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnclaveSync,
	}
	cmd.Flags().String("add-members", "", "Comma-separated list of member emails to add")
	cmd.Flags().String("remove-members", "", "Comma-separated list of member emails to remove")
	cmd.Flags().String("new-owner", "", "Transfer ownership to this email")
	cmd.Flags().String("channel-name", "", "Update the platform channel name")
	cmd.Flags().String("status", "", "Set enclave status (active|frozen)")
	cmd.Flags().String("quota", "", "Update resource quota preset (small|medium|large)")
	return cmd
}

func runEnclaveSync(cmd *cobra.Command, args []string) error {
	name := args[0]
	outputFormat := flagString(cmd, "output")

	addRaw, _ := cmd.Flags().GetString("add-members")
	removeRaw, _ := cmd.Flags().GetString("remove-members")
	newOwner, _ := cmd.Flags().GetString("new-owner")
	channelName, _ := cmd.Flags().GetString("channel-name")
	status, _ := cmd.Flags().GetString("status")
	quota, _ := cmd.Flags().GetString("quota")

	if addRaw == "" && removeRaw == "" && newOwner == "" && channelName == "" && status == "" && quota == "" {
		return errors.New("at least one of --add-members, --remove-members, --new-owner, --channel-name, --status, or --quota must be specified")
	}

	addMembers := splitEmails(addRaw)
	removeMembers := splitEmails(removeRaw)

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	params := mcp.EnclaveSyncParams{
		Name:           name,
		AddMembers:     addMembers,
		RemoveMembers:  removeMembers,
		NewOwner:       newOwner,
		ChannelName:    channelName,
		Status:         status,
		NewQuotaPreset: quota,
	}

	result, err := mcpClient.EnclaveSync(cmd.Context(), params)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w\n  hint: only the enclave owner can modify membership", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("syncing enclave: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("syncing enclave: %w", err)
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
	if len(result.Updated) > 0 {
		fmt.Fprintf(out, "Updated enclave %s\n", result.Name)
		for _, field := range result.Updated {
			fmt.Fprintf(out, "  %s\n", field)
		}
	} else {
		fmt.Fprintf(out, "No changes for enclave %s\n", result.Name)
	}
	return nil
}

// --- enclave deprovision ---

func newEnclaveDeprovisionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deprovision <name>",
		Short: "Deprovision and destroy an enclave",
		Args:  cobra.ExactArgs(1),
		RunE:  runEnclaveDeprovision,
	}
	cmd.Flags().Bool("confirm", false, "Required: confirm destructive deprovision operation")
	return cmd
}

func runEnclaveDeprovision(cmd *cobra.Command, args []string) error {
	name := args[0]
	outputFormat := flagString(cmd, "output")

	confirm, _ := cmd.Flags().GetBool("confirm")
	if !confirm {
		return errors.New("enclave deprovision is destructive; re-run with --confirm to proceed")
	}

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	result, err := mcpClient.EnclaveDeprovision(cmd.Context(), name)
	if err != nil {
		if isAuthzError(err) {
			return fmt.Errorf("permission denied: %w\n  hint: only the enclave owner can deprovision", err)
		}
		if hint := mcpErrorHint(err); hint != "" {
			return fmt.Errorf("deprovisioning enclave: %w\n  hint: %s", err, hint)
		}
		return fmt.Errorf("deprovisioning enclave: %w", err)
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
	if result.Deleted {
		fmt.Fprintf(out, "Deprovisioned enclave %s\n", result.Name)
	} else {
		fmt.Fprintf(out, "Enclave %s not found or already removed\n", result.Name)
	}
	return nil
}

// resolveEnclaveNamespace resolves the Kubernetes namespace for a named enclave.
// It calls enclave_info to retrieve the enclave's namespace field.
// If enclaveName is the special value "auto", it calls enclave_list --mine and
// resolves to the namespace of the single enclave the caller belongs to;
// if there are zero or more than one, it returns an error.
func resolveEnclaveNamespace(cmd *cobra.Command, mcpClient *mcp.Client, enclaveName string) (string, error) {
	if enclaveName == "auto" {
		return resolveAutoEnclave(cmd, mcpClient)
	}
	info, err := mcpClient.EnclaveInfo(cmd.Context(), enclaveName)
	if err != nil {
		if isAuthzError(err) {
			return "", fmt.Errorf("permission denied accessing enclave %q: %w", enclaveName, err)
		}
		return "", fmt.Errorf("resolving enclave %q: %w", enclaveName, err)
	}
	// The namespace name is always the enclave name.
	return info.Name, nil
}

// resolveAutoEnclave lists the caller's enclaves and returns the namespace if exactly one.
func resolveAutoEnclave(cmd *cobra.Command, mcpClient *mcp.Client) (string, error) {
	items, err := mcpClient.EnclaveList(cmd.Context(), "")
	if err != nil {
		return "", fmt.Errorf("auto-resolving enclave: %w", err)
	}
	if len(items) == 0 {
		return "", errors.New("--enclave auto: you are not a member of any enclave")
	}
	if len(items) > 1 {
		names := make([]string, len(items))
		for i, it := range items {
			names[i] = it.Name
		}
		return "", fmt.Errorf("--enclave auto: you belong to multiple enclaves (%s); specify one with --enclave <name>",
			strings.Join(names, ", "))
	}
	// The namespace name is always the enclave name.
	return items[0].Name, nil
}

// splitEmails splits a comma-separated email string into a slice, trimming whitespace.
// Returns nil if the input is empty.
func splitEmails(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, e := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(e); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// resolveCallerEmail attempts to determine the caller's email for --mine filtering.
// Returns the email string if resolved, empty string otherwise (server falls back
// to the token identity for filtering).
func resolveCallerEmail(_ *cobra.Command, _ *mcp.Client) string {
	// Email resolution from token identity is handled server-side when
	// caller_email is empty. This function is a hook for future client-side
	// resolution (e.g., from a cached OIDC token claims file).
	return ""
}
