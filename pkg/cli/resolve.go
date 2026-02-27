package cli

import (
	"context"
	"fmt"

	"github.com/randybias/tentacular/pkg/mcp"
	"github.com/spf13/cobra"
)

// resolveMCPClient attempts to create an MCP client from the current config.
// Returns (client, nil) if MCP is configured and available.
// Returns (nil, nil) if no MCP configuration is found (pre-bootstrap).
// Returns (nil, err) if MCP is configured but connection fails.
func resolveMCPClient(cmd *cobra.Command) (*mcp.Client, error) {
	// LoadConfigFromCluster checks config files and env vars in priority order.
	mcpCfg, err := mcp.LoadConfigFromCluster(context.Background())
	if err != nil {
		return nil, fmt.Errorf("MCP configuration error: %w", err)
	}
	if mcpCfg == nil {
		return nil, nil
	}
	return mcp.NewClient(*mcpCfg), nil
}

// requireMCPClient is like resolveMCPClient but returns an error if MCP is not configured.
// Use this for commands that require MCP after cluster bootstrap.
func requireMCPClient(cmd *cobra.Command) (*mcp.Client, error) {
	client, err := resolveMCPClient(cmd)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("MCP server not configured; run `tntc cluster install` first")
	}
	return client, nil
}

// mcpErrorHint returns a user-friendly hint for common MCP errors.
func mcpErrorHint(err error) string {
	if mcp.IsServerUnavailable(err) {
		return "MCP server unreachable; check with: kubectl get deploy -n tentacular-system"
	}
	if mcp.IsUnauthorized(err) {
		return "MCP authentication failed; regenerate token with: tntc cluster install --rotate-token"
	}
	if mcp.IsForbidden(err) {
		return "MCP namespace guard rejected request; check namespace permissions"
	}
	return ""
}
