package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	clusterName := flagString(cmd, "cluster")

	// Try workflow.yaml deployment.namespace first
	if workflowDir != "" {
		if ns := readWorkflowNamespace(workflowDir); ns != "" {
			return ns
		}
	}

	// Try env config namespace
	env, err := ResolveEnvironment(clusterName)
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
// 1. OIDC token (from `tntc login` or TNTC_ACCESS_TOKEN env var)
// 2. Active cluster's mcp_endpoint (--cluster > TENTACULAR_CLUSTER > default_cluster)
// 3. Global mcp.endpoint from config files
// 4. TNTC_MCP_ENDPOINT environment variable
//
// Returns (client, nil) if MCP is configured and available.
// Returns (nil, nil) if no MCP configuration is found.
// Returns (nil, err) if configuration is invalid.
func resolveMCPClient(cmd *cobra.Command) (*mcp.Client, error) {
	// Determine active environment name
	clusterName := ""
	if cmd != nil {
		if f := cmd.Flag("cluster"); f != nil {
			clusterName = f.Value.String()
		}
	}
	if clusterName == "" {
		clusterName = os.Getenv("TENTACULAR_CLUSTER")
	}
	cfg := LoadConfig()
	if clusterName == "" {
		clusterName = cfg.DefaultCluster
	}

	// Resolve the MCP endpoint first (needed regardless of auth method)
	endpoint := resolveEndpoint(clusterName, cfg)
	if endpoint == "" {
		return nil, nil
	}

	// Try OIDC token first (from `tntc login`)
	oidcToken, err := resolveOIDCToken(clusterName)
	if err != nil {
		// OIDC was configured but failed (expired, refresh error, etc.).
		// Do NOT fall back to bearer token — that would silently escalate
		// to superuser privileges. Return a hard error instead.
		return nil, fmt.Errorf("OIDC token expired. Run `tntc login` to re-authenticate (detail: %w)", err)
	}
	if oidcToken != "" {
		return mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: oidcToken}), nil
	}

	// No OIDC token available — connect without auth (server may reject)
	return mcp.NewClient(mcp.Config{Endpoint: endpoint}), nil
}

// resolveEndpoint determines the MCP endpoint from env config, global config, or env vars.
func resolveEndpoint(clusterName string, cfg TentacularConfig) string {
	if clusterName != "" {
		if env, ok := cfg.Clusters[clusterName]; ok && env.MCPEndpoint != "" {
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
//
// Resolution order:
//  1. TNTC_ACCESS_TOKEN env var (transitive trust: orchestrators inject the user's token)
//  2. Cached token on disk (~/.tentacular/tokens/<env>.json)
//  3. Refresh expired cached token using refresh_token
func resolveOIDCToken(clusterName string) (string, error) {
	// 1. Environment variable override (transitive trust for multi-tenant orchestrators).
	// When The Kraken or another orchestrator acts on behalf of a user, it injects
	// the user's OIDC access token here. This skips device flow entirely.
	if injected := os.Getenv("TNTC_ACCESS_TOKEN"); injected != "" {
		// Validate the token isn't expired before using it.
		claims, err := DecodeJWTClaims(injected)
		if err != nil {
			return "", fmt.Errorf("TNTC_ACCESS_TOKEN is not a valid JWT: %w", err)
		}
		if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
			// Token expired -- try refresh via env var if available.
			if refreshToken := os.Getenv("TNTC_REFRESH_TOKEN"); refreshToken != "" {
				refreshed, refreshErr := refreshWithToken(clusterName, injected, refreshToken)
				if refreshErr != nil {
					return "", fmt.Errorf("TNTC_ACCESS_TOKEN expired and refresh failed: %w", refreshErr)
				}
				return refreshed, nil
			}
			return "", errors.New("TNTC_ACCESS_TOKEN expired and no TNTC_REFRESH_TOKEN provided")
		}
		return injected, nil
	}

	// 2. Cached token on disk.
	tokenEnv := clusterName
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

	// 3. Token expired -- try refresh.
	if store.RefreshToken == "" {
		return "", errors.New("OIDC token expired and no refresh token available; run 'tntc login'")
	}

	refreshed, err := RefreshOIDCToken(tokenEnv, store)
	if err != nil {
		return "", fmt.Errorf("OIDC token refresh failed: %w; run 'tntc login' to re-authenticate", err)
	}
	return refreshed.AccessToken, nil
}

// refreshWithToken attempts to refresh an expired access token using the provided refresh token.
// Used when tokens are injected via environment variables (transitive trust).
func refreshWithToken(clusterName, accessToken, refreshToken string) (string, error) {
	tokenEnv := clusterName
	if tokenEnv == "" {
		tokenEnv = "default"
	}

	// Build a synthetic token store for the refresh flow.
	store := &OIDCTokenStore{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	refreshed, err := RefreshOIDCToken(tokenEnv, store)
	if err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
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
// using OIDC token from `tntc login` or TNTC_ACCESS_TOKEN.
func buildMCPClientForEnv(clusterName string) (*mcp.Client, error) {
	cfg := LoadConfig()

	endpoint := resolveEndpoint(clusterName, cfg)
	if endpoint == "" {
		return nil, fmt.Errorf("no MCP endpoint configured for cluster %q; add mcp_endpoint to your config", clusterName)
	}

	// Use OIDC token for authentication
	oidcToken, err := resolveOIDCToken(clusterName)
	if err != nil {
		return nil, fmt.Errorf("OIDC token expired. Run `tntc login` to re-authenticate (detail: %w)", err)
	}
	if oidcToken != "" {
		return mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: oidcToken}), nil
	}

	// No OIDC token available — connect without auth (server may reject)
	return mcp.NewClient(mcp.Config{Endpoint: endpoint}), nil
}

// mcpErrorHint returns a user-friendly hint for common MCP errors.
func mcpErrorHint(err error) string {
	if mcp.IsServerUnavailable(err) {
		return "MCP server unreachable; check with: kubectl get deploy -n tentacular-system"
	}
	if mcp.IsUnauthorized(err) {
		return "MCP authentication failed; run 'tntc login' to re-authenticate"
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
