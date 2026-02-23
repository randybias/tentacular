package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randybias/tentacular/pkg/k8s"
	"github.com/spf13/cobra"
)

const profileFreshnessThreshold = time.Hour

// resolveProfileDir returns the envprofiles directory alongside whichever
// .tentacular/config.yaml is active: project-level (CWD) takes priority,
// falling back to user-level (~/.tentacular/).
func resolveProfileDir() string {
	if _, err := os.Stat(filepath.Join(".tentacular", "config.yaml")); err == nil {
		return filepath.Join(".tentacular", "envprofiles")
	}
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
	cmd.Flags().String("env", "", "Environment name from .tentacular/config.yaml")
	cmd.Flags().Bool("all", false, "Profile all configured environments")
	cmd.Flags().String("output", "markdown", "Output format: markdown|json")
	cmd.Flags().Bool("save", false, "Write profiles to .tentacular/envprofiles/")
	cmd.Flags().Bool("force", false, "Rebuild even if a fresh profile exists (< 1h old)")
	return cmd
}

func runProfile(cmd *cobra.Command, args []string) error {
	envName, _ := cmd.Flags().GetString("env")
	all, _ := cmd.Flags().GetBool("all")
	output, _ := cmd.Flags().GetString("output")
	save, _ := cmd.Flags().GetBool("save")
	force, _ := cmd.Flags().GetBool("force")

	if all {
		return runProfileAll(output, save, force)
	}
	return runProfileForEnv(envName, output, save, force)
}

func runProfileAll(output string, save, force bool) error {
	cfg := LoadConfig()
	if len(cfg.Environments) == 0 {
		return fmt.Errorf("no environments configured in .tentacular/config.yaml")
	}

	var errs []string
	for envName := range cfg.Environments {
		fmt.Printf("Profiling environment %q...\n", envName)
		if err := runProfileForEnv(envName, output, save, force); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ %s: %s\n", envName, err)
			errs = append(errs, envName)
		}
	}
	if len(errs) == len(cfg.Environments) {
		return fmt.Errorf("all environments failed to profile")
	}
	return nil
}

func runProfileForEnv(envName, output string, save, force bool) error {
	// Freshness check
	if save && !force && envName != "" {
		mdPath := filepath.Join(resolveProfileDir(), envName+".md")
		if fi, err := os.Stat(mdPath); err == nil {
			age := time.Since(fi.ModTime())
			if age < profileFreshnessThreshold {
				fmt.Printf("  Profile for %q is fresh (generated %s ago). Use --force to rebuild.\n",
					envName, age.Truncate(time.Minute))
				return nil
			}
		}
	}

	env, err := ResolveEnvironment(envName)
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

	client, err := buildClientForEnv(env)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	label := envName
	if label == "" {
		label = "default"
	}

	profile, err := client.Profile(ctx, namespace, label)
	if err != nil {
		return fmt.Errorf("profiling cluster: %w", err)
	}

	// Render
	var rendered string
	switch output {
	case "json":
		rendered = profile.JSON()
	default:
		rendered = profile.Markdown()
	}

	if save {
		dir := resolveProfileDir()
		if err := saveProfile(profile, label, dir); err != nil {
			return fmt.Errorf("saving profile: %w", err)
		}
		fmt.Printf("  ✓ Profile for %q saved to %s/%s.{md,json}\n", label, dir, label)
	} else {
		fmt.Println(rendered)
	}

	return nil
}

// saveProfile writes both markdown and JSON representations to dir.
func saveProfile(p *k8s.ClusterProfile, envName, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating profile directory: %w", err)
	}

	mdPath := filepath.Join(dir, envName+".md")
	if err := os.WriteFile(mdPath, []byte(p.Markdown()), 0o644); err != nil {
		return fmt.Errorf("writing markdown profile: %w", err)
	}

	jsonPath := filepath.Join(dir, envName+".json")
	if err := os.WriteFile(jsonPath, []byte(p.JSON()), 0o644); err != nil {
		return fmt.Errorf("writing JSON profile: %w", err)
	}

	return nil
}

// buildClientForEnv creates a k8s.Client for the given environment config.
func buildClientForEnv(env *EnvironmentConfig) (*k8s.Client, error) {
	if env.Kubeconfig != "" {
		return k8s.NewClientFromConfig(env.Kubeconfig, env.Context)
	}
	if env.Context != "" {
		return k8s.NewClientWithContext(env.Context)
	}
	return k8s.NewClient()
}

// AutoProfileEnvironments generates profiles for all reachable environments.
// Called from configure after writing config. Errors are warnings only.
func AutoProfileEnvironments() {
	cfg := LoadConfig()
	if len(cfg.Environments) == 0 {
		return
	}
	for envName := range cfg.Environments {
		fmt.Printf("Profiling environment %q... ", envName)
		if err := runProfileForEnv(envName, "markdown", true, false); err != nil {
			fmt.Printf("⚠ skipped (%s)\n", err)
		}
	}
}
