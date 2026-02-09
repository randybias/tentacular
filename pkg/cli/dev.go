package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

func NewDevCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev [dir]",
		Short: "Run locally with hot-reload",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDev,
	}
	cmd.Flags().IntP("port", "p", 8080, "HTTP server port")
	return cmd
}

func runDev(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	port, _ := cmd.Flags().GetInt("port")

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	specPath := filepath.Join(absDir, "workflow.yaml")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("no workflow.yaml found in %s", absDir)
	}

	engineDir := findEngineDir()
	if engineDir == "" {
		return fmt.Errorf("cannot find engine directory; ensure pipedreamer is installed correctly")
	}

	fmt.Printf("Starting dev server for %s on port %d...\n", absDir, port)

	denoCmd := exec.Command("deno", "run",
		"--allow-net", "--allow-read", "--allow-write=/tmp", "--allow-env",
		filepath.Join(engineDir, "main.ts"),
		"--workflow", specPath,
		"--port", fmt.Sprintf("%d", port),
		"--watch",
	)
	denoCmd.Dir = absDir
	denoCmd.Stdout = os.Stdout
	denoCmd.Stderr = os.Stderr
	denoCmd.Env = append(os.Environ(), fmt.Sprintf("PIPEDREAMER_PORT=%d", port))

	if err := denoCmd.Start(); err != nil {
		return fmt.Errorf("starting deno engine: %w", err)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	doneCh := make(chan error, 1)
	go func() { doneCh <- denoCmd.Wait() }()

	select {
	case sig := <-sigCh:
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
		_ = denoCmd.Process.Signal(syscall.SIGTERM)
		<-doneCh
		return nil
	case err := <-doneCh:
		if err != nil {
			return fmt.Errorf("engine exited: %w", err)
		}
		return nil
	}
}

func findEngineDir() string {
	// Check relative to the binary first
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "engine")
		if _, err := os.Stat(filepath.Join(candidate, "main.ts")); err == nil {
			return candidate
		}
	}

	// Check relative to working directory (development)
	if _, err := os.Stat("engine/main.ts"); err == nil {
		abs, _ := filepath.Abs("engine")
		return abs
	}

	// Check common install paths
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".pipedreamer", "engine"),
		"/usr/local/share/pipedreamer/engine",
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "main.ts")); err == nil {
			return c
		}
	}

	return ""
}
