## ADDED Requirements

### Requirement: Trigger name field
The `Trigger` struct SHALL have an optional `Name` field. Trigger names MUST match the `identRe` pattern (`[a-z][a-z0-9_-]*`). Trigger names MUST be unique within a workflow.

#### Scenario: Valid named trigger
- **WHEN** a workflow has a cron trigger with `name: daily-digest`
- **THEN** parsing succeeds and the trigger's Name field equals "daily-digest"

#### Scenario: Duplicate trigger names rejected
- **WHEN** a workflow has two triggers both named "daily-digest"
- **THEN** parsing returns a validation error about duplicate trigger names

#### Scenario: Invalid trigger name rejected
- **WHEN** a trigger has `name: "Invalid Name"`
- **THEN** parsing returns a validation error about the name format

### Requirement: CronJob manifest generation
`GenerateK8sManifests()` SHALL generate a CronJob manifest for each cron trigger. The CronJob SHALL use `curlimages/curl` to POST to `http://{name}.{namespace}.svc.cluster.local:8080/run`.

#### Scenario: Single cron trigger generates CronJob
- **WHEN** a workflow has one cron trigger with schedule "0 9 * * *"
- **THEN** manifests include a CronJob named `{wf}-cron` with that schedule

#### Scenario: Multiple cron triggers generate numbered CronJobs
- **WHEN** a workflow has two cron triggers
- **THEN** manifests include CronJobs named `{wf}-cron-0` and `{wf}-cron-1`

#### Scenario: Named trigger includes name in POST body
- **WHEN** a cron trigger has `name: daily-digest`
- **THEN** the CronJob's curl command POSTs `{"trigger":"daily-digest"}`

#### Scenario: Unnamed trigger posts empty body
- **WHEN** a cron trigger has no name
- **THEN** the CronJob's curl command POSTs `{}`

#### Scenario: CronJob has correct properties
- **WHEN** a CronJob manifest is generated
- **THEN** it has `concurrencyPolicy: Forbid`, `successfulJobsHistoryLimit: 3`, `failedJobsHistoryLimit: 3`, and correct labels

### Requirement: CronJob cleanup on undeploy
`DeleteResources()` SHALL delete all CronJobs matching the workflow's label selector (`app.kubernetes.io/name={name},app.kubernetes.io/managed-by=pipedreamer`).

#### Scenario: Undeploy removes CronJobs
- **WHEN** `DeleteResources` is called for a workflow with CronJobs
- **THEN** all CronJobs with matching labels are deleted

### Requirement: CronJob RBAC preflight
Preflight checks SHALL verify permissions for `batch/cronjobs` (create, update, delete, list) and `batch/jobs` (list).

#### Scenario: Missing CronJob permissions detected
- **WHEN** the deploying identity lacks `batch/cronjobs create` permission
- **THEN** preflight returns a failure with remediation instructions

### Requirement: POST body passthrough to executor
The `/run` endpoint SHALL parse the POST body as JSON and pass it as initial input to root nodes in the workflow executor.

#### Scenario: POST body flows to first node
- **WHEN** a POST to `/run` includes body `{"trigger":"daily-digest"}`
- **THEN** root nodes receive `{"trigger":"daily-digest"}` as their input

#### Scenario: Empty or missing body defaults to empty object
- **WHEN** a POST to `/run` has no body or empty body
- **THEN** root nodes receive `{}` as their input

### Requirement: Auto-preflight before deploy
`pipedreamer deploy` SHALL run preflight checks before applying manifests. On failure, it SHALL print remediation instructions and abort.

#### Scenario: Preflight failure aborts deploy
- **WHEN** preflight detects missing RBAC permissions
- **THEN** deploy prints the failed checks and aborts without applying manifests

### Requirement: Dockerfile includes --allow-env
The generated Dockerfile ENTRYPOINT SHALL include `--allow-env` in Deno permissions.

#### Scenario: Dockerfile has --allow-env
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the ENTRYPOINT includes `--allow-env`
