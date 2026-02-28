## ADDED Requirements

### Requirement: MCP server ingress NetworkPolicy rule
The `DeriveIngressRules()` function SHALL include an ingress rule allowing TCP port 8080 from the `tentacular-system` namespace. This rule SHALL be unconditional (present for all workflows regardless of trigger type) to enable MCP server health probes.

#### Scenario: MCP ingress rule present for non-webhook workflow
- **WHEN** `DeriveIngressRules(wf)` is called for a workflow without webhook triggers
- **THEN** the returned rules SHALL include an ingress rule with port 8080, protocol TCP, and `FromNamespaceLabels` containing `kubernetes.io/metadata.name: tentacular-system`
- **AND** the existing trigger-scoped ingress rule (FromLabels `tentacular.dev/role: trigger`) SHALL still be present

#### Scenario: MCP ingress rule present for webhook workflow
- **WHEN** `DeriveIngressRules(wf)` is called for a workflow with webhook triggers
- **THEN** the returned rules SHALL include an ingress rule with port 8080, protocol TCP, and `FromNamespaceLabels` containing `kubernetes.io/metadata.name: tentacular-system`
- **AND** the existing webhook ingress rule (open podSelector with istio-system namespace) SHALL still be present

#### Scenario: NetworkPolicy renders MCP ingress rule
- **WHEN** `GenerateNetworkPolicy()` produces a NetworkPolicy YAML for any workflow
- **THEN** the ingress section SHALL contain a rule with `namespaceSelector` matching `kubernetes.io/metadata.name: tentacular-system` on port 8080/TCP
