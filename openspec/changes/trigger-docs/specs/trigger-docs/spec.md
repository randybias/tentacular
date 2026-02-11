## ADDED Requirements

### Requirement: Architecture docs cover triggers
The `docs/architecture.md` SHALL include a Triggers section with a table of all trigger types, their mechanism, required fields, K8s resources, and implementation status.

#### Scenario: Trigger table present
- **WHEN** a developer reads architecture.md
- **THEN** they find a comprehensive trigger types table

### Requirement: Roadmap doc exists
A `docs/roadmap.md` SHALL document future enhancements including webhook bridge, JetStream, ConfigMap overrides, and rate limiting.

#### Scenario: Roadmap covers future triggers
- **WHEN** a developer reads roadmap.md
- **THEN** they find planned enhancements for webhook, JetStream, and DLQ

### Requirement: Skill file documents triggers
The `tentacular-skill/SKILL.md` SHALL include trigger types, NATS configuration, config block documentation, and the full trigger lifecycle.

#### Scenario: Skill file has trigger info
- **WHEN** an AI assistant reads SKILL.md
- **THEN** it can explain all trigger types and how to configure them

### Requirement: Deployment guide covers triggers
The `tentacular-skill/references/deployment-guide.md` SHALL include sections on cron trigger deployment, NATS connection setup, and undeploy cleanup.

#### Scenario: Deployment guide has trigger sections
- **WHEN** a developer reads the deployment guide
- **THEN** they find step-by-step trigger deployment instructions
