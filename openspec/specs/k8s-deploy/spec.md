# k8s-deploy Specification

## Purpose
TBD - created by archiving change build-and-deploy. Update Purpose after archive.
## Requirements
### Requirement: K8s Deployment manifest generation
The `pkg/builder/k8s.go` `GenerateK8sManifests()` function SHALL generate a Deployment manifest that includes a code ConfigMap volume mount.

#### Scenario: Deployment manifest structure
- **WHEN** `GenerateK8sManifests(wf, imageTag, namespace)` is called
- **THEN** the returned manifests SHALL include a Deployment with:
  - `apiVersion: apps/v1`
  - `kind: Deployment`
  - `metadata.name` set to the workflow name
  - `metadata.namespace` set to the target namespace
  - `spec.replicas` set to 1

#### Scenario: Container spec
- **WHEN** a Deployment manifest is generated
- **THEN** the container SHALL:
  - Be named `engine`
  - Use the provided `imageTag` as the image
  - Expose `containerPort: 8080` with `protocol: TCP`
  - NOT have an `args` field (relies on base image ENTRYPOINT defaults)

#### Scenario: Code volume mount
- **WHEN** a Deployment manifest is generated for workflow `my-workflow`
- **THEN** the container SHALL have a volumeMount:
  - `name: code`
  - `mountPath: /app/workflow`
  - `readOnly: true`

#### Scenario: Code volume source
- **WHEN** a Deployment manifest is generated for workflow `my-workflow`
- **THEN** the pod spec SHALL have a volume:
  - `name: code`
  - `configMap.name: my-workflow-code`

#### Scenario: Existing volumes preserved
- **WHEN** a Deployment manifest is generated
- **THEN** the pod spec SHALL still include the `secrets` volume (secret mount at `/app/secrets`) and `tmp` volume (emptyDir at `/tmp`)

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
The `tntc deploy` command SHALL generate K8s manifests including a code ConfigMap, apply them to the cluster, and trigger a rollout restart. The preflight secret existence check SHALL be skipped when local secrets will be auto-provisioned during the same deploy.

#### Scenario: Successful deployment
- **WHEN** `tntc deploy` is executed in a directory containing a valid `workflow.yaml`
- **THEN** it SHALL generate K8s manifests (Deployment + Service + ConfigMap)
- **AND** it SHALL apply them to the target namespace via client-go
- **AND** it SHALL trigger a rollout restart of the Deployment
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

#### Scenario: Preflight skips secret check when local secrets present
- **WHEN** `tntc deploy` is executed and a `.secrets.yaml` file or `.secrets/` directory exists in the workflow directory
- **THEN** the preflight check SHALL NOT verify secret existence in the cluster
- **AND** the deploy SHALL proceed to auto-provision the secret from local files

#### Scenario: Preflight checks secret when no local secrets
- **WHEN** `tntc deploy` is executed and no `.secrets.yaml` file or `.secrets/` directory exists
- **THEN** the preflight check SHALL skip the secret existence check entirely
- **AND** it SHALL log that no local secrets were found

### Requirement: Deploy validates workflow spec
The deploy command SHALL validate the workflow spec before deploying.

#### Scenario: Invalid spec rejected
- **WHEN** `tntc deploy` is executed with an invalid `workflow.yaml`
- **THEN** it SHALL return a validation error
- **AND** it SHALL NOT apply any manifests to the cluster

### Requirement: Deploy uses registry flag for image tag
The deploy command SHALL resolve the base image tag using a cascade: `--image` flag > `.tentacular/base-image.txt` > `tentacular-engine:latest`.

#### Scenario: Image from --image flag
- **WHEN** `tntc deploy --image gcr.io/proj/engine:v2` is executed
- **THEN** the Deployment manifest SHALL reference image `gcr.io/proj/engine:v2`

#### Scenario: Image from base-image.txt
- **WHEN** `tntc deploy` is executed without `--image` and `.tentacular/base-image.txt` contains `gcr.io/proj/tentacular-engine:latest`
- **THEN** the Deployment manifest SHALL reference image `gcr.io/proj/tentacular-engine:latest`

#### Scenario: Image from default fallback
- **WHEN** `tntc deploy` is executed without `--image` and `.tentacular/base-image.txt` does not exist
- **THEN** the Deployment manifest SHALL reference image `tentacular-engine:latest`

#### Scenario: Deprecated --cluster-registry flag
- **WHEN** `tntc deploy --cluster-registry gcr.io/proj` is executed
- **THEN** it SHALL return an error indicating `--cluster-registry` is removed and to use `--image` instead

### Requirement: Deploy generates code ConfigMap
The deploy command SHALL generate a code ConfigMap from the workflow directory and include it in the applied manifests.

#### Scenario: ConfigMap included in apply
- **WHEN** `tntc deploy` is executed in a valid workflow directory
- **THEN** the manifests applied to the cluster SHALL include a ConfigMap named `{workflow-name}-code`

#### Scenario: ConfigMap size exceeded
- **WHEN** `tntc deploy` is executed and workflow code exceeds 900KB
- **THEN** it SHALL return an error before applying any manifests

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

### Requirement: K8s API call timeouts
All K8s API calls SHALL use a 30-second timeout context to prevent indefinite hangs when the API server is unreachable.

#### Scenario: Apply timeout
- **WHEN** `Apply()` is called and the K8s API server is unreachable
- **THEN** the operation SHALL fail with a context deadline exceeded error after 30 seconds

#### Scenario: GetStatus timeout
- **WHEN** `GetStatus()` is called and the K8s API server is unreachable
- **THEN** the operation SHALL fail with a context deadline exceeded error after 30 seconds

#### Scenario: PreflightCheck timeout
- **WHEN** `PreflightCheck()` is called and the K8s API server is unreachable
- **THEN** the operation SHALL fail with a context deadline exceeded error after 30 seconds

### Requirement: Nil-safe replica count
The `GetStatus()` function SHALL handle nil `Spec.Replicas` (K8s default when unset, meaning 1 replica).

#### Scenario: Replicas unset
- **WHEN** a Deployment has nil `Spec.Replicas`
- **THEN** `GetStatus()` SHALL default to 1 replica (matching K8s behavior)
- **AND** it SHALL NOT panic with a nil pointer dereference

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

### Requirement: Undeploy deletes code ConfigMap
The `tntc undeploy` command SHALL delete the code ConfigMap (`<name>-code`) alongside Service, Deployment, Secret, and CronJobs.

#### Scenario: ConfigMap deleted on undeploy
- **WHEN** `tntc undeploy my-workflow` is executed and `my-workflow-code` ConfigMap exists
- **THEN** the ConfigMap `my-workflow-code` SHALL be deleted
- **AND** the deletion SHALL be reported in the output (e.g., `deleted ConfigMap/my-workflow-code`)

#### Scenario: ConfigMap not found on undeploy
- **WHEN** `tntc undeploy my-workflow` is executed and `my-workflow-code` ConfigMap does not exist
- **THEN** the undeploy SHALL continue without error (NotFound is silently skipped)

#### Scenario: Undeploy deletes all resource types
- **WHEN** `tntc undeploy my-workflow` is executed
- **THEN** it SHALL attempt to delete: Service, Deployment, Secret (`my-workflow-secrets`), ConfigMap (`my-workflow-code`), and CronJobs matching the label selector

