package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/randybias/tentacular/pkg/builder"
	"github.com/randybias/tentacular/pkg/spec"
	"github.com/spf13/cobra"
)

func NewBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [dir]",
		Short: "Build container image",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runBuild,
	}
	cmd.Flags().StringP("tag", "t", "", "Image tag (default: tentacular-engine:latest)")
	cmd.Flags().StringP("registry", "r", "", "Container registry URL")
	cmd.Flags().Bool("push", false, "Push image to registry after build")
	cmd.Flags().String("platform", "", "Target platform (e.g., linux/arm64)")
	return cmd
}

func runBuild(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	specPath := filepath.Join(absDir, "workflow.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading workflow spec: %w", err)
	}

	_, errs := spec.Parse(data)
	if len(errs) > 0 {
		return fmt.Errorf("workflow spec has %d validation error(s)", len(errs))
	}

	registry, _ := cmd.Flags().GetString("registry")
	tag, _ := cmd.Flags().GetString("tag")
	push, _ := cmd.Flags().GetBool("push")
	platform, _ := cmd.Flags().GetString("platform")

	// Apply config default for registry
	if !cmd.Flags().Changed("registry") {
		cfg := LoadConfig()
		if cfg.Registry != "" {
			registry = cfg.Registry
		}
	}

	if tag == "" {
		tag = "tentacular-engine:latest"
	}
	if registry != "" {
		tag = registry + "/" + tag
	}

	// Default to linux/arm64 when pushing (target cluster is ARM64)
	if push && platform == "" {
		platform = "linux/arm64"
	}

	engineDir := findEngineDir()
	if engineDir == "" {
		return fmt.Errorf("cannot find engine directory")
	}

	// Generate Dockerfile
	dockerfile := builder.GenerateDockerfile()
	dockerfilePath := filepath.Join(absDir, "Dockerfile.tentacular")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}
	defer os.Remove(dockerfilePath)

	// Copy engine into build context
	engineDest := filepath.Join(absDir, ".engine")
	if err := copyDir(engineDir, engineDest); err != nil {
		return fmt.Errorf("copying engine: %w", err)
	}
	defer os.RemoveAll(engineDest)

	fmt.Printf("Building image %s...\n", tag)

	buildArgs := []string{"build", "-f", dockerfilePath, "-t", tag}
	if platform != "" {
		buildArgs = append(buildArgs, "--platform", platform)
	}
	buildArgs = append(buildArgs, absDir)

	buildCmd := exec.Command("docker", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	fmt.Printf("✓ Built image: %s\n", tag)

	// Push if requested
	if push {
		if registry == "" {
			return fmt.Errorf("--push requires --registry (-r) to be set")
		}
		fmt.Printf("Pushing image %s...\n", tag)
		pushCmd := exec.Command("docker", "push", tag)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("docker push failed: %w", err)
		}
		fmt.Printf("✓ Pushed image: %s\n", tag)
	}

	// Save image tag to .tentacular/base-image.txt (relative to workflow directory)
	tentacularDir := filepath.Join(absDir, ".tentacular")
	if err := os.MkdirAll(tentacularDir, 0o755); err != nil {
		return fmt.Errorf("creating .tentacular directory: %w", err)
	}
	tagFilePath := filepath.Join(tentacularDir, "base-image.txt")
	if err := os.WriteFile(tagFilePath, []byte(tag), 0o644); err != nil {
		return fmt.Errorf("writing base-image.txt: %w", err)
	}

	return nil
}

func copyDir(src, dst string) error {
	cmd := exec.Command("cp", "-r", src, dst)
	return cmd.Run()
}
