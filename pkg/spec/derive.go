package spec

import (
	"sort"
	"strconv"
	"strings"
)

// DeriveSecrets returns the list of required secret keys from contract dependencies.
// Returns empty slice if contract is nil or has no dependencies with auth.
func DeriveSecrets(c *Contract) []string {
	if c == nil || len(c.Dependencies) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var secrets []string
	for _, dep := range c.Dependencies {
		if dep.Auth != nil && dep.Auth.Secret != "" && !seen[dep.Auth.Secret] {
			secrets = append(secrets, dep.Auth.Secret)
			seen[dep.Auth.Secret] = true
		}
	}
	sort.Strings(secrets)
	return secrets
}

// EgressRule represents a single egress network policy rule.
type EgressRule struct {
	Host     string
	Port     int
	Protocol string // "TCP" or "UDP"
}

// DeriveEgressRules returns egress rules derived from contract dependencies.
// Includes DNS egress (UDP/TCP 53 to kube-dns) by default.
// Returns empty slice if contract is nil.
func DeriveEgressRules(c *Contract) []EgressRule {
	var rules []EgressRule

	// Always include DNS egress
	rules = append(rules,
		EgressRule{Host: "kube-dns.kube-system.svc.cluster.local", Port: 53, Protocol: "UDP"},
		EgressRule{Host: "kube-dns.kube-system.svc.cluster.local", Port: 53, Protocol: "TCP"},
	)

	if c == nil || len(c.Dependencies) == 0 {
		return rules
	}

	// Add dependency-derived egress
	for _, dep := range c.Dependencies {
		// jsr/npm deps are resolved via the in-cluster module proxy — no direct external egress.
		if dep.Protocol == "jsr" || dep.Protocol == "npm" {
			continue
		}

		// Dynamic-target dependencies use CIDR + DynPorts instead of Host + Port
		if dep.Type == "dynamic-target" {
			for _, portStr := range dep.DynPorts {
				port, proto := parsePortSpec(portStr)
				if port > 0 {
					rules = append(rules, EgressRule{
						Host:     dep.CIDR,
						Port:     port,
						Protocol: proto,
					})
				}
			}
			continue
		}

		port := dep.Port
		if port == 0 {
			// Apply default port if not specified
			if defaultPort, ok := protocolDefaultPorts[dep.Protocol]; ok {
				port = defaultPort
			}
		}

		if dep.Host != "" && port > 0 {
			rules = append(rules, EgressRule{
				Host:     dep.Host,
				Port:     port,
				Protocol: "TCP",
			})
		}
	}

	// Add additional egress overrides from networkPolicy
	if c.NetworkPolicy != nil {
		for _, override := range c.NetworkPolicy.AdditionalEgress {
			for _, portStr := range override.Ports {
				port, proto := parsePortSpec(portStr)
				if port > 0 {
					rules = append(rules, EgressRule{
						Host:     override.ToCIDR,
						Port:     port,
						Protocol: proto,
					})
				}
			}
			// If no ports specified, add a rule with port 0 (any)
			if len(override.Ports) == 0 {
				rules = append(rules, EgressRule{
					Host:     override.ToCIDR,
					Port:     0,
					Protocol: "TCP",
				})
			}
		}
	}

	// Sort dependency-derived rules for deterministic output (DNS rules stay first)
	sort.Slice(rules[2:], func(i, j int) bool {
		ri, rj := rules[2+i], rules[2+j]
		if ri.Host != rj.Host {
			return ri.Host < rj.Host
		}
		if ri.Port != rj.Port {
			return ri.Port < rj.Port
		}
		return ri.Protocol < rj.Protocol
	})

	return rules
}

// IngressRule represents a single ingress network policy rule.
type IngressRule struct {
	Port                int
	Protocol            string            // "TCP"
	FromLabels          map[string]string // if non-nil, restrict to matching pods (podSelector)
	FromNamespaceLabels map[string]string // if non-nil, also allow from matching namespaces (namespaceSelector)
}

// DeriveIngressRules returns ingress rules derived from workflow triggers.
// Returns label-scoped ingress for internal triggers (CronJob/runner) and open ingress for webhooks.
func DeriveIngressRules(wf *Workflow) []IngressRule {
	var rules []IngressRule

	// Check if workflow has webhook triggers
	hasWebhook := false
	for _, trigger := range wf.Triggers {
		if trigger.Type == "webhook" {
			hasWebhook = true
			break
		}
	}

	if hasWebhook {
		// Webhook triggers need ingress from:
		//   - any pod in the same namespace (podSelector: {})
		//   - Istio gateway pods in istio-system (for cluster ingress routing)
		rules = append(rules, IngressRule{
			Port:       8080,
			Protocol:   "TCP",
			FromLabels: nil, // nil = podSelector: {} (any pod in namespace)
			FromNamespaceLabels: map[string]string{
				"kubernetes.io/metadata.name": "istio-system",
			},
		})
	} else {
		// Non-webhook workflows only need label-scoped ingress for internal triggers
		// The runner Job (tntc test --live) and CronJob triggers POST to the engine service
		rules = append(rules, IngressRule{
			Port:       8080,
			Protocol:   "TCP",
			FromLabels: map[string]string{"tentacular.dev/role": "trigger"},
		})
	}

	return rules
}

// ApplyDefaultPorts applies default ports to dependencies where port is not specified.
func ApplyDefaultPorts(c *Contract) {
	if c == nil || len(c.Dependencies) == 0 {
		return
	}

	for name, dep := range c.Dependencies {
		if dep.Port == 0 {
			if defaultPort, ok := protocolDefaultPorts[dep.Protocol]; ok {
				dep.Port = defaultPort
				c.Dependencies[name] = dep
			}
		}
	}
}

