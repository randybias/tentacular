package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigureCmd creates the "configure" subcommand.
func NewConfigureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configure",
		Short: "Set default configuration",
		Long: `Set default configuration values for tntc.

Without --cluster, sets top-level defaults (registry, namespace, runtime-class).
With --cluster, creates or updates a cluster with OIDC, MCP, and Kubernetes settings.

Examples:
  # Set top-level defaults
  tntc configure --registry ghcr.io/myorg --default-namespace myapp

  # Configure a cluster with OIDC SSO
  tntc configure -c staging --oidc-issuer https://auth.example.com/realms/dev \
    --oidc-client-id myclient --mcp-endpoint https://mcp.example.com

  # Interactive SSO setup
  tntc configure --sso -c staging

  # Set the default cluster
  tntc configure --default-cluster staging`,
		RunE: runConfigure,
	}

	// Top-level flags
	cmd.Flags().String("registry", "", "Default container registry URL")
	cmd.Flags().String("default-namespace", "", "Default Kubernetes namespace")
	cmd.Flags().String("runtime-class", "", "Default RuntimeClass name")
	cmd.Flags().String("default-cluster", "", "Set the default cluster name")
	cmd.Flags().Bool("project", false, "Write to project config (.tentacular/config.yaml) instead of user config")

	// Cluster-scoped flags (require --cluster)
	cmd.Flags().String("oidc-issuer", "", "OIDC issuer URL (e.g. Keycloak realm URL)")
	cmd.Flags().String("oidc-client-id", "", "OIDC client ID")
	cmd.Flags().String("oidc-client-secret", "", "OIDC client secret")
	cmd.Flags().String("mcp-endpoint", "", "MCP server endpoint URL")
	cmd.Flags().String("context", "", "Kubernetes context name")

	// SSO guided setup
	cmd.Flags().Bool("sso", false, "Interactive SSO/OIDC setup (prompts for missing values)")

	return cmd
}

