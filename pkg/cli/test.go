package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [dir][/<node>]",
		Short: "Test workflow or individual node",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runTest,
	}
	cmd.Flags().Bool("pipeline", false, "Run full pipeline test")
	return cmd
}

func runTest(cmd *cobra.Command, args []string) error {
	target := "."
	var nodeName string

	if len(args) > 0 {
		target = args[0]
		// Support workflow/node syntax: if the target path is not a directory,
		// treat the last component as a node name and the parent as the
		// workflow directory.
		// Examples:
		//   "myworkflow/fetch-data" -> dir="myworkflow", node="fetch-data"
		//   "/tmp/test-workflow/greet" -> dir="/tmp/test-workflow", node="greet"
		//   "myworkflow" (is a dir) -> dir="myworkflow", node=""
		//   "/tmp/test-workflow" (is a dir) -> dir="/tmp/test-workflow", node=""
		if info, err := os.Stat(target); err != nil || !info.IsDir() {
			nodeName = filepath.Base(target)
			target = filepath.Dir(target)
		}
	}

	absDir, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	specPath := filepath.Join(absDir, "workflow.yaml")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("no workflow.yaml found in %s", absDir)
	}

	engineDir := findEngineDir()
	if engineDir == "" {
		return fmt.Errorf("cannot find engine directory")
	}

	pipeline, _ := cmd.Flags().GetBool("pipeline")

	denoArgs := []string{
		"run", "--allow-net", "--allow-read", "--allow-write=/tmp", "--allow-env",
		filepath.Join(engineDir, "testing", "runner.ts"),
		"--workflow", specPath,
	}

	if nodeName != "" {
		denoArgs = append(denoArgs, "--node", nodeName)
	}
	if pipeline {
		denoArgs = append(denoArgs, "--pipeline")
	}

	denoCmd := exec.Command("deno", denoArgs...)
	denoCmd.Dir = absDir
	denoCmd.Stdout = os.Stdout
	denoCmd.Stderr = os.Stderr

	if err := denoCmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	return nil
}
