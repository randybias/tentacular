package k8s

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ClusterProfile is a point-in-time capability snapshot of a target cluster environment.
// Written to .tentacular/envprofiles/<env>.md and <env>.json.
type ClusterProfile struct {
	GeneratedAt    time.Time          `json:"generatedAt"          yaml:"generatedAt"`
	LimitRange     *LimitRangeSummary `json:"limitRange,omitempty" yaml:"limitRange,omitempty"`
	Quota          *QuotaSummary      `json:"quota,omitempty"      yaml:"quota,omitempty"`
	Environment    string             `json:"environment"          yaml:"environment"`
	K8sVersion     string             `json:"k8sVersion"           yaml:"k8sVersion"`
	PodSecurity    string             `json:"podSecurity"          yaml:"podSecurity"`
	RWXNote        string             `json:"rwxNote"              yaml:"rwxNote"`
	Namespace      string             `json:"namespace"            yaml:"namespace"`
	Distribution   string             `json:"distribution"         yaml:"distribution"`
	CNI            CNIInfo            `json:"cni"                  yaml:"cni"`
	StorageClasses []StorageClassInfo `json:"storageClasses"       yaml:"storageClasses"`
	Guidance       []string           `json:"guidance"             yaml:"guidance"`
	CSIDrivers     []string           `json:"csiDrivers"           yaml:"csiDrivers"`
	Ingress        []string           `json:"ingress"              yaml:"ingress"`
	RuntimeClasses []RuntimeClassInfo `json:"runtimeClasses"       yaml:"runtimeClasses"`
	Nodes          []NodeInfo         `json:"nodes"                yaml:"nodes"`
	Exoskeleton    ExoskeletonInfo    `json:"exoskeleton"          yaml:"exoskeleton"`
	Extensions     ExtensionSet       `json:"extensions"           yaml:"extensions"`
	NetworkPolicy  NetPolInfo         `json:"networkPolicy"        yaml:"networkPolicy"`
	GVisor         bool               `json:"gvisor"               yaml:"gvisor"`
}

// ExoskeletonInfo describes exoskeleton service availability in the cluster.
type ExoskeletonInfo struct {
	Services          []string `json:"services,omitempty"          yaml:"services,omitempty"`
	Enabled           bool     `json:"enabled"                     yaml:"enabled"`
	CleanupOnUndeploy bool     `json:"cleanupOnUndeploy,omitempty" yaml:"cleanupOnUndeploy,omitempty"`
}

// NodeInfo summarizes a single cluster node.
type NodeInfo struct {
	Name   string            `json:"name"   yaml:"name"`
	Arch   string            `json:"arch"   yaml:"arch"`
	OS     string            `json:"os"     yaml:"os"`
	Labels map[string]string `json:"labels" yaml:"labels"`
	Taints []string          `json:"taints" yaml:"taints"`
}

// RuntimeClassInfo describes a Kubernetes RuntimeClass.
type RuntimeClassInfo struct {
	Name    string `json:"name"    yaml:"name"`
	Handler string `json:"handler" yaml:"handler"`
}

// CNIInfo describes the detected Container Network Interface plugin.
type CNIInfo struct {
	Name                   string `json:"name"                   yaml:"name"`
	Version                string `json:"version,omitempty"      yaml:"version,omitempty"`
	NetworkPolicySupported bool   `json:"networkPolicySupported" yaml:"networkPolicySupported"`
	EgressSupported        bool   `json:"egressSupported"        yaml:"egressSupported"`
}

// NetPolInfo describes NetworkPolicy support and usage in the cluster.
type NetPolInfo struct {
	Supported bool `json:"supported" yaml:"supported"`
	InUse     bool `json:"inUse"     yaml:"inUse"`
}

