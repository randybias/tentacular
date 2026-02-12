## MODIFIED Requirements

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
  - Have `imagePullPolicy: Always`
  - Expose `containerPort: 8080` with `protocol: TCP`
  - NOT have an `args` field (relies on base image ENTRYPOINT defaults)

#### Scenario: ImagePullPolicy always set
- **WHEN** a Deployment manifest is generated
- **THEN** the container spec SHALL include `imagePullPolicy: Always`
- **AND** this SHALL appear immediately after the `image:` field

#### Scenario: Version label on all resources
- **WHEN** K8s manifests are generated for a workflow with `version: "1.0"`
- **THEN** all generated resources (ConfigMap, Deployment, Service, CronJob) SHALL include the label `app.kubernetes.io/version: 1.0`

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

## MODIFIED Requirements

### Requirement: Deploy command applies manifests via client-go
The `tntc deploy` command SHALL generate K8s manifests including a code ConfigMap, apply them to the cluster, and trigger a rollout restart.

#### Scenario: Namespace cascade resolution
- **WHEN** `tntc deploy` is executed
- **THEN** namespace SHALL be resolved in priority order: CLI `-n` flag > `workflow.yaml` `deployment.namespace` > config file default > `"default"`

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

#### Scenario: Namespace from workflow.yaml
- **WHEN** `tntc deploy` is executed without `-n` flag and `workflow.yaml` contains `deployment: { namespace: pd-uptime-prober }`
- **THEN** all manifests SHALL target the `pd-uptime-prober` namespace

#### Scenario: CLI flag overrides workflow.yaml namespace
- **WHEN** `tntc deploy -n prod` is executed and `workflow.yaml` contains `deployment: { namespace: pd-uptime-prober }`
- **THEN** all manifests SHALL target the `prod` namespace (CLI flag wins)

#### Scenario: Default namespace
- **WHEN** `tntc deploy` is executed without `--namespace`, without `deployment.namespace` in workflow.yaml, and without config file namespace
- **THEN** the default namespace SHALL be `default`
