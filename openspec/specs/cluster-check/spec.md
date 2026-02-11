# cluster-check Specification

## Purpose
TBD - created by archiving change cluster-readiness. Update Purpose after archive.
## Requirements
### Requirement: K8s API reachability check
The preflight check SHALL verify that the Kubernetes API server is reachable using the current kubeconfig or in-cluster configuration.

#### Scenario: API is reachable
- **WHEN** `tntc cluster check` is executed and the K8s API server is reachable
- **THEN** the check SHALL pass with the name "K8s API reachable"

#### Scenario: API is unreachable
- **WHEN** `tntc cluster check` is executed and the K8s API server is not reachable
- **THEN** the check SHALL fail with the name "K8s API reachable"
- **AND** the remediation message SHALL include the connection error details
- **AND** all remaining checks SHALL be skipped (early termination)

### Requirement: gVisor RuntimeClass check
The preflight check SHALL verify that a RuntimeClass named "gvisor" exists in the cluster.

#### Scenario: gVisor RuntimeClass exists
- **WHEN** `tntc cluster check` is executed and a RuntimeClass named "gvisor" exists
- **THEN** the check SHALL pass with the name "gVisor RuntimeClass"

#### Scenario: gVisor RuntimeClass missing
- **WHEN** `tntc cluster check` is executed and no RuntimeClass named "gvisor" exists
- **THEN** the check SHALL fail with the name "gVisor RuntimeClass"
- **AND** the remediation message SHALL include a link to gVisor installation documentation

### Requirement: Namespace existence check
The preflight check SHALL verify that the target namespace exists in the cluster. The target namespace is determined by the `--namespace` global flag (default: "default").

#### Scenario: Namespace exists
- **WHEN** `tntc cluster check --namespace prod` is executed and namespace "prod" exists
- **THEN** the check SHALL pass with the name "Namespace 'prod'"

#### Scenario: Namespace missing without --fix
- **WHEN** `tntc cluster check --namespace prod` is executed without `--fix` and namespace "prod" does not exist
- **THEN** the check SHALL fail with the name "Namespace 'prod'"
- **AND** the remediation message SHALL include `kubectl create namespace prod` and suggest using `--fix`

#### Scenario: Namespace missing with --fix
- **WHEN** `tntc cluster check --namespace prod --fix` is executed and namespace "prod" does not exist
- **THEN** the namespace "prod" SHALL be created in the cluster
- **AND** the check SHALL pass with the name "Namespace 'prod'" and indicate it was auto-created

### Requirement: RBAC permissions check
The preflight check SHALL verify that the current identity has sufficient RBAC permissions to manage tentacular resources in the target namespace.

#### Scenario: RBAC permissions sufficient
- **WHEN** `tntc cluster check` is executed and the current identity can create, update, and delete Deployments, Services, ConfigMaps, and Secrets in the target namespace
- **THEN** the check SHALL pass with the name "RBAC permissions"

#### Scenario: RBAC permissions insufficient
- **WHEN** `tntc cluster check` is executed and the current identity lacks any required permission
- **THEN** the check SHALL fail with the name "RBAC permissions"
- **AND** the remediation message SHALL list the specific permissions that are missing

### Requirement: Secret references check
The preflight check SHALL verify that secret references in the workflow spec resolve to existing Kubernetes Secrets in the target namespace.

#### Scenario: All secrets exist
- **WHEN** `tntc cluster check` is executed and all secret names referenced in the workflow spec exist as K8s Secrets in the target namespace
- **THEN** the check SHALL pass with the name "Secret references"

#### Scenario: Missing secrets
- **WHEN** `tntc cluster check` is executed and one or more secret names referenced in the workflow spec do not exist in the target namespace
- **THEN** the check SHALL fail with the name "Secret references"
- **AND** the remediation message SHALL list the missing secret names

#### Scenario: No workflow spec found
- **WHEN** `tntc cluster check` is executed and no workflow spec is provided or found in the current directory
- **THEN** the secret references check SHALL be skipped with an informational message

### Requirement: --fix flag
The `tntc cluster check` command SHALL accept a `--fix` flag that auto-creates resources when possible.

#### Scenario: --fix creates namespace
- **WHEN** `tntc cluster check --fix --namespace my-ns` is executed and namespace "my-ns" does not exist
- **THEN** namespace "my-ns" SHALL be created via the K8s API
- **AND** the check result SHALL indicate the namespace was auto-created

#### Scenario: --fix does not affect other checks
- **WHEN** `tntc cluster check --fix` is executed and gVisor RuntimeClass is missing
- **THEN** the gVisor check SHALL still fail (--fix does not install gVisor)

### Requirement: Output format
The `tntc cluster check` command SHALL display results in human-readable text by default and JSON when `--output json` is specified.

#### Scenario: Text output
- **WHEN** `tntc cluster check` is executed with default output
- **THEN** each check SHALL be printed as a line with a pass icon or fail icon followed by the check name
- **AND** failed checks SHALL include a remediation message on the following line

#### Scenario: JSON output
- **WHEN** `tntc cluster check --output json` is executed
- **THEN** the output SHALL be a JSON array of objects, each with "name" (string), "passed" (boolean), and "remediation" (string, empty if passed) fields

#### Scenario: Overall result
- **WHEN** all checks pass
- **THEN** the command SHALL print a success summary and exit with code 0
- **WHEN** any check fails
- **THEN** the command SHALL exit with a non-zero exit code

