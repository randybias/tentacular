# configmap-code-delivery Specification

## Purpose
K8s ConfigMap generation for workflow code delivery, enabling fast iteration without Docker rebuilds.

## Requirements
### Requirement: ConfigMap generation from workflow directory
The `pkg/builder/k8s.go` `GenerateCodeConfigMap(wf *spec.Workflow, workflowDir, namespace string) (Manifest, error)` function SHALL generate a K8s ConfigMap manifest containing workflow code files.

#### Scenario: ConfigMap contains workflow.yaml
- **WHEN** `GenerateCodeConfigMap()` is called with a valid workflow directory
- **THEN** the returned ConfigMap data SHALL include a key `workflow.yaml` with the file's content as the value

#### Scenario: ConfigMap contains node files with flattened keys
- **WHEN** `GenerateCodeConfigMap()` is called and the workflow directory contains `nodes/fetch.ts` and `nodes/summarize.ts`
- **THEN** the returned ConfigMap data SHALL include keys `nodes__fetch.ts` and `nodes__summarize.ts` with their respective file contents
- **NOTE:** Keys use `__` instead of `/` because Kubernetes ConfigMap keys cannot contain forward slashes (validation regex: `[-._a-zA-Z0-9]+`)

#### Scenario: ConfigMap name follows convention
- **WHEN** `GenerateCodeConfigMap()` is called for a workflow named `my-workflow`
- **THEN** the ConfigMap `metadata.name` SHALL be `my-workflow-code`

#### Scenario: ConfigMap namespace
- **WHEN** `GenerateCodeConfigMap()` is called with namespace `production`
- **THEN** the ConfigMap `metadata.namespace` SHALL be `production`

#### Scenario: ConfigMap labels
- **WHEN** `GenerateCodeConfigMap()` is called for workflow `my-workflow`
- **THEN** the ConfigMap SHALL have labels `app.kubernetes.io/name: my-workflow` and `app.kubernetes.io/managed-by: pipedreamer`

### Requirement: ConfigMap size validation
The `GenerateCodeConfigMap()` function SHALL return an error if total data size exceeds 900KB.

#### Scenario: Data within limit
- **WHEN** `GenerateCodeConfigMap()` is called and total file content is under 900KB
- **THEN** it SHALL return the ConfigMap manifest without error

#### Scenario: Data exceeds limit
- **WHEN** `GenerateCodeConfigMap()` is called and total file content exceeds 900KB
- **THEN** it SHALL return an error indicating the size limit was exceeded
- **AND** the error message SHALL include the actual total size

### Requirement: ConfigMap reads only expected file types
The `GenerateCodeConfigMap()` function SHALL only include workflow.yaml and .ts files from the nodes/ directory.

#### Scenario: Non-ts files in nodes/ excluded
- **WHEN** the workflow directory contains `nodes/README.md` or `nodes/.gitkeep`
- **THEN** these files SHALL NOT appear as keys in the ConfigMap

#### Scenario: Missing nodes directory
- **WHEN** the workflow directory has no `nodes/` subdirectory
- **THEN** `GenerateCodeConfigMap()` SHALL still succeed with only `workflow.yaml` in the ConfigMap

### Requirement: Deployment includes code ConfigMap volume
The `GenerateK8sManifests()` function SHALL add a code volume to the Deployment pod spec that references the workflow's ConfigMap.

#### Scenario: Code volume defined
- **WHEN** `GenerateK8sManifests()` is called for workflow `my-workflow`
- **THEN** the Deployment pod spec SHALL include a volume named `code` with `configMap.name: my-workflow-code`

#### Scenario: Code volume mounted
- **WHEN** `GenerateK8sManifests()` generates a Deployment
- **THEN** the engine container SHALL have a volumeMount with `name: code`, `mountPath: /app/workflow`, `readOnly: true`

#### Scenario: ConfigMap items field maps flattened keys to paths
- **WHEN** `GenerateK8sManifests()` generates a Deployment for a workflow with nodes
- **THEN** the code ConfigMap volume SHALL include an `items` field
- **AND** the items SHALL map `workflow.yaml` to `workflow.yaml`
- **AND** the items SHALL map each `nodes__<filename>.ts` key to `nodes/<filename>.ts` path
- **RATIONALE:** ConfigMap data keys cannot contain slashes, so flattened keys (`nodes__foo.ts`) are mapped back to proper paths (`nodes/foo.ts`) at mount time

### Requirement: Deployment relies on ENTRYPOINT defaults for workflow path
The generated Deployment SHALL NOT include container `args` for `--workflow` or `--port`. The base image ENTRYPOINT already defaults to `--workflow /app/workflow/workflow.yaml --port 8080`, matching the ConfigMap mount path.

#### Scenario: No container args set
- **WHEN** `GenerateK8sManifests()` generates a Deployment
- **THEN** the engine container SHALL NOT have an `args` field
- **AND** the ENTRYPOINT defaults from the base image SHALL provide `--workflow /app/workflow/workflow.yaml` and `--port 8080`

### Requirement: Rollout restart after deploy
The `pkg/k8s/client.go` `RolloutRestart(namespace, deploymentName string) error` method SHALL trigger a rolling restart by patching the deployment's pod template annotation.

#### Scenario: Restart annotation patched
- **WHEN** `RolloutRestart("default", "my-workflow")` is called
- **THEN** the deployment's `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]` SHALL be set to the current timestamp

#### Scenario: Restart uses 30-second timeout
- **WHEN** `RolloutRestart()` is called and the K8s API is unreachable
- **THEN** it SHALL fail with a context deadline exceeded error after 30 seconds
