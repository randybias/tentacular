package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randybias/tentacular/pkg/catalog"
	"github.com/randybias/tentacular/pkg/version"
)

func newCatalogInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <template-name> [workflow-name]",
		Short: "Scaffold a workflow from a catalog template",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runCatalogInit,
	}
	cmd.Flags().String("namespace", "", "Set deployment.namespace in workflow.yaml")
	cmd.Flags().Bool("no-cache", false, "Force re-fetch of catalog index")
	return cmd
}

func runCatalogInit(cmd *cobra.Command, args []string) error {
	templateName := args[0]
	workflowName := templateName
	if len(args) > 1 {
		workflowName = args[1]
	}

	if !kebabCaseRe.MatchString(workflowName) {
		return fmt.Errorf("workflow name must be kebab-case (e.g., my-workflow), got: %s", workflowName)
	}

	namespace, _ := cmd.Flags().GetString("namespace")
	noCache, _ := cmd.Flags().GetBool("no-cache")

	cfg := LoadConfig()
	client := catalog.NewClient(cfg.Catalog)

	idx, err := client.FetchIndex(noCache)
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	tmpl, err := findTemplate(idx.Templates, templateName)
	if err != nil {
		return err
	}

	// Version check (warn only)
	checkMinVersion(tmpl.MinTentacularVersion)

	// Fetch and write each file
	dir := workflowName
	for _, file := range tmpl.Files {
		remotePath := tmpl.Path + "/" + file
		data, err := client.FetchFile(remotePath)
		if err != nil {
			return fmt.Errorf("fetching template file %s: %w", file, err)
		}

		localPath := filepath.Join(dir, file)
		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil { //nolint:gosec // 0o755 for template directory
			return fmt.Errorf("creating directory for %s: %w", file, err)
		}

		// Replace name in workflow.yaml
		if file == "workflow.yaml" {
			content := string(data)
			nameRe := regexp.MustCompile(`(?m)^name:\s+\S+`)
			content = nameRe.ReplaceAllString(content, "name: "+workflowName)
			if namespace != "" {
				nsRe := regexp.MustCompile(`(?m)^(\s*namespace:\s+)\S+`)
				if nsRe.MatchString(content) {
					content = nsRe.ReplaceAllString(content, "${1}"+namespace)
				} else {
					// Insert namespace under deployment section or append deployment block
					deployRe := regexp.MustCompile(`(?m)^deployment:`)
					if deployRe.MatchString(content) {
						content = deployRe.ReplaceAllString(content, "deployment:\n  namespace: "+namespace)
					} else {
						content += "\ndeployment:\n  namespace: " + namespace + "\n"
					}
				}
			}
			data = []byte(content)
		}

		if err := os.WriteFile(localPath, data, 0o644); err != nil { //nolint:gosec // non-sensitive template file
			return fmt.Errorf("writing %s: %w", file, err)
		}
	}

	fmt.Printf("Created workflow '%s' from template '%s' in ./%s/\n", workflowName, templateName, dir)
	for _, f := range tmpl.Files {
		fmt.Printf("  %s\n", f)
	}
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", dir)
	fmt.Printf("  tntc validate\n")
	fmt.Printf("  tntc dev\n")
	return nil
}

func checkMinVersion(minVer string) {
	if minVer == "" {
		return
	}
	current := version.Version
	if current == "dev" {
		return
	}
	if compareSemver(current, minVer) < 0 {
		fmt.Fprintf(os.Stderr, "Warning: this template requires tntc %s or later (you have %s)\n", minVer, current)
	}
}

// compareSemver returns -1, 0, or 1 comparing two semver strings.
// Returns 0 if either is unparseable.
func compareSemver(a, b string) int {
	aParts := parseSemver(a)
	bParts := parseSemver(b)
	if aParts == nil || bParts == nil {
		return 0
	}
	for i := range 3 {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		// Strip any pre-release suffix (e.g., "1-beta")
		if idx := strings.IndexAny(p, "-+"); idx >= 0 {
			p = p[:idx]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}
