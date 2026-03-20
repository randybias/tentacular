## 1. Annotation Migration

- [ ] 1.1 Update annotation constants in `pkg/builder/k8s.go` from `tentacular.dev/*` to `tentacular.io/*`
- [ ] 1.2 Verify all builder-generated manifests use new annotation namespace

## 2. WfApplyParams Extension

- [ ] 2.1 Add Group (string) and Share (bool) fields to WfApplyParams in `pkg/mcp/tools.go`
- [ ] 2.2 Add --group flag to deploy command in `pkg/cli/deploy.go`
- [ ] 2.3 Add --share flag to deploy command in `pkg/cli/deploy.go`
- [ ] 2.4 Wire group/share flags through to WfApplyParams

## 3. Permissions Commands

- [ ] 3.1 Create `pkg/cli/permissions.go` with permissions parent command
- [ ] 3.2 Implement `permissions get` subcommand (calls permissions_get MCP tool, displays owner/group/mode)
- [ ] 3.3 Implement `permissions set` subcommand (calls permissions_set MCP tool, --mode/--group/--owner flags)
- [ ] 3.4 Implement `permissions chmod` convenience subcommand
- [ ] 3.5 Implement `permissions chgrp` convenience subcommand
- [ ] 3.6 Register permissions commands in `cmd/tntc/main.go`

## 4. Whoami Extension

- [ ] 4.1 Extend `pkg/cli/whoami.go` to display group membership from OIDC claims
- [ ] 4.2 Handle case where groups are not available (bearer token)

## 5. Testing

- [ ] 5.1 Add tests for permissions commands
- [ ] 5.2 Add tests for deploy --group and --share flag handling
- [ ] 5.3 Add tests for annotation migration in builder
