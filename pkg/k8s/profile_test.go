package k8s

import (
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// --- detectDistribution ---

func TestDetectDistribution_EKS(t *testing.T) {
	nodes := &corev1.NodeList{Items: []corev1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"eks.amazonaws.com/nodegroup": "workers"},
		},
	}}}
	if got := detectDistribution(nodes); got != "eks" {
		t.Errorf("expected eks, got %s", got)
	}
}

func TestDetectDistribution_GKE(t *testing.T) {
	nodes := &corev1.NodeList{Items: []corev1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"cloud.google.com/gke-nodepool": "default-pool"},
		},
	}}}
	if got := detectDistribution(nodes); got != "gke" {
		t.Errorf("expected gke, got %s", got)
	}
}

func TestDetectDistribution_AKS(t *testing.T) {
	nodes := &corev1.NodeList{Items: []corev1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"kubernetes.azure.com/agentpool": "agentpool"},
		},
	}}}
	if got := detectDistribution(nodes); got != "aks" {
		t.Errorf("expected aks, got %s", got)
	}
}

func TestDetectDistribution_Vanilla(t *testing.T) {
	nodes := &corev1.NodeList{Items: []corev1.Node{{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}},
	}}}
	if got := detectDistribution(nodes); got != "vanilla" {
		t.Errorf("expected vanilla, got %s", got)
	}
}

// --- detectCNI ---

func TestDetectCNI_Calico(t *testing.T) {
	pods := &corev1.PodList{Items: []corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "calico-node-abc",
			Labels: map[string]string{"k8s-app": "calico-node"},
		},
	}}}
	cni := detectCNI(pods)
	if cni.Name != "calico" {
		t.Errorf("expected calico, got %s", cni.Name)
	}
	if !cni.NetworkPolicySupported {
		t.Error("expected NetworkPolicy supported for calico")
	}
	if !cni.EgressSupported {
		t.Error("expected egress supported for calico")
	}
}

func TestDetectCNI_Cilium(t *testing.T) {
	pods := &corev1.PodList{Items: []corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cilium-xyz",
			Labels: map[string]string{"k8s-app": "cilium"},
		},
	}}}
	cni := detectCNI(pods)
	if cni.Name != "cilium" {
		t.Errorf("expected cilium, got %s", cni.Name)
	}
}

func TestDetectCNI_Flannel(t *testing.T) {
	pods := &corev1.PodList{Items: []corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "kube-flannel-ds-abc",
			Labels: map[string]string{},
		},
	}}}
	cni := detectCNI(pods)
	if cni.Name != "flannel" {
		t.Errorf("expected flannel, got %s", cni.Name)
	}
	if cni.NetworkPolicySupported {
		t.Error("flannel should not support NetworkPolicy")
	}
}

func TestDetectCNI_Kindnet(t *testing.T) {
	pods := &corev1.PodList{Items: []corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "kindnet-abc",
			Labels: map[string]string{},
		},
	}}}
	cni := detectCNI(pods)
	if cni.Name != "kindnet" {
		t.Errorf("expected kindnet, got %s", cni.Name)
	}
}

func TestDetectCNI_Unknown(t *testing.T) {
	pods := &corev1.PodList{Items: []corev1.Pod{}}
	cni := detectCNI(pods)
	if cni.Name != "unknown" {
		t.Errorf("expected unknown, got %s", cni.Name)
	}
}

// --- isRWXCapable ---

func TestIsRWXCapable(t *testing.T) {
	cases := []struct {
		provisioner string
		want        bool
	}{
		{"efs.csi.aws.com", true},
		{"nfs.k8s.io", true},
		{"ebs.csi.aws.com", false},
		{"pd.csi.storage.gke.io", false},
		{"kubernetes.io/azure-file", true},  // contains "azure-file"
		{"file.csi.azure.com", false},       // Azure Files CSI — not matched by current heuristic
	}
	for _, c := range cases {
		if got := isRWXCapable(c.provisioner); got != c.want {
			t.Errorf("isRWXCapable(%q) = %v, want %v", c.provisioner, got, c.want)
		}
	}
}

// --- classifyExtensions ---

func TestClassifyExtensions_Istio(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			makeUnstructured("virtualservices.networking.istio.io"),
			makeUnstructured("destinationrules.networking.istio.io"),
		},
	}
	ext := classifyExtensions(list)
	if !ext.Istio {
		t.Error("expected Istio = true")
	}
}

func TestClassifyExtensions_CertManager(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			makeUnstructured("certificates.cert-manager.io"),
		},
	}
	ext := classifyExtensions(list)
	if !ext.CertManager {
		t.Error("expected CertManager = true")
	}
}

func TestClassifyExtensions_PrometheusOp(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			makeUnstructured("servicemonitors.monitoring.coreos.com"),
		},
	}
	ext := classifyExtensions(list)
	if !ext.PrometheusOp {
		t.Error("expected PrometheusOp = true")
	}
}

func TestClassifyExtensions_OtherGroups(t *testing.T) {
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			makeUnstructured("foos.custom.example.com"),
			makeUnstructured("bars.custom.example.com"),
			makeUnstructured("bazzes.other.io"),
		},
	}
	ext := classifyExtensions(list)
	if len(ext.OtherCRDGroups) != 2 {
		t.Errorf("expected 2 other groups, got %d: %v", len(ext.OtherCRDGroups), ext.OtherCRDGroups)
	}
}

