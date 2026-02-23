package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ClusterProfile is a point-in-time capability snapshot of a target cluster environment.
// Written to .tentacular/envprofiles/<env>.md and <env>.json.
type ClusterProfile struct {
	GeneratedAt    time.Time          `json:"generatedAt"          yaml:"generatedAt"`
	Environment    string             `json:"environment"          yaml:"environment"`
	K8sVersion     string             `json:"k8sVersion"           yaml:"k8sVersion"`
	Distribution   string             `json:"distribution"         yaml:"distribution"`
	Nodes          []NodeInfo         `json:"nodes"                yaml:"nodes"`
	RuntimeClasses []RuntimeClassInfo `json:"runtimeClasses"       yaml:"runtimeClasses"`
	GVisor         bool               `json:"gvisor"               yaml:"gvisor"`
	CNI            CNIInfo            `json:"cni"                  yaml:"cni"`
	NetworkPolicy  NetPolInfo         `json:"networkPolicy"        yaml:"networkPolicy"`
	StorageClasses []StorageClassInfo `json:"storageClasses"       yaml:"storageClasses"`
	CSIDrivers     []string           `json:"csiDrivers"           yaml:"csiDrivers"`
	Ingress        []string           `json:"ingress"              yaml:"ingress"`
	Extensions     ExtensionSet       `json:"extensions"           yaml:"extensions"`
	Namespace      string             `json:"namespace"            yaml:"namespace"`
	Quota          *QuotaSummary      `json:"quota,omitempty"      yaml:"quota,omitempty"`
	LimitRange     *LimitRangeSummary `json:"limitRange,omitempty" yaml:"limitRange,omitempty"`
	PodSecurity    string             `json:"podSecurity"          yaml:"podSecurity"`
	Guidance       []string           `json:"guidance"             yaml:"guidance"`
}

// NodeInfo summarises a single cluster node.
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
type StorageClassInfo struct {
	Name                 string `json:"name"                 yaml:"name"`
	Provisioner          string `json:"provisioner"          yaml:"provisioner"`
	IsDefault            bool   `json:"isDefault"            yaml:"isDefault"`
	ReclaimPolicy        string `json:"reclaimPolicy"        yaml:"reclaimPolicy"`
	AllowVolumeExpansion bool   `json:"allowVolumeExpansion" yaml:"allowVolumeExpansion"`
	RWXCapable           bool   `json:"rwxCapable"           yaml:"rwxCapable"`
}

// ExtensionSet records which well-known CRD-based extensions are installed.
type ExtensionSet struct {
	Istio           bool     `json:"istio"           yaml:"istio"`
	CertManager     bool     `json:"certManager"     yaml:"certManager"`
	PrometheusOp    bool     `json:"prometheusOp"    yaml:"prometheusOp"`
	ExternalSecrets bool     `json:"externalSecrets" yaml:"externalSecrets"`
	ArgoCD          bool     `json:"argoCD"          yaml:"argoCD"`
	GatewayAPI      bool     `json:"gatewayAPI"      yaml:"gatewayAPI"`
	MetricsServer   bool     `json:"metricsServer"   yaml:"metricsServer"`
	OtherCRDGroups  []string `json:"otherCRDGroups"  yaml:"otherCRDGroups"`
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

// Profile collects a capability snapshot for the given namespace and environment name.
// The envName is used only for labelling the profile — it does not affect cluster access,
// which is already established via the Client's kubeconfig/context.
func (c *Client) Profile(ctx context.Context, namespace, envName string) (*ClusterProfile, error) {
	p := &ClusterProfile{
		GeneratedAt: time.Now().UTC(),
		Environment: envName,
		Namespace:   namespace,
	}

	// K8s server version
	sv, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("fetching server version: %w", err)
	}
	p.K8sVersion = sv.GitVersion

	// Nodes + distribution
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	p.Distribution = detectDistribution(nodes)
	for _, n := range nodes.Items {
		ni := NodeInfo{
			Name:   n.Name,
			Arch:   n.Status.NodeInfo.Architecture,
			OS:     n.Status.NodeInfo.OperatingSystem,
			Labels: n.Labels,
		}
		for _, t := range n.Spec.Taints {
			ni.Taints = append(ni.Taints, fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect))
		}
		p.Nodes = append(p.Nodes, ni)
	}

	// RuntimeClasses
	rcs, err := c.clientset.NodeV1().RuntimeClasses().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, rc := range rcs.Items {
			p.RuntimeClasses = append(p.RuntimeClasses, RuntimeClassInfo{
				Name:    rc.Name,
				Handler: rc.Handler,
			})
			if rc.Name == "gvisor" || strings.Contains(rc.Handler, "runsc") {
				p.GVisor = true
			}
		}
	}

	// StorageClasses + CSI drivers
	scs, err := c.clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, sc := range scs.Items {
			isDefault := sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true"
			reclaimPolicy := "Delete"
			if sc.ReclaimPolicy != nil {
				reclaimPolicy = string(*sc.ReclaimPolicy)
			}
			allowExpand := false
			if sc.AllowVolumeExpansion != nil {
				allowExpand = *sc.AllowVolumeExpansion
			}
			p.StorageClasses = append(p.StorageClasses, StorageClassInfo{
				Name:                 sc.Name,
				Provisioner:          sc.Provisioner,
				IsDefault:            isDefault,
				ReclaimPolicy:        reclaimPolicy,
				AllowVolumeExpansion: allowExpand,
				RWXCapable:           isRWXCapable(sc.Provisioner),
			})
		}
	}
	csiDrivers, err := c.clientset.StorageV1().CSIDrivers().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, d := range csiDrivers.Items {
			p.CSIDrivers = append(p.CSIDrivers, d.Name)
		}
	}

	// CNI (via kube-system pods)
	ksPods, err := c.clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err == nil {
		p.CNI = detectCNI(ksPods)
	}

	// NetworkPolicy support + usage
	netpols, err := c.clientset.NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{})
	if err == nil {
		p.NetworkPolicy = NetPolInfo{
			Supported: p.CNI.NetworkPolicySupported,
			InUse:     len(netpols.Items) > 0,
		}
	} else {
		p.NetworkPolicy = NetPolInfo{Supported: p.CNI.NetworkPolicySupported}
	}

	// Ingress controllers (all namespaces)
	allPods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err == nil {
		p.Ingress = detectIngress(allPods)
	}

	// CRD-based extensions
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	crdList, err := c.dynamic.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		p.Extensions = classifyExtensions(crdList)
	}

	// Metrics server (kube-system)
	if err == nil {
		msPods, _ := c.clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=metrics-server",
		})
		if msPods != nil && len(msPods.Items) > 0 {
			p.Extensions.MetricsServer = true
		}
	}

	// Resource quotas
	quotas, err := c.clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err == nil && len(quotas.Items) > 0 {
		p.Quota = summariseQuotas(quotas.Items)
	}

	// LimitRanges
	limits, err := c.clientset.CoreV1().LimitRanges(namespace).List(ctx, metav1.ListOptions{})
	if err == nil && len(limits.Items) > 0 {
		p.LimitRange = summariseLimitRanges(limits.Items)
	}

	// Pod Security Admission
	ns, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		p.PodSecurity = detectPodSecurity(ns)
	} else {
		p.PodSecurity = "unknown"
	}

	// Agent guidance
	p.Guidance = deriveGuidance(p)

	return p, nil
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

