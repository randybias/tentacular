# cluster-profile — Technical Design

## Data Model

```go
// ClusterProfile is a point-in-time capability snapshot of a target environment.
// It is written to .tentacular/envprofiles/<env>.md (markdown) and
// .tentacular/envprofiles/<env>.json (JSON sidecar).
type ClusterProfile struct {
    GeneratedAt    time.Time        `json:"generatedAt"    yaml:"generatedAt"`
    Environment    string           `json:"environment"    yaml:"environment"`
    K8sVersion     string           `json:"k8sVersion"     yaml:"k8sVersion"`
    Distribution   string           `json:"distribution"   yaml:"distribution"` // eks|gke|aks|kind|k3s|vanilla
    Nodes          []NodeInfo       `json:"nodes"          yaml:"nodes"`
    RuntimeClasses []RuntimeClass   `json:"runtimeClasses" yaml:"runtimeClasses"`
    GVisor         bool             `json:"gvisor"         yaml:"gvisor"`
    CNI            CNIInfo          `json:"cni"            yaml:"cni"`
    NetworkPolicy  NetPolInfo       `json:"networkPolicy"  yaml:"networkPolicy"`
    StorageClasses []StorageClass   `json:"storageClasses" yaml:"storageClasses"`
    CSIDrivers     []string         `json:"csiDrivers"     yaml:"csiDrivers"`
    Ingress        []string         `json:"ingress"        yaml:"ingress"`
    Extensions     ExtensionSet     `json:"extensions"     yaml:"extensions"`
    Namespace      string           `json:"namespace"      yaml:"namespace"`
    Quota          *QuotaSummary    `json:"quota,omitempty"     yaml:"quota,omitempty"`
    LimitRange     *LimitRangeSummary `json:"limitRange,omitempty" yaml:"limitRange,omitempty"`
    PodSecurity    string           `json:"podSecurity"    yaml:"podSecurity"` // restricted|baseline|privileged|unknown
    Guidance       []string         `json:"guidance"       yaml:"guidance"`
}

type NodeInfo struct {
    Name   string            `json:"name"   yaml:"name"`
    Arch   string            `json:"arch"   yaml:"arch"`   // amd64|arm64
    OS     string            `json:"os"     yaml:"os"`
    Labels map[string]string `json:"labels" yaml:"labels"`
    Taints []string          `json:"taints" yaml:"taints"`
}

type RuntimeClass struct {
    Name    string `json:"name"    yaml:"name"`
    Handler string `json:"handler" yaml:"handler"`
}

type CNIInfo struct {
    Name                  string `json:"name"                  yaml:"name"`    // calico|cilium|flannel|weave|kindnet|unknown
    Version               string `json:"version,omitempty"     yaml:"version,omitempty"`
    NetworkPolicySupported bool   `json:"networkPolicySupported" yaml:"networkPolicySupported"`
    EgressSupported        bool   `json:"egressSupported"        yaml:"egressSupported"`
}

type NetPolInfo struct {
    Supported bool `json:"supported" yaml:"supported"`
    InUse     bool `json:"inUse"     yaml:"inUse"`
}

type StorageClass struct {
    Name                 string `json:"name"                 yaml:"name"`
    Provisioner          string `json:"provisioner"          yaml:"provisioner"`
    IsDefault            bool   `json:"isDefault"            yaml:"isDefault"`
    ReclaimPolicy        string `json:"reclaimPolicy"        yaml:"reclaimPolicy"`
    AllowVolumeExpansion bool   `json:"allowVolumeExpansion" yaml:"allowVolumeExpansion"`
    AccessModes          string `json:"accessModes"          yaml:"accessModes"` // RWO|RWX (inferred from provisioner)
}

type ExtensionSet struct {
    Istio           bool     `json:"istio"            yaml:"istio"`
    CertManager     bool     `json:"certManager"      yaml:"certManager"`
    PrometheusOp    bool     `json:"prometheusOp"     yaml:"prometheusOp"`
    ExternalSecrets bool     `json:"externalSecrets"  yaml:"externalSecrets"`
    ArgoCD          bool     `json:"argoCD"           yaml:"argoCD"`
    GatewayAPI      bool     `json:"gatewayAPI"       yaml:"gatewayAPI"`
    MetricsServer   bool     `json:"metricsServer"    yaml:"metricsServer"`
    OtherCRDGroups  []string `json:"otherCRDGroups"   yaml:"otherCRDGroups"`
}

type QuotaSummary struct {
    CPURequest    string `json:"cpuRequest"    yaml:"cpuRequest"`
    CPULimit      string `json:"cpuLimit"      yaml:"cpuLimit"`
    MemoryRequest string `json:"memoryRequest" yaml:"memoryRequest"`
    MemoryLimit   string `json:"memoryLimit"   yaml:"memoryLimit"`
    MaxPods       int    `json:"maxPods"       yaml:"maxPods"`
}

type LimitRangeSummary struct {
    DefaultCPURequest    string `json:"defaultCPURequest"    yaml:"defaultCPURequest"`
    DefaultCPULimit      string `json:"defaultCPULimit"      yaml:"defaultCPULimit"`
    DefaultMemoryRequest string `json:"defaultMemoryRequest" yaml:"defaultMemoryRequest"`
    DefaultMemoryLimit   string `json:"defaultMemoryLimit"   yaml:"defaultMemoryLimit"`
}
```

