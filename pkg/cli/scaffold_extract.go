package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/scaffold"
)

func newScaffoldExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract a reusable scaffold from the current tentacle directory",
		Long: `Extract a reusable scaffold from a working tentacle.

Run from within a tentacle directory. Analyzes workflow.yaml config values,
replaces org-specific values with safe examples, generates params.schema.yaml,
and writes scaffold files to the output location.

By default, writes to ~/.tentacular/scaffolds/<name>/ (private scaffolds).
Use --public to write to ./scaffold-output/ for PR submission.
Use --json to preview the analysis without writing any files.`,
		RunE: runScaffoldExtract,
	}
	cmd.Flags().String("name", "", "Name for the scaffold (default: derived from tentacle name)")
	cmd.Flags().Bool("private", true, "Save to ~/.tentacular/scaffolds/<name>/ (default)")
	cmd.Flags().Bool("public", false, "Save to ./scaffold-output/ for PR to tentacular-scaffolds")
	cmd.Flags().String("output", "", "Override output directory")
	cmd.Flags().Bool("json", false, "Output analysis as JSON without generating files")
	return cmd
}

func runScaffoldExtract(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	public, _ := cmd.Flags().GetBool("public")
	outputDir, _ := cmd.Flags().GetString("output")
	jsonOnly, _ := cmd.Flags().GetBool("json")

	// Validate name if explicitly provided
	if name != "" {
		if err := scaffold.ValidateScaffoldName(name); err != nil {
			return fmt.Errorf("invalid scaffold name: %w", err)
		}
	}

	// Run from current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	opts := scaffold.ExtractOptions{
		Name:      name,
		OutputDir: outputDir,
		JSONOnly:  jsonOnly,
		Public:    public,
	}

	result, extractErr := scaffold.Extract(cwd, opts)
	if extractErr != nil {
		return extractErr
	}

	if jsonOnly {
		return printExtractJSON(result)
	}

	// Print any secret warnings first
	if len(result.Analysis.SecretWarnings) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: Potential secrets detected in workflow.yaml:\n")
		for _, w := range result.Analysis.SecretWarnings {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", w)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "\nExtract aborted. Remove secrets from workflow.yaml config before extracting.\n")
		fmt.Fprintf(cmd.ErrOrStderr(), "Secrets belong in .secrets.yaml (which is never copied to the scaffold).\n")
		return fmt.Errorf("extraction blocked: %d secret(s) found in config values", len(result.Analysis.SecretWarnings))
	}

	fmt.Print(scaffold.FormatExtractSummary(result))
	return nil
}

func printExtractJSON(result *scaffold.ExtractResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result.Analysis)
}
