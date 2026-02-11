## ADDED Requirements

### Requirement: K8s Deployment manifest generation
The `pkg/builder/k8s.go` `GenerateK8sManifests()` function SHALL generate a Deployment manifest with gVisor RuntimeClass.

#### Scenario: Deployment manifest structure
- **WHEN** `GenerateK8sManifests(wf, imageTag, namespace)` is called
- **THEN** the returned manifests SHALL include a Deployment with:
  - `apiVersion: apps/v1`
  - `kind: Deployment`
  - `metadata.name` set to the workflow name
  - `metadata.namespace` set to the target namespace
  - `spec.replicas` set to 1

#### Scenario: gVisor RuntimeClass
- **WHEN** a Deployment manifest is generated
- **THEN** `spec.template.spec.runtimeClassName` SHALL be `gvisor`

#### Scenario: Container spec
- **WHEN** a Deployment manifest is generated
- **THEN** the container SHALL:
  - Be named `engine`
  - Use the provided `imageTag` as the image
  - Expose `containerPort: 8080` with `protocol: TCP`

#### Scenario: Tentacular labels
- **WHEN** a Deployment manifest is generated
- **THEN** both the Deployment and Pod template SHALL have labels:
  - `app.kubernetes.io/name: <workflow-name>`
  - `app.kubernetes.io/managed-by: tentacular`

#### Scenario: Resource limits
- **WHEN** a Deployment manifest is generated
- **THEN** the container SHALL have resource requests and limits:
  - Requests: `memory: 64Mi`, `cpu: 100m`
  - Limits: `memory: 256Mi`, `cpu: 500m`

### Requirement: K8s Service manifest generation
The `GenerateK8sManifests()` function SHALL generate a ClusterIP Service manifest.

#### Scenario: Service manifest structure
- **WHEN** `GenerateK8sManifests(wf, imageTag, namespace)` is called
- **THEN** the returned manifests SHALL include a Service with:
  - `apiVersion: v1`
  - `kind: Service`
  - `metadata.name` set to the workflow name
  - `metadata.namespace` set to the target namespace
  - `spec.type: ClusterIP`

#### Scenario: Service port mapping
- **WHEN** a Service manifest is generated
- **THEN** it SHALL map port `8080` to `targetPort: 8080` with `protocol: TCP`

#### Scenario: Service selector
- **WHEN** a Service manifest is generated
- **THEN** `spec.selector` SHALL match `app.kubernetes.io/name: <workflow-name>`

### Requirement: Secrets mounted as K8s Secret volumes
Secrets SHALL be mounted from a K8s Secret as a read-only volume, never as environment variables.

#### Scenario: Secret volume mount
- **WHEN** a Deployment manifest is generated
- **THEN** the container SHALL have a volume mount:
  - `name: secrets`
  - `mountPath: /app/secrets`
  - `readOnly: true`

#### Scenario: Secret volume source
- **WHEN** a Deployment manifest is generated for workflow named `my-workflow`
- **THEN** the pod spec SHALL have a volume:
  - `name: secrets`
  - `secret.secretName: my-workflow-secrets`
  - `secret.optional: true`

#### Scenario: No env var secrets
- **WHEN** a Deployment manifest is generated
- **THEN** the container spec SHALL NOT contain `env` or `envFrom` fields referencing secrets

#### Scenario: Temp volume
- **WHEN** a Deployment manifest is generated
- **THEN** the pod spec SHALL include an `emptyDir` volume mounted at `/tmp` for temporary file writes

### Requirement: Deploy command applies manifests via client-go
The `tntc deploy` command SHALL generate K8s manifests and apply them to the cluster using client-go.

#### Scenario: Successful deployment
- **WHEN** `tntc deploy` is executed in a directory with a valid `workflow.yaml`
- **THEN** it SHALL generate K8s manifests (Deployment + Service)
- **AND** it SHALL apply them to the target namespace via client-go
- **AND** it SHALL print confirmation with the workflow name and namespace

