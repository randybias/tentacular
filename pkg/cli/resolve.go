package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/randybias/tentacular/pkg/mcp"
	"github.com/spf13/cobra"
)

// flagString returns the value of a named flag, traversing parent commands for persistent flags.
// Returns "" if the flag is not found or not set.
func flagString(cmd *cobra.Command, name string) string {
	if f := cmd.Flag(name); f != nil {
		return f.Value.String()
	}
	return ""
}

// resolveMCPClient attempts to create an MCP client using the per-env resolution cascade:
// 1. Active environment's mcp_endpoint / mcp_token_path (--env > TENTACULAR_ENV > default_env)
// 2. Global mcp.endpoint / mcp.token_path from config files
// 3. TNTC_MCP_ENDPOINT / TNTC_MCP_TOKEN environment variables
//
// Returns (client, nil) if MCP is configured and available.
// Returns (nil, nil) if no MCP configuration is found.
// Returns (nil, err) if configuration is invalid.
func resolveMCPClient(cmd *cobra.Command) (*mcp.Client, error) {
	// Determine active environment name
	envName := ""
	if cmd != nil {
		if f := cmd.Flag("env"); f != nil {
			envName = f.Value.String()
		}
	}
	if envName == "" {
		envName = os.Getenv("TENTACULAR_ENV")
	}
	cfg := LoadConfig()
	if envName == "" {
		envName = cfg.DefaultEnv
	}

	// Try per-environment MCP config first
	if envName != "" {
		if env, ok := cfg.Environments[envName]; ok {
			if env.MCPEndpoint != "" {
				tokenPath := env.MCPTokenPath
				if tokenPath != "" {
					tokenPath = expandHome(tokenPath)
				}
				mcpCfg := mcp.Config{Endpoint: env.MCPEndpoint, TokenPath: tokenPath}
				if mcpCfg.TokenPath != "" {
					token, err := readTokenFile(mcpCfg.TokenPath)
					if err != nil {
						return nil, fmt.Errorf("reading MCP token for env %q: %w", envName, err)
					}
					mcpCfg.Token = token
				}
				return mcp.NewClient(mcpCfg), nil
			}
		}
	}

	// Fall back to global config + env vars
	mcpCfg := mcp.Config{}
	if cfg.MCP.Endpoint != "" {
		mcpCfg.Endpoint = cfg.MCP.Endpoint
		mcpCfg.TokenPath = cfg.MCP.TokenPath
	}
	if v := os.Getenv("TNTC_MCP_ENDPOINT"); v != "" {
		mcpCfg.Endpoint = v
	}
	if v := os.Getenv("TNTC_MCP_TOKEN"); v != "" {
		mcpCfg.Token = v
	}
	if mcpCfg.Endpoint == "" {
		return nil, nil
	}
	if mcpCfg.Token == "" && mcpCfg.TokenPath != "" {
		token, err := readTokenFile(expandHome(mcpCfg.TokenPath))
		if err != nil {
			return nil, fmt.Errorf("reading MCP token: %w", err)
		}
		mcpCfg.Token = token
	}
	return mcp.NewClient(mcpCfg), nil
}

// readTokenFile reads a bearer token from a file, trimming whitespace.
func readTokenFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// requireMCPClient is like resolveMCPClient but returns an error if MCP is not configured.
// Use this for commands that require MCP.
func requireMCPClient(cmd *cobra.Command) (*mcp.Client, error) {
	client, err := resolveMCPClient(cmd)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("MCP server not configured; set mcp_endpoint in your environment config or TNTC_MCP_ENDPOINT")
	}
	return client, nil
}

// buildMCPClientForEnv creates an MCP client for a named environment,
// using per-env mcp_endpoint / mcp_token_path with fallback to global config.
func buildMCPClientForEnv(envName string) (*mcp.Client, error) {
	cfg := LoadConfig()

	var endpoint, tokenPath string
	if env, ok := cfg.Environments[envName]; ok && env.MCPEndpoint != "" {
		endpoint = env.MCPEndpoint
		tokenPath = expandHome(env.MCPTokenPath)
	} else {
		endpoint = cfg.MCP.Endpoint
		tokenPath = expandHome(cfg.MCP.TokenPath)
	}

	if endpoint == "" {
		return nil, fmt.Errorf("no MCP endpoint configured for environment %q; add mcp_endpoint to your config", envName)
	}

	token := ""
	if tokenPath != "" {
		t, err := readTokenFile(tokenPath)
		if err != nil {
			return nil, fmt.Errorf("reading token for env %q: %w", envName, err)
		}
		token = t
	}

	return mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: token}), nil
}

// mcpErrorHint returns a user-friendly hint for common MCP errors.
func mcpErrorHint(err error) string {
	if mcp.IsServerUnavailable(err) {
		return "MCP server unreachable; check with: kubectl get deploy -n tentacular-system"
	}
	if mcp.IsUnauthorized(err) {
		return "MCP authentication failed; check your mcp_token_path or TNTC_MCP_TOKEN"
	}
	if mcp.IsForbidden(err) {
		return "MCP namespace guard rejected request; check namespace permissions"
	}
	return ""
}
