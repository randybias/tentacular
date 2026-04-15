package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/randybias/tentacular/pkg/mcp"
)

const profileFreshnessThreshold = time.Hour

// resolveProfileDir returns the envprofiles directory under ~/.tentacular/.
func resolveProfileDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".tentacular", "envprofiles") // last resort
	}
	return filepath.Join(home, ".tentacular", "envprofiles")
}

// NewProfileCmd creates the "cluster profile" subcommand.
func NewProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Profile a cluster environment's capabilities",
		Long: `Collect a capability snapshot of a target cluster environment.

The profile captures: K8s version and distribution, node topology, RuntimeClasses,
CNI plugin, NetworkPolicy support, StorageClasses, CSI drivers, installed CRD-based
extensions (Istio, cert-manager, Prometheus Operator, etc.), namespace resource quotas,
LimitRanges, and Pod Security Admission posture.

Profiles are stored at .tentacular/envprofiles/<env>.md and <env>.json. AI agents
load the markdown profile as context before designing tentacles for that environment.

Profiles are automatically generated when running 'tntc configure'. Rebuild manually
when the agent detects environment drift (new RuntimeClass, changed CNI, cluster upgrade).`,
		RunE: runProfile,
	}
	cmd.Flags().Bool("all", false, "Profile all configured environments")
	cmd.Flags().String("output", "markdown", "Output format: markdown|json")
	cmd.Flags().Bool("save", false, "Write profiles to .tentacular/envprofiles/")
	cmd.Flags().Bool("force", false, "Rebuild even if a fresh profile exists (< 1h old)")
	return cmd
}

func runProfile(cmd *cobra.Command, args []string) error {
	clusterName := flagString(cmd, "cluster")
	all, _ := cmd.Flags().GetBool("all")
	output, _ := cmd.Flags().GetString("output")
	save, _ := cmd.Flags().GetBool("save")
	force, _ := cmd.Flags().GetBool("force")

	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return err
	}

	profileFn := func(ctx context.Context, namespace string) ([]byte, error) {
		result, err := mcpClient.ClusterProfile(ctx, namespace)
		if err != nil {
			return nil, err
		}
		return result.Raw, nil
	}

	if all {
		return runProfileAll(profileFn, output, save, force)
	}
	return runProfileForEnv(clusterName, output, save, force, profileFn)
}

// clusterProfileFn is the signature for the MCP-based profile function, injectable for tests.
type clusterProfileFn func(ctx context.Context, namespace string) ([]byte, error)

func runProfileAll(profileFn clusterProfileFn, output string, save, force bool) error {
	cfg := LoadConfig()
	if len(cfg.Clusters) == 0 {
		return errors.New("no environments configured in ~/.tentacular/config.yaml")
	}

	var errs []string
	for clusterName := range cfg.Clusters {
		fmt.Printf("Profiling environment %q...\n", clusterName)
		if err := runProfileForEnv(clusterName, output, save, force, profileFn); err != nil {
			fmt.Fprintf(os.Stderr, "  \u26a0 %s: %s\n", clusterName, err)
			errs = append(errs, clusterName)
		}
	}
	if len(errs) == len(cfg.Clusters) {
		return errors.New("all environments failed to profile")
	}
	return nil
}