func detectIngress(pods *corev1.PodList) []string {
	seen := map[string]bool{}
	for _, pod := range pods.Items {
		labels := pod.Labels
		name := pod.Name
		appName := labels["app.kubernetes.io/name"]
		app := labels["app"]
		switch {
		case appName == "ingress-nginx" || strings.Contains(name, "ingress-nginx"):
			seen["nginx"] = true
		case appName == "traefik" || app == "traefik":
			seen["traefik"] = true
		case app == "istio-ingressgateway" || strings.Contains(name, "istio-ingressgateway"):
			seen["istio"] = true
		case appName == "contour" || app == "contour":
			seen["contour"] = true
		}
	}
	var result []string
	for k := range seen {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
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
	return ext
}

func summariseQuotas(quotas []corev1.ResourceQuota) *QuotaSummary {
	qs := &QuotaSummary{}
	for _, q := range quotas {
		hard := q.Spec.Hard
		if v, ok := hard[corev1.ResourceRequestsCPU]; ok {
			qs.CPURequest = v.String()
		}
		if v, ok := hard[corev1.ResourceLimitsCPU]; ok {
			qs.CPULimit = v.String()
		}
		if v, ok := hard[corev1.ResourceRequestsMemory]; ok {
			qs.MemoryRequest = v.String()
		}
		if v, ok := hard[corev1.ResourceLimitsMemory]; ok {
			qs.MemoryLimit = v.String()
		}
		if v, ok := hard[corev1.ResourcePods]; ok {
			pods, _ := v.AsInt64()
			qs.MaxPods = int(pods)
		}
	}
	return qs
}

func summariseLimitRanges(lrs []corev1.LimitRange) *LimitRangeSummary {
	lr := &LimitRangeSummary{}
	for _, r := range lrs {
		for _, item := range r.Spec.Limits {
			if item.Type == corev1.LimitTypeContainer {
				if v, ok := item.Default[corev1.ResourceCPU]; ok {
					lr.DefaultCPULimit = v.String()
				}
				if v, ok := item.DefaultRequest[corev1.ResourceCPU]; ok {
					lr.DefaultCPURequest = v.String()
				}
				if v, ok := item.Default[corev1.ResourceMemory]; ok {
					lr.DefaultMemoryLimit = v.String()
				}
				if v, ok := item.DefaultRequest[corev1.ResourceMemory]; ok {
					lr.DefaultMemoryRequest = v.String()
				}
			}
		}
	}
	return lr
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
			g = append(g, fmt.Sprintf("RWX storage available via StorageClass %q — suitable for shared volume mounts", sc.Name))
		}
	}
	if !hasRWX {
		g = append(g, "No RWX-capable StorageClass detected — avoid shared volume mounts across replicas")
	}

	if p.Quota != nil {
		g = append(g, fmt.Sprintf("ResourceQuota active in namespace %q: CPU limit %s, memory limit %s",
			p.Namespace, p.Quota.CPULimit, p.Quota.MemoryLimit))
	}

	if p.PodSecurity == "restricted" {
		g = append(g, "Namespace enforces restricted PodSecurity — containers must run as non-root with no privilege escalation")
	}

	if p.Extensions.CertManager {
		g = append(g, "cert-manager available — TLS certificates can be provisioned automatically")
	}

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
		var rcs []string
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
		fmt.Fprintf(&sb, "| Name | Provisioner | Default | Reclaim | RWX |\n")
		fmt.Fprintf(&sb, "|------|-------------|---------|---------|-----|\n")
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
