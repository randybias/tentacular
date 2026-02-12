## ADDED Requirements

### Requirement: Example workflows demonstrate deployment namespace
Production-targeted example workflows SHALL include a `deployment.namespace` field in their workflow.yaml.

#### Scenario: cluster-health-collector namespace
- **WHEN** `example-workflows/cluster-health-collector/workflow.yaml` is parsed
- **THEN** `Deployment.Namespace` SHALL equal `pd-cluster-health`

#### Scenario: cluster-health-reporter namespace
- **WHEN** `example-workflows/cluster-health-reporter/workflow.yaml` is parsed
- **THEN** `Deployment.Namespace` SHALL equal `pd-cluster-health`

#### Scenario: sep-tracker namespace
- **WHEN** `example-workflows/sep-tracker/workflow.yaml` is parsed
- **THEN** `Deployment.Namespace` SHALL equal `pd-sep-tracker`

#### Scenario: uptime-prober namespace
- **WHEN** `example-workflows/uptime-prober/workflow.yaml` is parsed
- **THEN** `Deployment.Namespace` SHALL equal `pd-uptime-prober`

#### Scenario: General-purpose workflows omit namespace
- **WHEN** `example-workflows/github-digest/workflow.yaml`, `pr-digest/workflow.yaml`, `hn-digest/workflow.yaml`, or `word-counter/workflow.yaml` is parsed
- **THEN** `Deployment.Namespace` SHALL be empty (no `deployment:` section)

### Requirement: Example workflows have secrets examples
Workflows that use secrets SHALL include a `.secrets.yaml.example` file.

#### Scenario: github-digest secrets example
- **WHEN** `example-workflows/github-digest/.secrets.yaml.example` is read
- **THEN** it SHALL contain a template for `slack.webhook_url`

#### Scenario: pr-digest secrets example
- **WHEN** `example-workflows/pr-digest/.secrets.yaml.example` is read
- **THEN** it SHALL contain templates for `github.token`, `anthropic.api_key`, and `slack.webhook_url`

### Requirement: Test fixtures include secrets for credential-dependent nodes
Fixtures for nodes that access `ctx.secrets` SHALL include a `secrets` field with test values.

#### Scenario: uptime-prober notify-slack fixture
- **WHEN** `example-workflows/uptime-prober/tests/fixtures/notify-slack.json` is loaded
- **THEN** the fixture SHALL include `secrets.slack.webhook_url` with a test value

#### Scenario: All example workflows pass validation
- **WHEN** `tntc validate` is run on each of the 8 example workflow directories
- **THEN** all SHALL pass validation without errors
