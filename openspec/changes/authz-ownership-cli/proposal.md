## Why

The CLI needs to support the new authorization model by passing group/share parameters to the MCP server at deploy time, exposing new permissions commands (get, set, chmod, chgrp), and updating annotation handling to use the new `tentacular.io/*` namespace. Without CLI changes, users cannot set permissions or see group membership.

## What Changes

- **BREAKING**: Annotation migration in `pkg/builder/k8s.go` from `tentacular.dev/*` to `tentacular.io/*`
- New `--group` and `--share` flags on `tntc deploy`
- Extend WfApplyParams in `pkg/mcp/tools.go` with Group and Share fields
- New `pkg/cli/permissions.go` with get, set, chmod, chgrp subcommands
- Extend `tntc whoami` to show group membership
- Register new permissions commands in the Cobra command tree

## Capabilities

### New Capabilities
- `cli-permissions`: New permissions CLI commands (get, set, chmod, chgrp) that call MCP permissions_get/permissions_set tools
- `cli-deploy-authz`: --group and --share flags on deploy, WfApplyParams extension
- `cli-identity`: Extended whoami output showing group membership

### Modified Capabilities
<!-- No existing OpenSpec capabilities to modify -->

## Impact

- **pkg/builder/k8s.go**: Annotation key changes (tentacular.dev -> tentacular.io)
- **pkg/mcp/tools.go**: WfApplyParams struct gains Group and Share fields
- **pkg/cli/deploy.go**: New --group and --share flags
- **pkg/cli/whoami.go**: Extended output
- **cmd/tntc/main.go**: New command registration
- **Cross-repo dependency**: Requires tentacular-mcp permissions_get/permissions_set tools to be implemented first