// StorageClassInfo describes a Kubernetes StorageClass.
// RWXCapable is inferred from the provisioner name — it is a heuristic hint,
// not a guarantee. See ClusterProfile.RWXNote for the qualification.
type StorageClassInfo struct {
	Name                 string `json:"name"                 yaml:"name"`
	Provisioner          string `json:"provisioner"          yaml:"provisioner"`
	ReclaimPolicy        string `json:"reclaimPolicy"        yaml:"reclaimPolicy"`
	IsDefault            bool   `json:"isDefault"            yaml:"isDefault"`
	AllowVolumeExpansion bool   `json:"allowVolumeExpansion" yaml:"allowVolumeExpansion"`
	RWXCapable           bool   `json:"rwxCapable"           yaml:"rwxCapable"` // inferred from provisioner name; see RWXNote
}

// ExtensionSet records which well-known CRD-based extensions are installed.
type ExtensionSet struct {
	OtherCRDGroups  []string `json:"otherCRDGroups"  yaml:"otherCRDGroups"`
	Istio           bool     `json:"istio"           yaml:"istio"`
	CertManager     bool     `json:"certManager"     yaml:"certManager"`
	PrometheusOp    bool     `json:"prometheusOp"    yaml:"prometheusOp"`
	ExternalSecrets bool     `json:"externalSecrets" yaml:"externalSecrets"`
	ArgoCD          bool     `json:"argoCD"          yaml:"argoCD"`
	GatewayAPI      bool     `json:"gatewayAPI"      yaml:"gatewayAPI"`
	MetricsServer   bool     `json:"metricsServer"   yaml:"metricsServer"`
}

// QuotaSummary contains resource quota limits for the target namespace.
type QuotaSummary struct {
	CPURequest    string `json:"cpuRequest,omitempty"    yaml:"cpuRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"      yaml:"cpuLimit,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty" yaml:"memoryRequest,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"   yaml:"memoryLimit,omitempty"`
	MaxPods       int    `json:"maxPods,omitempty"       yaml:"maxPods,omitempty"`
}

// LimitRangeSummary contains default container resource limits for the target namespace.
type LimitRangeSummary struct {
	DefaultCPURequest    string `json:"defaultCPURequest,omitempty"    yaml:"defaultCPURequest,omitempty"`
	DefaultCPULimit      string `json:"defaultCPULimit,omitempty"      yaml:"defaultCPULimit,omitempty"`
	DefaultMemoryRequest string `json:"defaultMemoryRequest,omitempty" yaml:"defaultMemoryRequest,omitempty"`
	DefaultMemoryLimit   string `json:"defaultMemoryLimit,omitempty"   yaml:"defaultMemoryLimit,omitempty"`
}

// --- helpers ---

func detectDistribution(nodes *corev1.NodeList) string {
	for _, n := range nodes.Items {
		labels := n.Labels
		if _, ok := labels["eks.amazonaws.com/nodegroup"]; ok {
			return "eks"
		}
		if _, ok := labels["cloud.google.com/gke-nodepool"]; ok {
			return "gke"
		}
		if _, ok := labels["kubernetes.azure.com/agentpool"]; ok {
			return "aks"
		}
		if v, ok := labels["node.kubernetes.io/instance-type"]; ok && strings.HasPrefix(v, "k3s") {
			return "k3s"
		}
		if _, ok := labels["node.k0sproject.io/role"]; ok {
			return "k0s"
		}
	}
	return "vanilla"
}

func detectCNI(pods *corev1.PodList) CNIInfo {
	for _, pod := range pods.Items {
		app := pod.Labels["k8s-app"]
		name := pod.Name
		switch {
		case app == "calico-node":
			return CNIInfo{Name: "calico", NetworkPolicySupported: true, EgressSupported: true}
		case app == "cilium":
			return CNIInfo{Name: "cilium", NetworkPolicySupported: true, EgressSupported: true}
		case app == "kube-router":
			return CNIInfo{Name: "kube-router", NetworkPolicySupported: true, EgressSupported: true}
		case strings.Contains(name, "weave"):
			return CNIInfo{Name: "weave", NetworkPolicySupported: true, EgressSupported: true}
		case strings.Contains(name, "flannel"):
			return CNIInfo{Name: "flannel", NetworkPolicySupported: false, EgressSupported: false}
		case strings.Contains(name, "kindnet"):
			return CNIInfo{Name: "kindnet", NetworkPolicySupported: false, EgressSupported: false}
		}
	}
	return CNIInfo{Name: "unknown", NetworkPolicySupported: false, EgressSupported: false}
}