func runConfigure(cmd *cobra.Command, args []string) error {
	project, _ := cmd.Flags().GetBool("project")
	clusterName := flagString(cmd, "cluster")
	sso, _ := cmd.Flags().GetBool("sso")

	// Determine config file path
	var configPath string
	if project {
		configPath = filepath.Join(".tentacular", "config.yaml")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("determining home directory: %w", err)
		}
		configPath = filepath.Join(home, ".tentacular", "config.yaml")
	}

	// Load existing config
	cfg := TentacularConfig{}
	if data, err := os.ReadFile(configPath); err == nil { //nolint:gosec // reading config file
		_ = yaml.Unmarshal(data, &cfg)
	}

	// Validate: cluster-scoped flags require --cluster
	clusterScopedFlags := []string{"oidc-issuer", "oidc-client-id", "oidc-client-secret", "mcp-endpoint", "context"}
	if clusterName == "" && !sso {
		for _, f := range clusterScopedFlags {
			if cmd.Flags().Changed(f) {
				return fmt.Errorf("--%s requires --cluster/-c to specify which cluster to configure", f)
			}
		}
	}

	// Validate: --sso requires --cluster
	if sso && clusterName == "" {
		return errors.New("--sso requires --cluster/-c to specify which cluster to configure")
	}

	// Apply top-level flag overrides
	if cmd.Flags().Changed("registry") {
		cfg.Registry, _ = cmd.Flags().GetString("registry")
	}
	if cmd.Flags().Changed("default-namespace") {
		cfg.Namespace, _ = cmd.Flags().GetString("default-namespace")
	}
	if cmd.Flags().Changed("runtime-class") {
		cfg.RuntimeClass, _ = cmd.Flags().GetString("runtime-class")
	}
	if cmd.Flags().Changed("default-cluster") {
		cfg.DefaultCluster, _ = cmd.Flags().GetString("default-cluster")
	}

	// Environment-scoped configuration
	if clusterName != "" {
		if cfg.Clusters == nil {
			cfg.Clusters = make(map[string]EnvironmentConfig)
		}
		env := cfg.Clusters[clusterName] // zero value if new

		// Apply env-scoped flags
		if cmd.Flags().Changed("oidc-issuer") {
			env.OIDCIssuer, _ = cmd.Flags().GetString("oidc-issuer")
		}
		if cmd.Flags().Changed("oidc-client-id") {
			env.OIDCClientID, _ = cmd.Flags().GetString("oidc-client-id")
		}
		if cmd.Flags().Changed("oidc-client-secret") {
			env.OIDCClientSecret, _ = cmd.Flags().GetString("oidc-client-secret")
		}
		if cmd.Flags().Changed("mcp-endpoint") {
			env.MCPEndpoint, _ = cmd.Flags().GetString("mcp-endpoint")
		}
		if cmd.Flags().Changed("context") {
			env.Context, _ = cmd.Flags().GetString("context")
		}

		// --sso guided flow: prompt for missing OIDC fields
		if sso {
			var err error
			env, err = ssoGuidedSetup(cmd, env)
			if err != nil {
				return err
			}
		}

		cfg.Clusters[clusterName] = env
	}

	// Write config
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil { //nolint:gosec // non-sensitive config directory
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Determine file permissions: 0o600 if any env has a client secret
	perm := os.FileMode(0o644)
	if configHasSecret(&cfg) {
		perm = 0o600
		fmt.Fprintln(cmd.OutOrStderr(), "Warning: config contains oidc_client_secret; writing with restricted permissions (0600)")
	}

	if err := os.WriteFile(configPath, data, perm); err != nil { //nolint:gosec // perm is 0o644 or 0o600 depending on secret presence
		return fmt.Errorf("writing config: %w", err)
	}

	// Also tighten existing file if permissions are too open
	if perm == 0o600 {
		_ = os.Chmod(configPath, 0o600)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Configuration saved to %s\n", configPath)
	if cfg.Registry != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  registry: %s\n", cfg.Registry)
	}
	if cfg.Namespace != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  namespace: %s\n", cfg.Namespace)
	}
	if cfg.RuntimeClass != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  runtime_class: %s\n", cfg.RuntimeClass)
	}
	if cfg.DefaultCluster != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  default_cluster: %s\n", cfg.DefaultCluster)
	}
	if clusterName != "" {
		env := cfg.Clusters[clusterName]
		fmt.Fprintf(cmd.OutOrStdout(), "  environment %q:\n", clusterName)
		if env.OIDCIssuer != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "    oidc_issuer: %s\n", env.OIDCIssuer)
		}
		if env.OIDCClientID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "    oidc_client_id: %s\n", env.OIDCClientID)
		}
		if env.OIDCClientSecret != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "    oidc_client_secret: ****\n")
		}
		if env.MCPEndpoint != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "    mcp_endpoint: %s\n", env.MCPEndpoint)
		}
		if env.Context != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "    context: %s\n", env.Context)
		}

		// Hint for next step if OIDC was configured
		if env.OIDCIssuer != "" && env.OIDCClientID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext step: run 'tntc login -c %s' to authenticate.\n", clusterName)
		}
	}

	// Auto-profile all configured environments (best-effort; skips unreachable clusters)
	if len(cfg.Clusters) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nGenerating cluster profiles...")
		AutoProfileEnvironments()
	}

	return nil
}

// ssoGuidedSetup prompts interactively for OIDC fields not already set via flags.
// If all required fields are already provided, skips prompts entirely (agent-safe).
func ssoGuidedSetup(cmd *cobra.Command, env EnvironmentConfig) (EnvironmentConfig, error) {
	reader := bufio.NewReader(cmd.InOrStdin())

	if env.OIDCIssuer == "" {
		val, err := promptValue(cmd, reader, "OIDC issuer URL")
		if err != nil {
			return env, err
		}
		if val == "" {
			return env, errors.New("oidc-issuer is required for SSO setup")
		}
		env.OIDCIssuer = val
	}

	if env.OIDCClientID == "" {
		val, err := promptValue(cmd, reader, "OIDC client ID")
		if err != nil {
			return env, err
		}
		if val == "" {
			return env, errors.New("oidc-client-id is required for SSO setup")
		}
		env.OIDCClientID = val
	}

	if env.OIDCClientSecret == "" {
		val, err := promptValue(cmd, reader, "OIDC client secret (optional, press Enter to skip)")
		if err != nil {
			return env, err
		}
		env.OIDCClientSecret = val
	}

	if env.MCPEndpoint == "" {
		val, err := promptValue(cmd, reader, "MCP endpoint URL (optional, press Enter to skip)")
		if err != nil {
			return env, err
		}
		env.MCPEndpoint = val
	}

	return env, nil
}

// promptValue prints a prompt and reads a line from stdin.
func promptValue(cmd *cobra.Command, reader *bufio.Reader, label string) (string, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s: ", label)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// configHasSecret returns true if any environment contains an oidc_client_secret.
func configHasSecret(cfg *TentacularConfig) bool {
	for _, env := range cfg.Clusters {
		if env.OIDCClientSecret != "" {
			return true
		}
	}
	return false
}
