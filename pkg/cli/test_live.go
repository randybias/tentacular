package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// runLiveTest deploys a workflow to a real cluster, runs it, validates the result,
// and (optionally) cleans up.
func runLiveTest(cmd *cobra.Command, args []string) error {
	startedAt := time.Now().UTC()

	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	specPath := filepath.Join(absDir, "workflow.yaml")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("no workflow.yaml found in %s", absDir)
	}

	envName, _ := cmd.Flags().GetString("env")
	keep, _ := cmd.Flags().GetBool("keep")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Load environment configuration
	env, err := ResolveEnvironment(envName)
	if err != nil {
		return fmt.Errorf("loading environment %q: %w", envName, err)
	}

	// Determine status output writer (stderr when -o json)
	w := StatusWriter(cmd)

	fmt.Fprintf(w, "Live test: environment=%s, namespace=%s\n", envName, env.Namespace)

	// Resolve MCP client
	mcpClient, err := requireMCPClient(cmd)
	if err != nil {
		return emitLiveResult(cmd, "fail", "MCP client error: "+err.Error(), nil, startedAt)
	}

	// Resolve image
	imageTag := env.Image
	if imageTag == "" {
		tagFilePath := filepath.Join(absDir, ".tentacular", "base-image.txt")
		if tagData, err := os.ReadFile(tagFilePath); err == nil {
			imageTag = strings.TrimSpace(string(tagData))
		}
	}
	if imageTag == "" {
		imageTag = "tentacular-engine:latest"
	}

	// Deploy
	deployOpts := InternalDeployOptions{
		Namespace:    env.Namespace,
		Image:        imageTag,
		RuntimeClass: env.RuntimeClass,
		Context:      env.Context,
		StatusOut:    w,
	}

	deployResult, err := deployWorkflow(absDir, deployOpts, mcpClient)
	if err != nil {
		return emitLiveResult(cmd, "fail", "deploy failed: "+err.Error(), nil, startedAt)
	}

	// Cleanup on exit unless --keep
	if !keep {
		defer func() {
			fmt.Fprintf(w, "Cleaning up %s from %s...\n", deployResult.WorkflowName, deployResult.Namespace)
			removeResult, delErr := deployResult.MCPClient.WfRemove(context.Background(), deployResult.Namespace, deployResult.WorkflowName)
			if delErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", delErr)
			} else {
				for _, d := range removeResult.Deleted {
					fmt.Fprintf(w, "  deleted %s\n", d)
				}
			}
		}()
	}

	// Trigger workflow run (MCP server handles readiness wait internally)
	fmt.Fprintf(w, "Running workflow %s (timeout: %s)...\n", deployResult.WorkflowName, timeout)
	runResult, err := deployResult.MCPClient.WfRun(cmd.Context(), deployResult.Namespace, deployResult.WorkflowName, nil, int(timeout.Seconds()))
	if err != nil {
		return emitLiveResult(cmd, "fail", "workflow run failed: "+err.Error(), nil, startedAt)
	}

	// Parse execution result
	var execution map[string]interface{}
	if err := json.Unmarshal(runResult.Output, &execution); err != nil {
		// Not valid JSON -- still a pass if we got output
		fmt.Printf("Workflow output (raw): %s\n", string(runResult.Output))
		return emitLiveResult(cmd, "pass", "workflow completed (raw output)", map[string]interface{}{"raw": string(runResult.Output)}, startedAt)
	}

	// Check for success field in the execution result
	success, _ := execution["success"].(bool)
	status := "pass"
	summary := "workflow completed successfully"
	if !success {
		status = "fail"
		summary = "workflow returned success=false"
	}

	return emitLiveResult(cmd, status, summary, execution, startedAt)
}

// emitLiveResult outputs the live test result in the appropriate format.
func emitLiveResult(cmd *cobra.Command, status, summary string, execution interface{}, startedAt time.Time) error {
	result := CommandResult{
		Version: "1",
		Command: "test",
		Status:  status,
		Summary: summary,
		Hints:   []string{},
		Timing: TimingInfo{
			StartedAt:  startedAt.Format(time.RFC3339),
			DurationMs: time.Since(startedAt).Milliseconds(),
		},
		Execution: execution,
	}

	if status == "fail" {
		result.Hints = append(result.Hints, "check deployment logs with: tntc logs <workflow-name>")
	}

	if err := EmitResult(cmd, result, os.Stdout); err != nil {
		return err
	}

	if status == "fail" {
		return fmt.Errorf("live test failed: %s", summary)
	}
	return nil
}
