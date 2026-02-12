## MODIFIED Requirements

### Requirement: Deploy command applies manifests via client-go
The `tntc deploy` command SHALL generate K8s manifests including a code ConfigMap, apply them to the cluster, and trigger a rollout restart. Secret manifests SHALL resolve `$shared.` references before generating K8s Secret resources.

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

#### Scenario: Shared secrets resolved during deploy
- **WHEN** `tntc deploy` is executed and `.secrets.yaml` contains `$shared.slack` references
- **THEN** `buildSecretFromYAML()` SHALL resolve shared references before generating the K8s Secret manifest
- **AND** the resolved values SHALL appear in the Secret's stringData

#### Scenario: Namespace targeting
- **WHEN** `tntc deploy --namespace prod` is executed
- **THEN** all manifests SHALL target the `prod` namespace
- **AND** client-go operations SHALL target the `prod` namespace

#### Scenario: Default namespace
- **WHEN** `tntc deploy` is executed without `--namespace`
- **THEN** the default namespace SHALL be `default`
