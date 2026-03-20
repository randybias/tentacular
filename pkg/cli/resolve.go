package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/mcp"
)

// flagString returns the value of a named flag, traversing parent commands for persistent flags.
// Returns "" if the flag is not found or not set.
func flagString(cmd *cobra.Command, name string) string {
	if f := cmd.Flag(name); f != nil {
		return f.Value.String()
	}
	return ""
}

// resolveNamespace determines the target namespace using the cascade:
// 1. deployment.namespace from workflow.yaml (when workflowDir is provided)
// 2. env config namespace (from --env or default environment)
// 3. global config namespace
// 4. "default"
func resolveNamespace(cmd *cobra.Command, workflowDir string) string {
	envName := flagString(cmd, "env")

	// Try workflow.yaml deployment.namespace first
	if workflowDir != "" {
		if ns := readWorkflowNamespace(workflowDir); ns != "" {
			return ns
		}
	}

	// Try env config namespace
	env, err := ResolveEnvironment(envName)
	if err == nil && env.Namespace != "" {
		return env.Namespace
	}

	// Try global config namespace
	cfg := LoadConfig()
	if cfg.Namespace != "" {
		return cfg.Namespace
	}

	return "default"
}

// readWorkflowNamespace reads deployment.namespace from workflow.yaml without full validation.
func readWorkflowNamespace(workflowDir string) string {
	specPath := filepath.Join(workflowDir, "workflow.yaml")
	data, err := os.ReadFile(specPath) //nolint:gosec // specPath is derived from workflow directory
	if err != nil {
		return ""
	}
	// Lenient parse — just extract deployment.namespace
	var stub struct {
		Deployment struct {
			Namespace string `yaml:"namespace"`
		} `yaml:"deployment"`
	}
	if err := yaml.Unmarshal(data, &stub); err != nil {
		return ""
	}
	return stub.Deployment.Namespace
}

// resolveMCPClient attempts to create an MCP client using the per-env resolution cascade:
// 1. OIDC token (from `tntc login`) -- preferred when available and not expired
// 2. Active environment's mcp_endpoint / mcp_token_path (--env > TENTACULAR_ENV > default_env)
// 3. Global mcp.endpoint / mcp.token_path from config files
// 4. TNTC_MCP_ENDPOINT / TNTC_MCP_TOKEN environment variables
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

	// Resolve the MCP endpoint first (needed regardless of auth method)
	endpoint := resolveEndpoint(envName, cfg)
	if endpoint == "" {
		return nil, nil
	}

	// Try OIDC token first (from `tntc login`)
	oidcToken, err := resolveOIDCToken(envName)
	if err != nil {
		// Non-fatal: fall through to static token
		fmt.Fprintf(os.Stderr, "Warning: OIDC token error: %v\n", err)
	}
	if oidcToken != "" {
		return mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: oidcToken}), nil
	}

	// Fall back to static bearer token
	token, err := resolveStaticToken(envName, cfg)
	if err != nil {
		return nil, err
	}
	return mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: token}), nil
}

// resolveEndpoint determines the MCP endpoint from env config, global config, or env vars.
func resolveEndpoint(envName string, cfg TentacularConfig) string {
	if envName != "" {
		if env, ok := cfg.Environments[envName]; ok && env.MCPEndpoint != "" {
			return env.MCPEndpoint
		}
	}
	if cfg.MCP.Endpoint != "" {
		return cfg.MCP.Endpoint
	}
	if v := os.Getenv("TNTC_MCP_ENDPOINT"); v != "" {
		return v
	}
	return ""
}

// resolveOIDCToken checks for a valid OIDC token, refreshing if needed.
// Returns "" if no OIDC token is available (not an error -- falls back to static).
func resolveOIDCToken(envName string) (string, error) {
	tokenEnv := envName
	if tokenEnv == "" {
		tokenEnv = "default"
	}

	store, err := LoadOIDCToken(tokenEnv)
	if err != nil {
		return "", err
	}
	if store == nil {
		return "", nil
	}

	if !store.IsExpired() {
		return store.AccessToken, nil
	}

	// Token expired -- try refresh
	if store.RefreshToken == "" {
		return "", errors.New("OIDC token expired and no refresh token available; run 'tntc login'")
	}

	refreshed, err := RefreshOIDCToken(tokenEnv, store)
	if err != nil {
		return "", fmt.Errorf("OIDC token refresh failed: %w; run 'tntc login' to re-authenticate", err)
	}
	return refreshed.AccessToken, nil
}

// resolveStaticToken resolves a static bearer token from env config, global config, or env vars.
func resolveStaticToken(envName string, cfg TentacularConfig) (string, error) {
	if envName != "" {
		if env, ok := cfg.Environments[envName]; ok {
			tokenPath := env.MCPTokenPath
			if tokenPath != "" {
				tokenPath = expandHome(tokenPath)
				token, err := readTokenFile(tokenPath)
				if err != nil {
					return "", fmt.Errorf("reading MCP token for env %q: %w", envName, err)
				}
				return token, nil
			}
		}
	}

	// Global config token path
	if cfg.MCP.TokenPath != "" {
		token, err := readTokenFile(expandHome(cfg.MCP.TokenPath))
		if err != nil {
			return "", fmt.Errorf("reading MCP token: %w", err)
		}
		return token, nil
	}

	// Environment variable
	if v := os.Getenv("TNTC_MCP_TOKEN"); v != "" {
		return v, nil
	}

	return "", nil
}

// readTokenFile reads a bearer token from a file, trimming whitespace.
func readTokenFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a known config file
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
		return nil, errors.New("MCP server not configured; set mcp_endpoint in your environment config or TNTC_MCP_ENDPOINT")
	}
	return client, nil
}

// buildMCPClientForEnv creates an MCP client for a named environment,
// using OIDC token (preferred), per-env mcp_token_path, or global config fallback.
func buildMCPClientForEnv(envName string) (*mcp.Client, error) {
	cfg := LoadConfig()

	endpoint := resolveEndpoint(envName, cfg)
	if endpoint == "" {
		return nil, fmt.Errorf("no MCP endpoint configured for environment %q; add mcp_endpoint to your config", envName)
	}

	// Try OIDC token first
	oidcToken, _ := resolveOIDCToken(envName)
	if oidcToken != "" {
		return mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: oidcToken}), nil
	}

	// Fall back to static token
	token, err := resolveStaticToken(envName, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolving token for env %q: %w", envName, err)
	}

	return mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: token}), nil
}

// mcpErrorHint returns a user-friendly hint for common MCP errors.
func mcpErrorHint(err error) string {
	if mcp.IsServerUnavailable(err) {
		return "MCP server unreachable; check with: kubectl get deploy -n tentacular-system"
	}
	if mcp.IsUnauthorized(err) {
		return "MCP authentication failed; try 'tntc login' or check your mcp_token_path / TNTC_MCP_TOKEN"
	}
	if mcp.IsForbidden(err) {
		return "MCP namespace guard rejected request; check namespace permissions"
	}
	return ""
}

// isAuthzError returns true if err is a permission denial from the authz layer (HTTP 403).
func isAuthzError(err error) bool {
	return mcp.IsForbidden(err)
}
