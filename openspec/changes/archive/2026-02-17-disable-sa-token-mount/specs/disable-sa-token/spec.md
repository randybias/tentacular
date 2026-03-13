## ADDED Requirements

### Requirement: Deployment pod spec disables ServiceAccount token automount
The generated Deployment manifest SHALL include `automountServiceAccountToken: false` at the pod spec level (under `spec.template.spec`).

#### Scenario: Generated Deployment contains automountServiceAccountToken false
- **WHEN** `GenerateK8sManifests` is called for any workflow
- **THEN** the Deployment manifest content SHALL contain `automountServiceAccountToken: false`

#### Scenario: Token mount disabled regardless of deploy options
- **WHEN** `GenerateK8sManifests` is called with any `DeployOptions` (including empty options and options with RuntimeClassName set)
- **THEN** the Deployment manifest SHALL always contain `automountServiceAccountToken: false`