#### Scenario: Create-or-update semantics
- **WHEN** `tntc deploy` is executed and a resource does not exist
- **THEN** it SHALL create the resource
- **WHEN** `tntc deploy` is executed and a resource already exists
- **THEN** it SHALL update the resource (preserving resourceVersion for optimistic concurrency)

#### Scenario: Namespace targeting
- **WHEN** `tntc deploy --namespace prod` is executed
- **THEN** all manifests SHALL target the `prod` namespace
- **AND** client-go operations SHALL target the `prod` namespace

#### Scenario: Default namespace
- **WHEN** `tntc deploy` is executed without `--namespace`
- **THEN** the default namespace SHALL be `default`

### Requirement: Deploy validates workflow spec
The deploy command SHALL validate the workflow spec before deploying.

#### Scenario: Invalid spec rejected
- **WHEN** `tntc deploy` is executed with an invalid `workflow.yaml`
- **THEN** it SHALL return a validation error
- **AND** it SHALL NOT apply any manifests to the cluster

### Requirement: Deploy uses registry flag for image tag
The deploy command SHALL construct the image tag using the workflow spec and optional registry flag.

#### Scenario: Image tag with registry
- **WHEN** `tntc deploy --registry gcr.io/myproject` is executed for workflow `data-pipeline` version `1.0`
- **THEN** the Deployment manifest SHALL reference image `gcr.io/myproject/data-pipeline:1-0`

#### Scenario: Image tag without registry
- **WHEN** `tntc deploy` is executed without `--registry` for workflow `data-pipeline` version `1.0`
- **THEN** the Deployment manifest SHALL reference image `data-pipeline:1-0`

### Requirement: K8s client initialization
The `pkg/k8s/client.go` `NewClient()` function SHALL create a K8s client from available configuration.

#### Scenario: In-cluster config
- **WHEN** `NewClient()` is called from within a K8s pod
- **THEN** it SHALL use in-cluster configuration (service account token)

#### Scenario: Kubeconfig fallback
- **WHEN** `NewClient()` is called outside a K8s cluster
- **THEN** it SHALL use `$KUBECONFIG` environment variable if set
- **OR** fall back to `~/.kube/config`

#### Scenario: Client creation failure
- **WHEN** no valid kubeconfig is available
- **THEN** `NewClient()` SHALL return an error with context about the failure

### Requirement: Status command reports deployment health
The `tntc status <name>` command SHALL query the K8s API and report deployment status.

#### Scenario: Healthy deployment
- **WHEN** `tntc status my-workflow` is executed and the deployment is healthy
- **THEN** it SHALL report: workflow name, namespace, status "ready", replica counts (available/total)

#### Scenario: Unhealthy deployment
- **WHEN** `tntc status my-workflow` is executed and the deployment is not ready
- **THEN** it SHALL report status "not ready" with available vs desired replica counts

#### Scenario: Deployment not found
- **WHEN** `tntc status my-workflow` is executed and no deployment exists
- **THEN** it SHALL return an error indicating the deployment was not found

#### Scenario: Text output
- **WHEN** `tntc status my-workflow` is executed with default `--output text`
- **THEN** it SHALL print human-readable status lines (Workflow, Namespace, Status, Replicas)

#### Scenario: JSON output
- **WHEN** `tntc status my-workflow --output json` is executed
- **THEN** it SHALL print a JSON object with fields: `name`, `namespace`, `ready`, `replicas`, `available`

#### Scenario: Namespace flag
- **WHEN** `tntc status my-workflow --namespace prod` is executed
- **THEN** it SHALL query the `prod` namespace for the deployment

### Requirement: Status requires workflow name argument
The status command SHALL require exactly one argument: the workflow name.

#### Scenario: Missing argument
- **WHEN** `tntc status` is executed without arguments
- **THEN** it SHALL return an error indicating the name argument is required

#### Scenario: Too many arguments
- **WHEN** `tntc status foo bar` is executed
- **THEN** it SHALL return an error indicating too many arguments
