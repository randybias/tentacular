package spec

import "strings"

// DeriveSecrets returns the list of required secret keys from contract dependencies.
// Returns empty slice if contract is nil or has no dependencies with auth.
func DeriveSecrets(c *Contract) []string {
	if c == nil || len(c.Dependencies) == 0 {
		return nil
	}

	var secrets []string
	for _, dep := range c.Dependencies {
		if dep.Auth != nil && dep.Auth.Secret != "" {
			secrets = append(secrets, dep.Auth.Secret)
		}
	}
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

	// Add additional egress overrides from networkPolicyOverride
	if c.NetworkPolicyOverride != nil {
		for _, override := range c.NetworkPolicyOverride.AdditionalEgress {
			// Parse CIDR and ports
			// For now, we just capture the override - actual NetworkPolicy rendering happens in pkg/k8s
			_ = override
		}
	}

	return rules
}

// IngressRule represents a single ingress network policy rule.
type IngressRule struct {
	Port     int
	Protocol string // "TCP"
}

// DeriveIngressRules returns ingress rules derived from workflow triggers.
// Webhook triggers enable ingress on port 8080.
// Returns empty slice if no webhook triggers.
func DeriveIngressRules(wf *Workflow) []IngressRule {
	var rules []IngressRule

	for _, trigger := range wf.Triggers {
		if trigger.Type == "webhook" {
			// Webhook triggers allow ingress on the service port
			rules = append(rules, IngressRule{
				Port:     8080,
				Protocol: "TCP",
			})
			break // Only need one ingress rule for webhooks
		}
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
