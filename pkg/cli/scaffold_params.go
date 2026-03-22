package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/params"
)

// newScaffoldParamsCmd returns the params subgroup command.
func newScaffoldParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Inspect and validate scaffold parameters for a tentacle",
	}
	cmd.AddCommand(newScaffoldParamsShowCmd())
	cmd.AddCommand(newScaffoldParamsValidateCmd())
	return cmd
}

func newScaffoldParamsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current parameter values for the tentacle in the current directory",
		Args:  cobra.NoArgs,
		RunE:  runScaffoldParamsShow,
	}
}

func runScaffoldParamsShow(_ *cobra.Command, _ []string) error {
	schema, doc, err := loadSchemaAndWorkflow()
	if err != nil {
		return err
	}

	fmt.Printf("%-24s %-12s %s\n", "PARAMETER", "TYPE", "CURRENT VALUE")
	fmt.Println(strings.Repeat("-", 70))
	for name, def := range schema.Parameters {
		segs, pathErr := params.ParsePath(def.Path)
		if pathErr != nil {
			fmt.Printf("%-24s %-12s (path error: %v)\n", name, def.Type, pathErr)
			continue
		}
		val, resolveErr := resolvePathInDoc(doc, segs)
		if resolveErr != nil {
			fmt.Printf("%-24s %-12s (not found)\n", name, def.Type)
		} else {
			fmt.Printf("%-24s %-12s %v\n", name, def.Type, val)
		}
	}
	return nil
}

func newScaffoldParamsValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate that parameters have non-example values",
		Args:  cobra.NoArgs,
		RunE:  runScaffoldParamsValidate,
	}
}

func runScaffoldParamsValidate(_ *cobra.Command, _ []string) error {
	schema, doc, err := loadSchemaAndWorkflow()
	if err != nil {
		return err
	}

	var warnings []string
	for name, def := range schema.Parameters {
		segs, pathErr := params.ParsePath(def.Path)
		if pathErr != nil {
			warnings = append(warnings, fmt.Sprintf("  %s: path error: %v", name, pathErr))
			continue
		}
		val, resolveErr := resolvePathInDoc(doc, segs)
		if resolveErr != nil {
			if def.Required {
				warnings = append(warnings, fmt.Sprintf("  %s: required parameter not found", name))
			}
			continue
		}
		valStr := fmt.Sprintf("%v", val)
		if looksLikeExample(valStr) {
			warnings = append(warnings, fmt.Sprintf("  %s: value looks like an example: %v", name, val))
		}
	}

	if len(warnings) > 0 {
		fmt.Printf("Parameter validation warnings:\n%s\n", strings.Join(warnings, "\n"))
		return errors.New("workflow.yaml appears to still have example values")
	}

	fmt.Println("All parameters have non-example values.")
	return nil
}

func loadSchemaAndWorkflow() (*params.Schema, map[string]any, error) {
	if !fileExists("params.schema.yaml") {
		return nil, nil, errors.New("no params.schema.yaml found in current directory")
	}
	schema, schemaErr := params.LoadSchema("params.schema.yaml")
	if schemaErr != nil {
		return nil, nil, schemaErr
	}

	if !fileExists("workflow.yaml") {
		return nil, nil, errors.New("no workflow.yaml found in current directory")
	}
	data, readErr := os.ReadFile("workflow.yaml") //nolint:gosec // reading local workflow file
	if readErr != nil {
		return nil, nil, fmt.Errorf("reading workflow.yaml: %w", readErr)
	}

	var doc map[string]any
	if unmarshalErr := yaml.Unmarshal(data, &doc); unmarshalErr != nil {
		return nil, nil, fmt.Errorf("parsing workflow.yaml: %w", unmarshalErr)
	}
	return schema, doc, nil
}

func looksLikeExample(s string) bool {
	examplePatterns := []string{
		"example.com",
		"my-org",
		"my-bucket",
		"my-repo",
		"your-org",
		"your-bucket",
	}
	lower := strings.ToLower(s)
	for _, p := range examplePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// resolvePathInDoc resolves a path expression in a generic YAML document.
func resolvePathInDoc(doc map[string]any, segs []params.Segment) (any, error) {
	var cur any = doc
	for _, seg := range segs {
		switch v := cur.(type) {
		case map[string]any:
			child, ok := v[seg.Key]
			if !ok {
				return nil, fmt.Errorf("key '%s' not found", seg.Key)
			}
			if seg.FilterField != "" {
				found, filterErr := filterSlice(child, seg.FilterField, seg.FilterValue)
				if filterErr != nil {
					return nil, filterErr
				}
				cur = found
			} else {
				cur = child
			}
		case []any:
			if seg.FilterField == "" {
				return nil, errors.New("sequence navigation requires a filter")
			}
			found, filterErr := filterSlice(v, seg.FilterField, seg.FilterValue)
			if filterErr != nil {
				return nil, filterErr
			}
			cur = found
		default:
			return nil, fmt.Errorf("cannot navigate into %T at key '%s'", cur, seg.Key)
		}
	}
	return cur, nil
}

func filterSlice(val any, field, value string) (any, error) {
	slice, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("expected sequence, got %T", val)
	}
	for _, elem := range slice {
		m, ok := elem.(map[string]any)
		if !ok {
			continue
		}
		if fv, ok := m[field]; ok && fmt.Sprintf("%v", fv) == value {
			return m, nil
		}
	}
	return nil, fmt.Errorf("no element with %s=%s found", field, value)
}
