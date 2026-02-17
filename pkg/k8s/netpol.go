package k8s

import (
	"fmt"

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

	// Build egress rules YAML
	var egressYAML string
	if len(egressRules) > 0 {
		egressYAML = "  egress:\n"
		for _, rule := range egressRules {
			egressYAML += fmt.Sprintf(`  - to:
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

			// Add non-DNS rules
			if rule.Port != 53 {
				egressYAML += fmt.Sprintf(`  - to:
    - podSelector: {}
    ports:
    - protocol: %s
      port: %d
`, rule.Protocol, rule.Port)
			}
		}
	}

	// Build ingress rules YAML
	var ingressYAML string
	if len(ingressRules) > 0 {
		ingressYAML = "  ingress:\n"
		for _, rule := range ingressRules {
			ingressYAML += fmt.Sprintf(`  - from:
    - podSelector: {}
    ports:
    - protocol: %s
      port: %d
`, rule.Protocol, rule.Port)
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
    app.kubernetes.io/managed-by: tentacular
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