func runProfileForEnv(clusterName, output string, save, force bool, profileFn clusterProfileFn) error {
	// Freshness check
	if save && !force && clusterName != "" {
		mdPath := filepath.Join(resolveProfileDir(), clusterName+".md")
		if fi, err := os.Stat(mdPath); err == nil {
			age := time.Since(fi.ModTime())
			if age < profileFreshnessThreshold {
				fmt.Printf("  Skipping %q: profile written %s ago (use --force to override)\n",
					clusterName, age.Truncate(time.Minute))
				return nil
			}
		}
	}

	env, err := ResolveEnvironment(clusterName)
	if err != nil {
		return fmt.Errorf("resolving environment: %w", err)
	}

	namespace := env.Namespace
	if namespace == "" {
		cfg := LoadConfig()
		namespace = cfg.Namespace
	}
	if namespace == "" {
		namespace = "default"
	}

	label := clusterName
	if label == "" {
		label = "default"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	rawJSON, err := profileFn(ctx, namespace)
	if err != nil {
		return fmt.Errorf("profiling cluster: %w", err)
	}

	// Render output
	var rendered string
	switch output {
	case "json":
		rendered = string(rawJSON)
	default:
		// Try to unmarshal into ClusterProfile for rich markdown rendering
		var profile k8s.ClusterProfile
		if jsonErr := json.Unmarshal(rawJSON, &profile); jsonErr == nil {
			profile.Environment = label
			rendered = profile.Markdown()
		} else {
			// Fall back to raw JSON if schema doesn't match
			rendered = string(rawJSON)
		}
	}

	if save {
		dir := resolveProfileDir()
		if err := saveProfileRaw(rawJSON, rendered, label, dir); err != nil {
			return fmt.Errorf("saving profile: %w", err)
		}
		fmt.Printf("  \u2713 Profile for %q saved to %s/%s.{md,json}\n", label, dir, label)
	} else {
		fmt.Println(rendered)
	}

	return nil
}

// saveProfileRaw writes both markdown and JSON representations to dir.
func saveProfileRaw(rawJSON []byte, markdown, clusterName, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // non-sensitive directory
		return fmt.Errorf("creating profile directory: %w", err)
	}

	mdPath := filepath.Join(dir, clusterName+".md")
	if err := os.WriteFile(mdPath, []byte(markdown), 0o644); err != nil { //nolint:gosec // non-sensitive file
		return fmt.Errorf("writing markdown profile: %w", err)
	}

	jsonPath := filepath.Join(dir, clusterName+".json")
	if err := os.WriteFile(jsonPath, rawJSON, 0o644); err != nil { //nolint:gosec // non-sensitive profile file
		return fmt.Errorf("writing JSON profile: %w", err)
	}

	return nil
}

// autoProfileTimeout is the maximum time allowed to profile a single environment
// during auto-profiling on configure. Keeps tntc configure responsive.
const autoProfileTimeout = 45 * time.Second

// AutoProfileEnvironments generates profiles for all reachable environments.
// Called from configure after writing config. Each environment is given a
// 45s deadline; unreachable or slow clusters emit a warning and are skipped.
// Environments without an MCP endpoint configured are silently skipped.
func AutoProfileEnvironments() {
	cfg := LoadConfig()
	if len(cfg.Clusters) == 0 {
		return
	}
	for clusterName := range cfg.Clusters {
		fmt.Printf("Profiling environment %q... ", clusterName)

		profileFn, err := buildProfileFnForEnv(clusterName, cfg)
		if err != nil || profileFn == nil {
			fmt.Printf("\u26a0 skipped (MCP not configured for env %q)\n", clusterName)
			continue
		}

		done := make(chan error, 1)
		fn := profileFn
		name := clusterName
		go func() {
			done <- runProfileForEnv(name, "markdown", true, false, fn)
		}()

		select {
		case err := <-done:
			if err != nil {
				fmt.Printf("\u26a0 skipped (%s)\n", err)
			}
		case <-time.After(autoProfileTimeout):
			fmt.Printf("\u26a0 skipped (timed out after %s)\n", autoProfileTimeout)
		}
	}
}

// buildProfileFnForEnv builds a clusterProfileFn for the named environment.
// Returns (nil, nil) if no MCP endpoint is configured for the environment.
func buildProfileFnForEnv(clusterName string, cfg TentacularConfig) (clusterProfileFn, error) {
	env := cfg.Clusters[clusterName]

	endpoint := env.MCPEndpoint
	if endpoint == "" {
		endpoint = cfg.MCP.Endpoint
	}
	if endpoint == "" {
		return nil, nil
	}

	// Use OIDC token for authentication (no static bearer tokens)
	token, err := resolveOIDCToken(clusterName)
	if err != nil {
		return nil, fmt.Errorf("resolving OIDC token for env %q: %w", clusterName, err)
	}

	mcpClient := mcp.NewClient(mcp.Config{Endpoint: endpoint, Token: token})
	return func(ctx context.Context, namespace string) ([]byte, error) {
		result, err := mcpClient.ClusterProfile(ctx, namespace)
		if err != nil {
			return nil, err
		}
		return result.Raw, nil
	}, nil
}