## Discovery Logic

### K8s Version & Distribution

```
ServerVersion()                   → K8sVersion
Node labels:
  eks.amazonaws.com/nodegroup      → "eks"
  cloud.google.com/gke-nodepool    → "gke"
  kubernetes.azure.com/agentpool   → "aks"
  node.kubernetes.io/instance-type=k3s.io  → "k3s"
kubeconfig context prefix "kind-" → "kind"
fallthrough                        → "vanilla"
```

### CNI Detection (kube-system pods by label)

```
k8s-app=calico-node                 → calico  (NetworkPolicy: true, Egress: true)
k8s-app=cilium                      → cilium  (NetworkPolicy: true, Egress: true)
pod name contains "flannel"         → flannel (NetworkPolicy: false, Egress: false)
pod name contains "weave"           → weave   (NetworkPolicy: true,  Egress: true)
pod name contains "kindnet"         → kindnet (NetworkPolicy: false, Egress: false)
fallthrough                         → unknown (NetworkPolicy: unknown)
```

NetworkPolicy.InUse: List all NetworkPolicies cluster-wide; InUse = count > 0.

### CSI Drivers

`StorageV1().CSIDrivers().List()` — names only.

### StorageClass Access Mode (inferred)

```
provisioner contains "efs" or "nfs" or "azureFile" or "cephfs" → RWX possible
provisioner contains "ebs" or "gce-pd" or "azure-disk"         → RWO only
default                                                         → RWO
```

### RuntimeClass

`NodeV1().RuntimeClasses().List()` — name + handler. GVisor = any entry with name "gvisor"
or handler containing "runsc".

### Ingress Controllers (pods, all namespaces)

```
app.kubernetes.io/name=ingress-nginx  → "nginx"
app=traefik or app.kubernetes.io/name=traefik → "traefik"
app=istio-ingressgateway              → "istio"
app.kubernetes.io/name=contour        → "contour"
```

### Extensions (CRD groups)

List `apiextensions.k8s.io/v1` CustomResourceDefinitions via dynamic client.
Map known CRD group suffixes:

```
*.istio.io                           → Istio
cert-manager.io                      → CertManager
monitoring.coreos.com                → PrometheusOp
external-secrets.io                  → ExternalSecrets
argoproj.io                          → ArgoCD
gateway.networking.k8s.io            → GatewayAPI
```

Metrics Server: List pods in kube-system with `k8s-app=metrics-server`.

All unmatched CRD groups → OtherCRDGroups (deduplicated, sorted).

### Pod Security Admission

Read namespace labels:
```
pod-security.kubernetes.io/enforce=restricted  → "restricted"
pod-security.kubernetes.io/enforce=baseline    → "baseline"
pod-security.kubernetes.io/enforce=privileged  → "privileged"
(no label)                                     → "unknown"
```

### Guidance Derivation

The `Guidance` slice is generated post-collection as human-readable strings for the AI agent:

