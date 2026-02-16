## ADDED Requirements

### Requirement: Auto-detect kind clusters from kubeconfig
`DetectKindCluster()` SHALL read the current kubeconfig context and identify kind clusters. A cluster is kind when the context name has a `kind-` prefix AND the cluster server URL contains `127.0.0.1`, `localhost`, or `[::1]`.

#### Scenario: Detect kind cluster
- **WHEN** the current kubeconfig context is `kind-dev` with server `https://127.0.0.1:6443`
- **THEN** `DetectKindCluster()` SHALL return `ClusterInfo{IsKind: true, ClusterName: "dev", Context: "kind-dev"}`

#### Scenario: Non-kind cluster
- **WHEN** the current kubeconfig context is `production` with server `https://k8s.example.com:6443`
- **THEN** `DetectKindCluster()` SHALL return `ClusterInfo{IsKind: false}`

#### Scenario: No kubeconfig
- **WHEN** no kubeconfig file exists
- **THEN** `DetectKindCluster()` SHALL return `ClusterInfo{IsKind: false}` with no error

### Requirement: Adjust deployment parameters for kind
When a kind cluster is detected, the deploy pipeline SHALL set `RuntimeClassName` to empty string (disabling gVisor) and `ImagePullPolicy` to `IfNotPresent`.

#### Scenario: Kind cluster disables gVisor
- **WHEN** deploying to a detected kind cluster
- **THEN** the generated Deployment manifest SHALL NOT include `runtimeClassName`

#### Scenario: Kind cluster uses IfNotPresent pull policy
- **WHEN** deploying to a detected kind cluster
- **THEN** the generated Deployment manifest SHALL have `imagePullPolicy: IfNotPresent`

### Requirement: Load images into kind after build
After `tntc build`, when a kind cluster is detected, the CLI SHALL call `kind load docker-image <image> --name <cluster>` to load the built image into the kind cluster.

#### Scenario: Image loaded into kind after build
- **WHEN** `tntc build` completes and the current context is a kind cluster named `dev`
- **THEN** the CLI SHALL execute `kind load docker-image <tag> --name dev`
