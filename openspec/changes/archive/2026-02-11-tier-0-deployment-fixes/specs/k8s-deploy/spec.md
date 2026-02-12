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

## ADDED Requirements

### Requirement: Nested YAML secrets produce JSON stringData
The `buildSecretFromYAML()` function in `pkg/cli/deploy.go` SHALL handle nested YAML maps by JSON-serializing them into K8s Secret stringData values.

#### Scenario: Flat string value unchanged
- **WHEN** `.secrets.yaml` contains `api_key: "sk_test_123"`
- **THEN** the generated K8s Secret stringData SHALL contain `api_key: "sk_test_123"` as a plain string

#### Scenario: Nested map JSON-serialized
- **WHEN** `.secrets.yaml` contains `slack: { webhook_url: "https://hooks.slack.com/xxx" }`
- **THEN** the generated K8s Secret stringData SHALL contain `slack` with a JSON-serialized string value: `{"webhook_url":"https://hooks.slack.com/xxx"}`

#### Scenario: Mixed flat and nested values
- **WHEN** `.secrets.yaml` contains both flat strings and nested maps
- **THEN** flat strings SHALL remain as plain string values
- **AND** nested maps SHALL be JSON-serialized
- **AND** both SHALL appear in the same K8s Secret stringData section

#### Scenario: Deeply nested maps
- **WHEN** `.secrets.yaml` contains multi-level nesting like `db: { postgres: { host: "localhost", port: 5432 } }`
- **THEN** the entire nested structure SHALL be JSON-serialized into a single stringData value for `db`

#### Scenario: Invalid YAML still rejected
- **WHEN** `.secrets.yaml` contains invalid YAML syntax
- **THEN** `buildSecretFromYAML()` SHALL return an error containing "parsing secrets YAML"

#### Scenario: Empty secrets YAML
- **WHEN** `.secrets.yaml` contains an empty map `{}`
- **THEN** `buildSecretFromYAML()` SHALL return nil (no manifest generated)
