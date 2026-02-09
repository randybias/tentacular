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
