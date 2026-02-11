## MODIFIED Requirements

### Requirement: Deploy command applies manifests via client-go
The `tntc deploy` command SHALL generate K8s manifests including a code ConfigMap, apply them to the cluster, and trigger a rollout restart.

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
