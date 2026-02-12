## ADDED Requirements

### Requirement: Optional deployment section in workflow.yaml
The workflow spec SHALL support an optional `deployment` section with deployment configuration.

#### Scenario: Deployment namespace parsed
- **WHEN** a workflow.yaml contains `deployment: { namespace: pd-uptime-prober }`
- **THEN** `spec.Parse()` SHALL return a Workflow with `Deployment.Namespace == "pd-uptime-prober"`

#### Scenario: No deployment section
- **WHEN** a workflow.yaml does not contain a `deployment` section
- **THEN** `spec.Parse()` SHALL return a Workflow with `Deployment` as zero-value (`Namespace == ""`)
- **AND** parsing SHALL succeed without errors

#### Scenario: Empty deployment section
- **WHEN** a workflow.yaml contains `deployment: {}` with no fields
- **THEN** `spec.Parse()` SHALL return a Workflow with `Deployment.Namespace == ""`
- **AND** parsing SHALL succeed without errors