// GetSecretServiceName extracts the service name from a "service.key" secret reference.
func GetSecretServiceName(secretKey string) string {
	parts := strings.SplitN(secretKey, ".", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// GetSecretKeyName extracts the key name from a "service.key" secret reference.
func GetSecretKeyName(secretKey string) string {
	parts := strings.SplitN(secretKey, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// parsePortSpec parses a port specification like "443/TCP" or "53/UDP".
// Returns port number and protocol. Defaults to TCP if no protocol specified.
func parsePortSpec(spec string) (int, string) {
	parts := strings.SplitN(spec, "/", 2)
	port, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, ""
	}
	proto := "TCP"
	if len(parts) == 2 {
		proto = strings.ToUpper(parts[1])
	}
	return port, proto
}

// HasModuleProxyDeps returns true if the workflow contract has any jsr or npm dependencies
// that are resolved via the in-cluster module proxy (esm.sh).
func HasModuleProxyDeps(wf *Workflow) bool {
	if wf == nil || wf.Contract == nil {
		return false
	}
	for _, dep := range wf.Contract.Dependencies {
		if dep.Protocol == "jsr" || dep.Protocol == "npm" {
			return true
		}
	}
	return false
}

// moduleProxyHost is the in-cluster hostname:port of the esm.sh module proxy service.
// TODO(allow-net): This is hardcoded to the default tentacular-system namespace.
// Once the module proxy config is plumbed through to DeriveDenoFlags, this should
// be derived from ModuleProxyConfig. The broader issue is that --allow-net scoping
// does not currently account for the proxy host at all — workflow pods using jsr/npm
// deps will have their imports rewritten to the proxy URL but the scoped --allow-net
// won't include it, blocking the connection. Fix: pass proxyHost into DeriveDenoFlags
// and add it to the scoped allow list. Tracked as known issue: "Deno allow-net broken
// for module proxy deps".
const moduleProxyHost = "esm-sh.tentacular-system.svc.cluster.local:8080"

// DeriveDenoFlags returns the complete Deno command with permission flags based on contract dependencies.
// Returns nil if contract is nil or has no dependencies.
// When any dependency has type "dynamic-target", returns broad --allow-net.
// When all dependencies are fixed-host, returns scoped --allow-net=host1:port,host2:port,...
// Always includes 0.0.0.0:8080 in scoped mode for internal health endpoints.
// When jsr/npm deps are present, adds the module proxy host to the scoped allow list.
// Scopes --allow-env to DENO_DIR,HOME only.
func DeriveDenoFlags(c *Contract) []string {
	if c == nil || len(c.Dependencies) == 0 {
		return nil
	}

	// Check if any dependency is dynamic-target or uses the module proxy (jsr/npm)
	hasDynamic := false
	hasModuleProxyDeps := false
	var allowedHosts []string
	for _, dep := range c.Dependencies {
		if dep.Type == "dynamic-target" {
			hasDynamic = true
			break
		}
		if dep.Protocol == "jsr" || dep.Protocol == "npm" {
			hasModuleProxyDeps = true
		}
	}

	var allowNetFlag string
	if hasDynamic {
		// Broad network access for dynamic targets
		allowNetFlag = "--allow-net"
	} else {
		// Build scoped network access list
		seen := make(map[string]bool)
		for _, dep := range c.Dependencies {
			// jsr/npm deps are served by the module proxy — add proxy host, not the package host
			if dep.Protocol == "jsr" || dep.Protocol == "npm" {
				continue
			}
			if dep.Host != "" {
				port := dep.Port
				if port == 0 {
					// Apply default port if not specified
					if defaultPort, ok := protocolDefaultPorts[dep.Protocol]; ok {
						port = defaultPort
					}
				}
				if port > 0 {
					hostPort := dep.Host + ":" + strconv.Itoa(port)
					if !seen[hostPort] {
						allowedHosts = append(allowedHosts, hostPort)
						seen[hostPort] = true
					}
				}
			}
		}

		// Add module proxy host when jsr/npm deps are present
		if hasModuleProxyDeps && !seen[moduleProxyHost] {
			allowedHosts = append(allowedHosts, moduleProxyHost)
			seen[moduleProxyHost] = true
		}

		// Always include localhost:8080 for health endpoints
		if !seen["0.0.0.0:8080"] {
			allowedHosts = append(allowedHosts, "0.0.0.0:8080")
		}

		// Sort for deterministic output
		sort.Strings(allowedHosts)
		allowNetFlag = "--allow-net=" + strings.Join(allowedHosts, ",")
	}

	flags := []string{
		"deno",
		"run",
		"--no-lock",
		"--unstable-net",
		allowNetFlag,
		"--allow-read=/app,/var/run/secrets",
		"--allow-write=/tmp",
		"--allow-env=DENO_DIR,HOME",
	}

	// Deno 2 requires explicit --allow-import permission for any host from which
	// modules are imported. The in-cluster module proxy is not on Deno's built-in
	// allowlist, so workflow pods that use jsr:/npm: deps must explicitly allow it.
	if hasModuleProxyDeps {
		flags = append(flags, "--allow-import="+moduleProxyHost)
	}

	// Note: when jsr/npm deps are present, the Deployment mounts a merged deno.json
	// (engine imports + workflow proxy rewrites) at /app/engine/deno.json. Deno
	// auto-discovers this config — no --import-map flag needed.

	flags = append(flags,
		"engine/main.ts",
		"--workflow",
		"/app/workflow/workflow.yaml",
		"--port",
		"8080",
	)
	return flags
}
