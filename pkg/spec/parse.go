package spec

import (
	"fmt"
	"log"
	"net"
	"regexp"

	"gopkg.in/yaml.v3"
)

var (
	kebabRe      = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	identRe      = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	semverRe     = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
	secretKeyRe  = regexp.MustCompile(`^[a-z][a-z0-9_-]*\.[a-z][a-z0-9_-]*$`)
)

var validTriggerTypes = map[string]bool{
	"manual":  true,
	"cron":    true,
	"webhook": true,
	"queue":   true,
}

var validProtocols = map[string]bool{
	"https":      true,
	"postgresql": true,
	"nats":       true,
	"blob":       true,
}

var protocolDefaultPorts = map[string]int{
	"https":      443,
	"postgresql": 5432,
	"nats":       4222,
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

	// Contract validation (optional section)
	if wf.Contract != nil {
		if contractErrs := ValidateContract(wf.Contract); len(contractErrs) > 0 {
			errs = append(errs, contractErrs...)
		}
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

// ValidateContract validates contract section including dependencies and network policy overrides.
// Exported for use in deploy preflight checks.
func ValidateContract(c *Contract) []string {
	var errs []string

	// Validate contract version
	if c.Version == "" {
		errs = append(errs, "contract.version is required")
	} else if c.Version != "1" {
		errs = append(errs, fmt.Sprintf("contract.version must be \"1\", got: %q", c.Version))
	}

	if c.Dependencies == nil {
		c.Dependencies = make(map[string]Dependency)
	}

	// Check for duplicate dependency names (map keys are unique by definition, but validate anyway)
	depNames := make(map[string]bool)
	for name := range c.Dependencies {
		if depNames[name] {
			errs = append(errs, fmt.Sprintf("contract: duplicate dependency name %q", name))
		}
		depNames[name] = true
	}

	// Validate each dependency
	for name, dep := range c.Dependencies {
		if !identRe.MatchString(name) {
			errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: name must match [a-z][a-z0-9_-]*", name))
		}

		// Protocol validation
		if dep.Protocol == "" {
			errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: protocol is required", name))
			continue
		}
		if !validProtocols[dep.Protocol] {
			log.Printf("Warning: contract.dependencies[%q]: unknown protocol %q (known protocols: https, postgresql, nats, blob)", name, dep.Protocol)
		}

		// Dynamic-target dependencies have their own validation
		if dep.Type == "dynamic-target" {
			if dep.CIDR == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: dynamic-target requires cidr", name))
			} else if !isValidCIDR(dep.CIDR) {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: invalid CIDR format %q", name, dep.CIDR))
			}
			if len(dep.DynPorts) == 0 {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: dynamic-target requires dynPorts", name))
			} else {
				for j, portSpec := range dep.DynPorts {
					port, _ := parsePortSpec(portSpec)
					if port <= 0 {
						errs = append(errs, fmt.Sprintf("contract.dependencies[%q].dynPorts[%d]: invalid port spec %q", name, j, portSpec))
					}
				}
			}
			// Skip protocol-specific field validation for dynamic-target
			if dep.Auth != nil {
				if dep.Auth.Type == "" {
					errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: auth.type is required when auth is present", name))
				}
				if dep.Auth.Secret == "" {
					errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: auth.secret is required when auth is present", name))
				} else if !secretKeyRe.MatchString(dep.Auth.Secret) {
					errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: auth.secret must be in \"service.key\" format, got: %q", name, dep.Auth.Secret))
				}
			}
			continue
		}

		// Protocol-specific field validation
		switch dep.Protocol {
		case "https":
			if dep.Host == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: https requires host", name))
			}
		case "postgresql":
			if dep.Host == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: postgresql requires host", name))
			}
			if dep.Database == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: postgresql requires database", name))
			}
			if dep.User == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: postgresql requires user", name))
			}
		case "nats":
			if dep.Host == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: nats requires host", name))
			}
			if dep.Subject == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: nats requires subject", name))
			}
		case "blob":
			if dep.Host == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: blob requires host", name))
			}
			if dep.Container == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: blob requires container", name))
			}
		}

		// Auth validation
		if dep.Auth != nil {
			if dep.Auth.Type == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: auth.type is required when auth is present", name))
			}
			if dep.Auth.Secret == "" {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: auth.secret is required when auth is present", name))
			} else if !secretKeyRe.MatchString(dep.Auth.Secret) {
				errs = append(errs, fmt.Sprintf("contract.dependencies[%q]: auth.secret must be in \"service.key\" format, got: %q", name, dep.Auth.Secret))
			}
		}
	}

	// Validate networkPolicy CIDR overrides
	if c.NetworkPolicy != nil {
		for i, override := range c.NetworkPolicy.AdditionalEgress {
			if override.ToCIDR == "" {
				errs = append(errs, fmt.Sprintf("contract.networkPolicy.additionalEgress[%d]: toCIDR is required", i))
			} else if !isValidCIDR(override.ToCIDR) {
				errs = append(errs, fmt.Sprintf("contract.networkPolicy.additionalEgress[%d]: invalid CIDR format %q", i, override.ToCIDR))
			}
			for j, portSpec := range override.Ports {
				port, _ := parsePortSpec(portSpec)
				if port <= 0 {
					errs = append(errs, fmt.Sprintf("contract.networkPolicy.additionalEgress[%d].ports[%d]: invalid port spec %q", i, j, portSpec))
				}
			}
		}
	}

	return errs
}

// isValidCIDR validates CIDR notation.
func isValidCIDR(s string) bool {
	_, _, err := net.ParseCIDR(s)
	return err == nil
}