func TestClassifyExtensions_OtherGroupsCap(t *testing.T) {
	// Build a list with 25 distinct unknown CRD groups — should be capped at 20 + a summary entry.
	items := make([]unstructured.Unstructured, 25)
	for i := range items {
		items[i] = makeUnstructured(fmt.Sprintf("resource%d.group%d.example.com", i, i))
	}
	list := &unstructured.UnstructuredList{Items: items}
	ext := classifyExtensions(list)
	if len(ext.OtherCRDGroups) != 21 { // 20 entries + 1 "... and N more" entry
		t.Errorf("expected 21 entries (20 + summary), got %d: %v", len(ext.OtherCRDGroups), ext.OtherCRDGroups)
	}
	last := ext.OtherCRDGroups[20]
	if !strings.Contains(last, "and 5 more") {
		t.Errorf("expected truncation summary in last entry, got %q", last)
	}
}

func TestClassifyExtensions_Empty(t *testing.T) {
	list := &unstructured.UnstructuredList{}
	ext := classifyExtensions(list)
	if ext.Istio || ext.CertManager || ext.PrometheusOp {
		t.Error("expected all extensions false for empty list")
	}
}

// --- detectPodSecurity ---

func TestDetectPodSecurity_Restricted(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"pod-security.kubernetes.io/enforce": "restricted"},
		},
	}
	if got := detectPodSecurity(ns); got != "restricted" {
		t.Errorf("expected restricted, got %s", got)
	}
}

func TestDetectPodSecurity_Unknown(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{}}
	if got := detectPodSecurity(ns); got != "unknown" {
		t.Errorf("expected unknown, got %s", got)
	}
}

func TestDetectPodSecurity_Nil(t *testing.T) {
	if got := detectPodSecurity(nil); got != "unknown" {
		t.Errorf("expected unknown for nil, got %s", got)
	}
}

// --- deriveGuidance ---

func TestDeriveGuidance_GVisor(t *testing.T) {
	p := &ClusterProfile{GVisor: true}
	g := deriveGuidance(p)
	if !containsSubstr(g, "runtime_class: gvisor") {
		t.Errorf("expected gvisor guidance, got: %v", g)
	}
}

func TestDeriveGuidance_NoGVisor(t *testing.T) {
	p := &ClusterProfile{GVisor: false}
	g := deriveGuidance(p)
	if !containsSubstr(g, "gVisor not available") {
		t.Errorf("expected no-gvisor guidance, got: %v", g)
	}
}

func TestDeriveGuidance_KindCluster(t *testing.T) {
	p := &ClusterProfile{Distribution: "kind"}
	g := deriveGuidance(p)
	if !containsSubstr(g, "kind cluster") {
		t.Errorf("expected kind guidance, got: %v", g)
	}
}

func TestDeriveGuidance_Istio(t *testing.T) {
	p := &ClusterProfile{Extensions: ExtensionSet{Istio: true}}
	g := deriveGuidance(p)
	if !containsSubstr(g, "Istio detected") {
		t.Errorf("expected istio guidance, got: %v", g)
	}
}

func TestDeriveGuidance_RestrictedPSA(t *testing.T) {
	p := &ClusterProfile{PodSecurity: "restricted"}
	g := deriveGuidance(p)
	if !containsSubstr(g, "restricted PodSecurity") {
		t.Errorf("expected PSA guidance, got: %v", g)
	}
}

func TestDeriveGuidance_Quota(t *testing.T) {
	p := &ClusterProfile{
		Namespace: "prod",
		Quota:     &QuotaSummary{CPULimit: "20", MemoryLimit: "40Gi"},
	}
	g := deriveGuidance(p)
	if !containsSubstr(g, "ResourceQuota active") {
		t.Errorf("expected quota guidance, got: %v", g)
	}
}

// --- Markdown output ---

func TestMarkdown_ContainsSections(t *testing.T) {
	p := &ClusterProfile{
		GeneratedAt:  time.Now(),
		Environment:  "test",
		K8sVersion:   "v1.29.0",
		Distribution: "vanilla",
		GVisor:       true,
		CNI:          CNIInfo{Name: "calico", NetworkPolicySupported: true},
		NetworkPolicy: NetPolInfo{Supported: true, InUse: true},
		PodSecurity:  "unknown",
		Guidance:     []string{"Use runtime_class: gvisor for untrusted workflow steps"},
	}
	md := p.Markdown()
	for _, want := range []string{
		"# Cluster Environment Profile: test",
		"## Identity",
		"## Runtime",
		"## Networking",
		"## Storage",
		"## Extensions",
		"## Namespace",
		"## Agent Guidance",
		"gvisor",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestMarkdown_GVisorAvailable(t *testing.T) {
	p := &ClusterProfile{GVisor: true}
	if !strings.Contains(p.Markdown(), "available ✓") {
		t.Error("expected gVisor available marker")
	}
}

func TestMarkdown_GVisorUnavailable(t *testing.T) {
	p := &ClusterProfile{GVisor: false}
	if !strings.Contains(p.Markdown(), "not available") {
		t.Error("expected gVisor not available marker")
	}
}

// --- helpers ---

func makeUnstructured(name string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetName(name)
	return u
}

func containsSubstr(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
