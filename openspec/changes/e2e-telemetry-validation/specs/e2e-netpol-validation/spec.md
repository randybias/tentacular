## ADDED Requirements

### Requirement: MCP ingress rule generation for all workflow types
The test suite SHALL verify that DeriveIngressRules produces an MCP ingress rule for every workflow type, allowing tentacular-system namespace to reach port 8080.

#### Scenario: Webhook workflow includes MCP ingress rule
- **WHEN** DeriveIngressRules is called for a workflow with a webhook trigger
- **THEN** the returned rules SHALL include an ingress rule with namespaceSelector matching tentacular-system and port 8080, in addition to the webhook ingress rule

#### Scenario: Non-webhook workflow includes MCP ingress rule
- **WHEN** DeriveIngressRules is called for a workflow without a webhook trigger (e.g., cron-only)
- **THEN** the returned rules SHALL include an ingress rule with namespaceSelector matching tentacular-system and port 8080

#### Scenario: MCP ingress rule is unconditional
- **WHEN** DeriveIngressRules is called for any workflow configuration
- **THEN** the MCP ingress rule SHALL always be present regardless of trigger type, network policy settings, or contract egress rules

### Requirement: NetworkPolicy YAML rendering with MCP ingress
The test suite SHALL verify that the rendered NetworkPolicy YAML correctly includes the MCP ingress rule with proper namespace selector structure.

#### Scenario: Rendered YAML contains tentacular-system namespace selector
- **WHEN** a NetworkPolicy is rendered from ingress rules that include the MCP ingress rule
- **THEN** the YAML output SHALL contain an ingress entry with a namespaceSelector matchLabels entry for the tentacular-system namespace and a port entry for TCP 8080

#### Scenario: Rendered YAML is valid Kubernetes NetworkPolicy
- **WHEN** a NetworkPolicy is rendered with the MCP ingress rule
- **THEN** the output SHALL parse as valid YAML and conform to the Kubernetes NetworkPolicy v1 schema structure (apiVersion, kind, metadata, spec with ingress array)
