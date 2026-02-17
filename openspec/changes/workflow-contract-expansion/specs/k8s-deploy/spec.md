## ADDED Requirements

### Requirement: NetworkPolicy derived from contract dependencies
`deploy` SHALL generate Kubernetes NetworkPolicy resources by deriving rules from `contract.dependencies` and workflow trigger types.

#### Scenario: Egress policy derived from dependencies
- **WHEN** a workflow declares dependencies with host/port/protocol metadata
- **THEN** generated manifests SHALL include NetworkPolicy with one egress allow rule per dependency

### Requirement: Default-deny implicit for hardened pod
Generated policy SHALL enforce deny-by-default semantics for both ingress and egress without requiring explicit declaration.

#### Scenario: Egress deny-by-default
- **WHEN** a workflow has a contract with dependencies
- **THEN** generated policy SHALL deny all egress not matching a dependency-derived allow rule

#### Scenario: Ingress deny-by-default for non-webhook
- **WHEN** a workflow has no webhook trigger
- **THEN** generated policy SHALL deny all ingress

### Requirement: Mandatory DNS egress
Generated policy SHALL always include DNS egress rules for cluster DNS resolution.

#### Scenario: DNS egress auto-generated
- **WHEN** NetworkPolicy is generated for any workflow
- **THEN** policy SHALL include egress allow for UDP and TCP port 53 to kube-dns
- **AND** the DNS rule SHALL be present regardless of dependency declarations

### Requirement: Ingress derived from trigger type
Ingress policy SHALL be derived from workflow trigger configuration.

#### Scenario: Webhook trigger ingress
- **WHEN** a workflow has a `webhook` trigger with a declared path
- **THEN** generated policy SHALL allow ingress on the webhook service port

### Requirement: Protocol/port scoping from dependency metadata
Generated egress rules SHALL use protocol and port from dependency declarations.

#### Scenario: Port-scoped egress
- **WHEN** a dependency declares `protocol: postgresql`, `port: 5432`
- **THEN** generated policy SHALL scope the egress allow rule to TCP port 5432

### Requirement: Additional egress overrides applied
When `contract.networkPolicy.additionalEgress` entries exist, they SHALL be merged into derived policy.

#### Scenario: CIDR-based additional egress
- **WHEN** an additional egress entry specifies a CIDR block and port
- **THEN** generated policy SHALL include that CIDR/port as an additional egress allow rule

### Requirement: Contract-aware deployment validation gate
Deploy SHALL fail when strict enforcement is active and contract validation fails.

#### Scenario: Strict deploy failure
- **WHEN** strict mode is active and contract validation detects issues
- **THEN** deploy SHALL abort before manifest application with diagnostics
