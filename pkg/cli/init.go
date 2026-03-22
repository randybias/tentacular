package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/scaffold"
)

var kebabCaseRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <name>",
		Short: "Create new workflow scaffold",
		Args:  cobra.ExactArgs(1),
		RunE:  runInit,
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	name := args[0]
	if !kebabCaseRe.MatchString(name) {
		return fmt.Errorf("workflow name must be kebab-case (e.g., my-workflow), got: %s", name)
	}

	dir := name
	if err := os.MkdirAll(filepath.Join(dir, "nodes"), 0o755); err != nil { //nolint:gosec // non-sensitive directory
		return fmt.Errorf("creating workflow directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "tests", "fixtures"), 0o755); err != nil { //nolint:gosec // non-sensitive directory
		return fmt.Errorf("creating tests directory: %w", err)
	}

	// Write workflow.yaml with contract and deployment sections
	workflowYAML := fmt.Sprintf(`name: %s
version: "1.0"
description: ""

triggers:
  - type: manual

nodes:
  hello:
    path: ./nodes/hello.ts

edges: []

config:
  timeout: 30s
  retries: 0

contract:
  version: "1"
  dependencies: {}

deployment:
  namespace: ""
`, name)
	if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(workflowYAML), 0o644); err != nil { //nolint:gosec // non-sensitive file
		return fmt.Errorf("writing workflow.yaml: %w", err)
	}

	// Write example node
	helloNode := `import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  ctx.log.info("hello node executed");
  return { message: "Hello from ` + name + `!" };
}
`
	if err := os.WriteFile(filepath.Join(dir, "nodes", "hello.ts"), []byte(helloNode), 0o644); err != nil { //nolint:gosec // non-sensitive file
		return fmt.Errorf("writing hello node: %w", err)
	}

	// Write .secrets.yaml.example
	secretsExample := `# Secrets configuration
# All values must be $shared.<name> references pointing to workspace .secrets/ directory.
# Example:
# github: $shared.github
# slack: $shared.slack
`
	if err := os.WriteFile(filepath.Join(dir, ".secrets.yaml.example"), []byte(secretsExample), 0o644); err != nil { //nolint:gosec // non-sensitive file
		return fmt.Errorf("writing secrets example: %w", err)
	}

	// Write .gitignore
	gitignore := `.secrets.yaml
scratch/
`
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil { //nolint:gosec // non-sensitive file
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	// Write test fixture
	fixture := `{
  "input": {},
  "expected": {
    "message": "Hello from ` + name + `!"
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "tests", "fixtures", "hello.json"), []byte(fixture), 0o644); err != nil { //nolint:gosec // non-sensitive file
		return fmt.Errorf("writing test fixture: %w", err)
	}

	// Write tentacle.yaml (from-scratch tentacle has no scaffold provenance)
	tentacleYAML := scaffold.TentacleYAML{
		Name:    name,
		Created: time.Now().UTC().Format(time.RFC3339),
	}
	tentacleData, err := yaml.Marshal(&tentacleYAML)
	if err != nil {
		return fmt.Errorf("marshaling tentacle.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tentacle.yaml"), tentacleData, 0o644); err != nil { //nolint:gosec // non-sensitive file
		return fmt.Errorf("writing tentacle.yaml: %w", err)
	}

	fmt.Printf("Created tentacle '%s' in ./%s/\n", name, dir)
	fmt.Printf("  workflow.yaml          - workflow definition (with contract + deployment)\n")
	fmt.Printf("  tentacle.yaml          - tentacle identity and provenance\n")
	fmt.Printf("  nodes/hello.ts         - example node\n")
	fmt.Printf("  tests/fixtures/        - test fixtures\n")
	fmt.Printf("  .secrets.yaml.example  - secrets template\n")
	fmt.Printf("  .gitignore             - default ignores\n")
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", dir)
	fmt.Printf("  tntc validate\n")
	fmt.Printf("  tntc dev\n")
	return nil
}