func isRWXCapable(provisioner string) bool {
	rwxKeywords := []string{"efs", "nfs", "azurefile", "azure-file", "cephfs", "glusterfs", "rbd"}
	lower := strings.ToLower(provisioner)
	for _, kw := range rwxKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func classifyExtensions(crdList *unstructured.UnstructuredList) ExtensionSet {
	ext := ExtensionSet{}
	knownGroups := map[string]bool{}

	for _, item := range crdList.Items {
		name := item.GetName() // e.g. "virtualservices.networking.istio.io"
		parts := strings.SplitN(name, ".", 2)
		if len(parts) < 2 {
			continue
		}
		group := parts[1]

		switch {
		case strings.HasSuffix(group, ".istio.io") || group == "networking.istio.io" || group == "security.istio.io":
			ext.Istio = true
		case group == "cert-manager.io" || strings.HasSuffix(group, ".cert-manager.io"):
			ext.CertManager = true
		case group == "monitoring.coreos.com":
			ext.PrometheusOp = true
		case group == "external-secrets.io" || strings.HasSuffix(group, ".external-secrets.io"):
			ext.ExternalSecrets = true
		case group == "argoproj.io":
			ext.ArgoCD = true
		case group == "gateway.networking.k8s.io":
			ext.GatewayAPI = true
		default:
			// Collect unknown CRD groups for the agent
			if !knownGroups[group] {
				knownGroups[group] = true
				ext.OtherCRDGroups = append(ext.OtherCRDGroups, group)
			}
		}
	}
	sort.Strings(ext.OtherCRDGroups)
	// Cap to 20 entries to avoid bloating agent context on heavily-operatored clusters.
	const maxOtherCRDGroups = 20
	if len(ext.OtherCRDGroups) > maxOtherCRDGroups {
		ext.OtherCRDGroups = append(
			ext.OtherCRDGroups[:maxOtherCRDGroups],
			fmt.Sprintf("... and %d more (truncated)", len(ext.OtherCRDGroups)-maxOtherCRDGroups),
		)
	}
	return ext
}

func detectPodSecurity(ns *corev1.Namespace) string {
	if ns == nil {
		return "unknown"
	}
	if v, ok := ns.Labels["pod-security.kubernetes.io/enforce"]; ok {
		return v
	}
	return "unknown"
}

func deriveGuidance(p *ClusterProfile) []string {
	var g []string

	if p.GVisor {
		g = append(g, "Use runtime_class: gvisor for untrusted workflow steps")
	} else {
		g = append(g, "gVisor not available — omit runtime_class or set it to \"\"")
	}

	if p.Distribution == "kind" {
		g = append(g, "kind cluster detected: set runtime_class: \"\" and imagePullPolicy: IfNotPresent")
	}

	if p.CNI.Name == "unknown" {
		g = append(g, "WARNING: CNI plugin could not be detected (kube-system pod labels may differ or RBAC may restrict listing pods) — NetworkPolicy support is unknown; verify manually before relying on egress controls")
	}

	if p.Extensions.Istio {
		g = append(g, "Istio detected: NetworkPolicy egress rules must include namespaceSelector for istio-system; mTLS available between pods")
	}

	if p.NetworkPolicy.Supported && !p.NetworkPolicy.InUse {
		g = append(g, "NetworkPolicy is supported but none exist yet — generated policies will be the first; test egress rules carefully")
	}

	hasRWX := false
	for _, sc := range p.StorageClasses {
		if sc.RWXCapable {
			hasRWX = true
			g = append(g, fmt.Sprintf("RWX storage (inferred) via StorageClass %q — verify actual RWX support with the CSI driver before use", sc.Name))
		}
	}
	if !hasRWX {
		g = append(g, "No RWX-capable StorageClass inferred — avoid shared volume mounts across replicas (verify with cluster admin)")
	}

	if p.Quota != nil {
		g = append(g, fmt.Sprintf("ResourceQuota active in namespace %q: CPU limit %s, memory limit %s",
			p.Namespace, p.Quota.CPULimit, p.Quota.MemoryLimit))
	}

	switch p.PodSecurity {
	case "restricted":
		g = append(g, "Namespace enforces restricted PodSecurity — containers must run as non-root with no privilege escalation")
	case "unknown":
		g = append(g, "WARNING: Pod Security Admission not configured — recommend enabling 'restricted' profile for production namespaces")
	}

	if p.Extensions.CertManager {
		g = append(g, "cert-manager available — TLS certificates can be provisioned automatically")
	}

	// Exoskeleton guidance
	if p.Exoskeleton.Enabled && len(p.Exoskeleton.Services) > 0 {
		g = append(g, fmt.Sprintf("Exoskeleton services detected: %s — use tentacular-* prefix in contracts to reference these services",
			strings.Join(p.Exoskeleton.Services, ", ")))
		if p.Exoskeleton.CleanupOnUndeploy {
			g = append(g, "Exoskeleton cleanup-on-undeploy is enabled — undeploying a workflow will permanently remove its provisioned resources (Postgres schemas, NATS credentials, RustFS objects)")
		}
	}

	// Security note: node labels are included verbatim and may contain sensitive metadata.
	// Cloud-managed clusters (EKS/GKE/AKS) routinely include account IDs and region info in labels.
	note := "SECURITY NOTE: Node labels are included verbatim in this profile"
	if p.Distribution == "eks" || p.Distribution == "gke" || p.Distribution == "aks" {
		note += fmt.Sprintf(" — %s clusters commonly include account IDs, region names, and internal topology metadata in node labels.", strings.ToUpper(p.Distribution))
	} else {
		note += " — review before committing this file to a shared repository."
	}
	g = append(g, note)

	return g
}

// Markdown renders the profile as human-readable markdown for agent consumption.
func (p *ClusterProfile) Markdown() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Cluster Environment Profile: %s\n", p.Environment)
	fmt.Fprintf(&sb, "Generated: %s | K8s: %s | Distribution: %s\n\n",
		p.GeneratedAt.Format(time.RFC3339), p.K8sVersion, p.Distribution)

	// Identity
	fmt.Fprintf(&sb, "## Identity\n")
	fmt.Fprintf(&sb, "- **Version:** %s\n", p.K8sVersion)
	fmt.Fprintf(&sb, "- **Distribution:** %s\n", p.Distribution)
	fmt.Fprintf(&sb, "- **Nodes:** %d\n", len(p.Nodes))

	// Runtime
	fmt.Fprintf(&sb, "\n## Runtime\n")
	if p.GVisor {
		fmt.Fprintf(&sb, "- **gVisor:** available ✓\n")
	} else {
		fmt.Fprintf(&sb, "- **gVisor:** not available\n")
	}
	if len(p.RuntimeClasses) == 0 {
		fmt.Fprintf(&sb, "- **RuntimeClasses:** none\n")
	} else {
		rcs := make([]string, 0, len(p.RuntimeClasses))
		for _, rc := range p.RuntimeClasses {
			rcs = append(rcs, fmt.Sprintf("%s (%s)", rc.Name, rc.Handler))
		}
		fmt.Fprintf(&sb, "- **RuntimeClasses:** %s\n", strings.Join(rcs, ", "))
	}

	// Networking
	fmt.Fprintf(&sb, "\n## Networking\n")
	fmt.Fprintf(&sb, "- **CNI:** %s\n", p.CNI.Name)
	npStatus := "not supported"
	if p.NetworkPolicy.Supported {
		if p.NetworkPolicy.InUse {
			npStatus = "supported, in use"
		} else {
			npStatus = "supported, none deployed"
		}
	}
	fmt.Fprintf(&sb, "- **NetworkPolicy:** %s\n", npStatus)
	if p.CNI.EgressSupported {
		fmt.Fprintf(&sb, "- **Egress control:** supported\n")
	}
	if len(p.Ingress) > 0 {
		fmt.Fprintf(&sb, "- **Ingress:** %s\n", strings.Join(p.Ingress, ", "))
	}

	// Storage
	fmt.Fprintf(&sb, "\n## Storage\n")
	if len(p.StorageClasses) == 0 {
		fmt.Fprintf(&sb, "No StorageClasses found.\n")
	} else {
		fmt.Fprintf(&sb, "| Name | Provisioner | Default | Reclaim | RWX (inferred) |\n")
		fmt.Fprintf(&sb, "|------|-------------|---------|---------|----------------|\n")
		for _, sc := range p.StorageClasses {
			def := "✗"
			if sc.IsDefault {
				def = "✓"
			}
			rwx := "✗"
			if sc.RWXCapable {
				rwx = "✓"
			}
			fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n",
				sc.Name, sc.Provisioner, def, sc.ReclaimPolicy, rwx)
		}
	}

	// Extensions
	fmt.Fprintf(&sb, "\n## Extensions\n")
	ext := p.Extensions
	checkmark := func(v bool) string {
		if v {
			return "✓"
		}
		return "✗"
	}
	fmt.Fprintf(&sb, "- %s Istio\n", checkmark(ext.Istio))
	fmt.Fprintf(&sb, "- %s cert-manager\n", checkmark(ext.CertManager))
	fmt.Fprintf(&sb, "- %s Prometheus Operator\n", checkmark(ext.PrometheusOp))
	fmt.Fprintf(&sb, "- %s External Secrets\n", checkmark(ext.ExternalSecrets))
	fmt.Fprintf(&sb, "- %s ArgoCD\n", checkmark(ext.ArgoCD))
	fmt.Fprintf(&sb, "- %s Gateway API\n", checkmark(ext.GatewayAPI))
	fmt.Fprintf(&sb, "- %s Metrics Server\n", checkmark(ext.MetricsServer))
	if len(ext.OtherCRDGroups) > 0 {
		fmt.Fprintf(&sb, "- Other CRD groups: %s\n", strings.Join(ext.OtherCRDGroups, ", "))
	}

	// Exoskeleton
	if p.Exoskeleton.Enabled {
		fmt.Fprintf(&sb, "\n## Exoskeleton Services\n")
		if len(p.Exoskeleton.Services) > 0 {
			for _, svc := range p.Exoskeleton.Services {
				fmt.Fprintf(&sb, "- %s\n", svc)
			}
		} else {
			fmt.Fprintf(&sb, "Enabled but no services detected.\n")
		}
		if p.Exoskeleton.CleanupOnUndeploy {
			fmt.Fprintf(&sb, "- **Cleanup on undeploy:** enabled\n")
		}
	}

	// Namespace
	fmt.Fprintf(&sb, "\n## Namespace: %s\n", p.Namespace)
	fmt.Fprintf(&sb, "- **Pod Security:** %s\n", p.PodSecurity)
	if p.Quota != nil {
		if p.Quota.CPULimit != "" {
			fmt.Fprintf(&sb, "- **CPU quota:** request %s / limit %s\n", p.Quota.CPURequest, p.Quota.CPULimit)
		}
		if p.Quota.MemoryLimit != "" {
			fmt.Fprintf(&sb, "- **Memory quota:** request %s / limit %s\n", p.Quota.MemoryRequest, p.Quota.MemoryLimit)
		}
		if p.Quota.MaxPods > 0 {
			fmt.Fprintf(&sb, "- **Max pods:** %d\n", p.Quota.MaxPods)
		}
	}
	if p.LimitRange != nil {
		fmt.Fprintf(&sb, "- **Default CPU:** request %s / limit %s\n",
			p.LimitRange.DefaultCPURequest, p.LimitRange.DefaultCPULimit)
		fmt.Fprintf(&sb, "- **Default Memory:** request %s / limit %s\n",
			p.LimitRange.DefaultMemoryRequest, p.LimitRange.DefaultMemoryLimit)
	}

	// Agent Guidance
	if len(p.Guidance) > 0 {
		fmt.Fprintf(&sb, "\n## Agent Guidance\n")
		for i, g := range p.Guidance {
			fmt.Fprintf(&sb, "%d. %s\n", i+1, g)
		}
	}

	return sb.String()
}

// JSON renders the profile as indented JSON.
func (p *ClusterProfile) JSON() string {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal profile: %s"}`, err)
	}
	return string(data)
}
