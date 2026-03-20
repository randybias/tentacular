## ADDED Requirements

### Requirement: whoami shows group membership
The `tntc whoami` command SHALL display the user's group membership when authenticated via OIDC.

#### Scenario: Whoami with OIDC shows groups
- **WHEN** the user runs `tntc whoami` with an active OIDC session that includes group claims
- **THEN** the output SHALL include a "Groups" line listing the user's groups

#### Scenario: Whoami with bearer token shows no groups
- **WHEN** the user runs `tntc whoami` with bearer-token authentication
- **THEN** the output SHALL not include a Groups line (bearer tokens have no group claims)