```
GVisor available         → "Use runtime_class: gvisor for untrusted workflow steps"
GVisor unavailable       → "gVisor not available — omit runtime_class or use ''"
Istio present            → "Istio detected: NetworkPolicy egress to istio-system required; mTLS available between pods"
NetworkPolicy not InUse  → "NetworkPolicy supported but none found — generated policies will be the first; test carefully"
No RWX storage           → "No RWX-capable StorageClass detected — avoid shared volume mounts across replicas"
Quota present            → "ResourceQuota active in namespace <ns>: CPU limit <x>, memory limit <y>"
PSA restricted           → "Namespace enforces restricted PodSecurity — containers must be non-root, no privilege escalation"
Kind distribution        → "kind cluster detected: disable gVisor (runtime_class: ''), use imagePullPolicy: IfNotPresent"
```

## CLI Shape

```
tntc cluster profile [flags]

Flags:
  --env string      Environment name from .tentacular/config.yaml (default: current context)
  --all             Profile all environments defined in config
  --output string   Output format: markdown|json (default: markdown)
  --save            Write profiles to .tentacular/envprofiles/<env>.{md,json}
  --force           Re-profile even if a fresh profile exists (< 1h old)
```

`--env` resolves via `ResolveEnvironment()` — same resolution as `tntc deploy --env`.

Without `--save`, output goes to stdout. With `--save`, output is written to files and a
summary is printed to stdout.

Exit codes: 0 = success, 1 = cannot reach cluster (non-fatal when using `--all`).

## Profile Storage

```
.tentacular/
  config.yaml
  envprofiles/
    dev.md
    dev.json
    staging.md
    staging.json
    prod.md
    prod.json
```

Recommended `.gitignore` entry: none by default (teams should commit profiles alongside
config so agents have context without running against live clusters). Add to `.gitignore`
only if profiles contain sensitive cluster details.

## Auto-Profile on Configure

`tntc configure` writes config then calls `runClusterProfile` in a best-effort goroutine
(or sequentially with a timeout) for each environment in the written config. If the cluster
is unreachable, the error is printed as a warning and `configure` exits 0.

Message: `Profiling environment 'prod'... saved to .tentacular/envprofiles/prod.md`

## Drift Detection (Agent-side, in SKILL.md)

The SKILL.md instructs the AI agent to trigger `tntc cluster profile --env <name> --save`
when it observes any of:

- Deployment fails with `unknown RuntimeClass`
- NetworkPolicy blocks traffic that the profile says should be allowed
- PVC fails to bind and profile shows the provisioner should be available
- `tntc cluster check` passes but deploy produces unexpected resource errors
- Agent is asked to build a workflow for an environment with no profile file
- Profile `generatedAt` is older than 7 days
- Cluster version in profile differs from `tntc cluster check` output

## Markdown Output Format

```markdown
# Cluster Environment Profile: prod
Generated: 2026-02-23T09:00:00Z | K8s: v1.29.3 | Distribution: eks

## Identity
- **Version:** v1.29.3
- **Distribution:** eks
- **Nodes:** 6 (amd64)

## Runtime
- **gVisor:** available ✓
- **RuntimeClasses:** gvisor (runsc), kata (kata-qemu)

## Networking (CNI)
- **CNI:** Calico
- **NetworkPolicy:** supported, in use (12 policies cluster-wide)
- **Egress control:** supported
- **Ingress:** istio-ingressgateway

## Storage
| Name | Provisioner | Default | Reclaim | RWX |
|------|-------------|---------|---------|-----|
| gp3 | ebs.csi.aws.com | ✓ | Delete | ✗ |
| efs-sc | efs.csi.aws.com | ✗ | Retain | ✓ |

## Extensions
- ✓ Istio (v1.20)
- ✓ cert-manager
- ✓ Prometheus Operator
- ✗ External Secrets
- ✗ ArgoCD
- ✓ Gateway API
- ✓ Metrics Server

## Namespace: tentacular
- **Pod Security:** restricted
- **CPU quota:** request 10 / limit 20
- **Memory quota:** request 20Gi / limit 40Gi

## Agent Guidance
1. Use `runtime_class: gvisor` for untrusted workflow steps
2. Istio detected: NetworkPolicy egress must include `namespaceSelector: istio-system`
3. Namespace enforces restricted PodSecurity — containers must run as non-root
4. RWX storage available via `efs-sc` for shared volumes
```
