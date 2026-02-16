package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// StatusWriter returns the appropriate writer for progress/status messages.
// When -o json is active, returns os.Stderr so stdout is clean JSON only.
// Otherwise returns os.Stdout.
func StatusWriter(cmd *cobra.Command) io.Writer {
	format, _ := cmd.Flags().GetString("output")
	if format == "json" {
		return os.Stderr
	}
	return os.Stdout
}

// TimingInfo holds execution timing metadata.
type TimingInfo struct {
	StartedAt  string `json:"startedAt"`
	DurationMs int64  `json:"durationMs"`
}

// CommandResult is the structured JSON envelope for all tntc commands.
type CommandResult struct {
	Version string   `json:"version"`
	Command string   `json:"command"`
	Status  string   `json:"status"`
	Summary string   `json:"summary"`
	Hints   []string `json:"hints"`
	Timing  TimingInfo `json:"timing"`

	// Command-specific fields (set by individual commands)
	Results   interface{} `json:"results,omitempty"`
	Phases    interface{} `json:"phases,omitempty"`
	Execution interface{} `json:"execution,omitempty"`
	Manifests interface{} `json:"manifests,omitempty"`
}

// EmitResult writes the CommandResult in the format requested by the -o flag.
// When -o json, it writes compact JSON. Otherwise it writes a human-readable
// text summary.
func EmitResult(cmd *cobra.Command, result CommandResult, w io.Writer) error {
	format, _ := cmd.Flags().GetString("output")
	if format == "json" {
		return emitJSON(result, w)
	}
	return emitText(result, w)
}

func emitJSON(result CommandResult, w io.Writer) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func emitText(result CommandResult, w io.Writer) error {
	icon := "PASS"
	if result.Status == "fail" {
		icon = "FAIL"
	}
	fmt.Fprintf(w, "[%s] %s\n", icon, result.Summary)
	for _, hint := range result.Hints {
		fmt.Fprintf(w, "  hint: %s\n", hint)
	}
	if result.Timing.DurationMs > 0 {
		fmt.Fprintf(w, "  (%dms)\n", result.Timing.DurationMs)
	}
	return nil
}
