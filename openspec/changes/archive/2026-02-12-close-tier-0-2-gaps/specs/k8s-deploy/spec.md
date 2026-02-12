## MODIFIED Requirements

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

## ADDED Requirements

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
