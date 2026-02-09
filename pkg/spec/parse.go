package spec

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

var (
	kebabRe  = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	identRe  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	semverRe = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
)

var validTriggerTypes = map[string]bool{
	"manual":  true,
	"cron":    true,
	"webhook": true,
	"queue":   true,
}

// Parse parses and validates a workflow YAML spec.
// Returns the parsed workflow and a slice of validation errors (empty if valid).
func Parse(data []byte) (*Workflow, []string) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, []string{fmt.Sprintf("YAML parse error: %s", err)}
	}

	var errs []string

	// Required fields
	if wf.Name == "" {
		errs = append(errs, "name is required")
	} else if !kebabRe.MatchString(wf.Name) {
		errs = append(errs, fmt.Sprintf("name must be kebab-case, got: %q", wf.Name))
	}

	if wf.Version == "" {
		errs = append(errs, "version is required")
	} else if !semverRe.MatchString(wf.Version) {
		errs = append(errs, fmt.Sprintf("version must be semver (e.g., 1.0), got: %q", wf.Version))
	}

	// Triggers
	if len(wf.Triggers) == 0 {
		errs = append(errs, "at least one trigger is required")
	}
	triggerNames := make(map[string]bool)
	for i, t := range wf.Triggers {
		if !validTriggerTypes[t.Type] {
			errs = append(errs, fmt.Sprintf("trigger[%d]: invalid type %q (must be manual, cron, webhook, or queue)", i, t.Type))
		}
		if t.Type == "cron" && t.Schedule == "" {
			errs = append(errs, fmt.Sprintf("trigger[%d]: cron trigger requires schedule", i))
		}
		if t.Type == "webhook" && t.Path == "" {
			errs = append(errs, fmt.Sprintf("trigger[%d]: webhook trigger requires path", i))
		}
		if t.Type == "queue" && t.Subject == "" {
			errs = append(errs, fmt.Sprintf("trigger[%d]: queue trigger requires subject", i))
		}
		if t.Name != "" {
			if !identRe.MatchString(t.Name) {
				errs = append(errs, fmt.Sprintf("trigger[%d]: name must match [a-z][a-z0-9_-]*, got: %q", i, t.Name))
			}
			if triggerNames[t.Name] {
				errs = append(errs, fmt.Sprintf("trigger[%d]: duplicate trigger name %q", i, t.Name))
			}
			triggerNames[t.Name] = true
		}
	}

	// Nodes
	if len(wf.Nodes) == 0 {
		errs = append(errs, "at least one node is required")
	}
	for name, node := range wf.Nodes {
		if !identRe.MatchString(name) {
			errs = append(errs, fmt.Sprintf("node %q: name must match [a-z][a-z0-9_-]*", name))
		}
		if node.Path == "" {
			errs = append(errs, fmt.Sprintf("node %q: path is required", name))
		}
	}

	// Edges — reference integrity
	for i, edge := range wf.Edges {
		if _, ok := wf.Nodes[edge.From]; !ok {
			errs = append(errs, fmt.Sprintf("edge[%d]: from node %q not defined", i, edge.From))
		}
		if _, ok := wf.Nodes[edge.To]; !ok {
			errs = append(errs, fmt.Sprintf("edge[%d]: to node %q not defined", i, edge.To))
		}
		if edge.From == edge.To {
			errs = append(errs, fmt.Sprintf("edge[%d]: self-loop on %q", i, edge.From))
		}
	}

	// DAG acyclicity check
	if cycleErrs := checkCycles(&wf); len(cycleErrs) > 0 {
		errs = append(errs, cycleErrs...)
	}

	if len(errs) > 0 {
		return nil, errs
	}
	return &wf, nil
}

// checkCycles detects cycles in the DAG using DFS.
func checkCycles(wf *Workflow) []string {
	adj := make(map[string][]string)
	for _, e := range wf.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	for name := range wf.Nodes {
		color[name] = white
	}

	var errs []string
	var dfs func(string) bool
	dfs = func(u string) bool {
		color[u] = gray
		for _, v := range adj[u] {
			if color[v] == gray {
				errs = append(errs, fmt.Sprintf("cycle detected: %s → %s", u, v))
				return true
			}
			if color[v] == white {
				if dfs(v) {
					return true
				}
			}
		}
		color[u] = black
		return false
	}

	for name := range wf.Nodes {
		if color[name] == white {
			dfs(name)
		}
	}
	return errs
}
