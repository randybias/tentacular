package k8s

import (
	"fmt"
	"sort"
	"strings"

	"github.com/randybias/tentacular/pkg/builder"
	"github.com/randybias/tentacular/pkg/spec"
)

// GenerateNetworkPolicy creates a K8s NetworkPolicy manifest from workflow contract.
// Returns nil if workflow has no contract (contract-less workflows skip NetworkPolicy).
func GenerateNetworkPolicy(wf *spec.Workflow, namespace string) *builder.Manifest {
	if wf.Contract == nil {
		return nil
	}

	egressRules := spec.DeriveEgressRules(wf.Contract)
	ingressRules := spec.DeriveIngressRules(wf)

	// Collect external hosts for annotation
	var externalHosts []string
	for _, rule := range egressRules {
		if rule.Port != 53 && !strings.HasSuffix(rule.Host, ".svc.cluster.local") && !strings.Contains(rule.Host, "/") {
			externalHosts = append(externalHosts, rule.Host)
		}
	}
	externalHostsAnnotation := ""
	if len(externalHosts) > 0 {
		externalHostsAnnotation = fmt.Sprintf("\n  annotations:\n    tentacular.dev/intended-hosts: %s", strings.Join(externalHosts, ","))
	}

	// Build egress rules YAML with proper network isolation
	var egressYAML string
	if len(egressRules) > 0 {
		egressYAML = "  egress:\n"
		for _, rule := range egressRules {
			egressYAML += buildEgressRule(rule)
		}
	}

	// Build ingress rules YAML
	var ingressYAML string
	if len(ingressRules) > 0 {
		ingressYAML = "  ingress:\n"
		for _, rule := range ingressRules {
			ingressYAML += "  - from:\n"
			if rule.FromLabels != nil {
				ingressYAML += "    - podSelector:\n"
				ingressYAML += "        matchLabels:\n"
				// Sort label keys for deterministic output
				labelKeys := make([]string, 0, len(rule.FromLabels))
				for k := range rule.FromLabels {
					labelKeys = append(labelKeys, k)
				}
				sort.Strings(labelKeys)
				for _, k := range labelKeys {
					ingressYAML += fmt.Sprintf("          %s: %s\n", k, rule.FromLabels[k])
				}
			} else {
				ingressYAML += "    - podSelector: {}\n"
			}
			// Add namespace selector if specified (e.g. allow ingress from istio-system)
			if rule.FromNamespaceLabels != nil {
				ingressYAML += "    - namespaceSelector:\n"
				ingressYAML += "        matchLabels:\n"
				nsLabelKeys := make([]string, 0, len(rule.FromNamespaceLabels))
				for k := range rule.FromNamespaceLabels {
					nsLabelKeys = append(nsLabelKeys, k)
				}
				sort.Strings(nsLabelKeys)
				for _, k := range nsLabelKeys {
					ingressYAML += fmt.Sprintf("          %s: %s\n", k, rule.FromNamespaceLabels[k])
				}
			}
			ingressYAML += fmt.Sprintf("    ports:\n    - protocol: %s\n      port: %d\n",
				rule.Protocol, rule.Port)
		}
	}

	// Generate NetworkPolicy manifest
	manifest := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %s-netpol
  namespace: %s
  labels:
    app.kubernetes.io/name: %s
    app.kubernetes.io/managed-by: tentacular%s
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: %s
  policyTypes:
  - Ingress
  - Egress
%s%s`,
		wf.Name,
		namespace,
		wf.Name,
		externalHostsAnnotation,
		wf.Name,
		egressYAML,
		ingressYAML,
	)

	return &builder.Manifest{
		Kind:    "NetworkPolicy",
		Name:    wf.Name + "-netpol",
		Content: manifest,
	}
}

// buildEgressRule creates a NetworkPolicy egress rule based on the host pattern.
// Three cases:
// 1. DNS (port 53 to kube-dns): podSelector + namespaceSelector for kube-system
// 2. Cluster-internal (*.svc.cluster.local): namespaceSelector targeting specific namespace
// 3. External hosts: ipBlock 0.0.0.0/0 with port restriction (v1 pragmatic approach)
func buildEgressRule(rule spec.EgressRule) string {
	// Case 1: DNS egress to kube-dns
	if rule.Port == 53 && strings.Contains(rule.Host, "kube-dns") {
		return fmt.Sprintf(`  - to:
    - podSelector:
        matchLabels:
          k8s-app: kube-dns
      namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
    ports:
    - protocol: %s
      port: %d
`, rule.Protocol, rule.Port)
	}

	// Case 2: Cluster-internal service (*.svc.cluster.local)
	if strings.HasSuffix(rule.Host, ".svc.cluster.local") {
		// Extract namespace from service FQDN: service-name.namespace.svc.cluster.local
		parts := strings.Split(rule.Host, ".")
		if len(parts) >= 2 {
			targetNamespace := parts[1]
			return fmt.Sprintf(`  # Cluster-internal service: %s
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: %s
    ports:
    - protocol: %s
      port: %d
`, rule.Host, targetNamespace, rule.Protocol, rule.Port)
		}
	}

	// Case 3: External host or CIDR override
	// For v1, use 0.0.0.0/0 with port restriction as pragmatic approach.
	// v2 enhancement: DNS-based CIDR resolution for specific hosts.
	var toBlock string
	if rule.Host == "0.0.0.0/0" || strings.Contains(rule.Host, "/") {
		// Already a CIDR (from networkPolicyOverride.additionalEgress)
		toBlock = fmt.Sprintf(`    - ipBlock:
        cidr: %s`, rule.Host)
	} else {
		// External hostname - allow to any non-private IP on this port
		// Excludes RFC1918 ranges to prevent external deps from reaching cluster-internal services
		toBlock = fmt.Sprintf(`    # External host: %s
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 10.0.0.0/8
        - 172.16.0.0/12
        - 192.168.0.0/16`, rule.Host)
	}

	if rule.Port == 0 {
		// No port restriction (from networkPolicyOverride with empty ports array)
		return fmt.Sprintf(`  - to:
%s
`, toBlock)
	}

	return fmt.Sprintf(`  - to:
%s
    ports:
    - protocol: %s
      port: %d
`, toBlock, rule.Protocol, rule.Port)
}
