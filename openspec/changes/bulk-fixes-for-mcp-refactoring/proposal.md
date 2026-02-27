## Why

After the MCP server refactoring, several issues were discovered during end-to-end testing that prevent workflows from deploying and running correctly in PSA-restricted namespaces. These are small, targeted fixes that individually are straightforward but collectively block production use of the MCP-based deployment flow.

## What Changes

- Fix import map path in generated Deno config (`./engine/mod.ts` to `./mod.ts`) so the engine resolves modules correctly in the ConfigMap-based code delivery model
- Add PSA-compliant security context to proxy-prewarm init container and CronJob trigger pods (runAsNonRoot, runAsUser 65534, drop ALL capabilities)
- Generate egress NetworkPolicy for trigger pods so CronJob curl containers can reach the workflow Service
- Fix secrets volume mount check to use contract dependency names instead of raw secret names

## Capabilities

### New Capabilities
- `trigger-egress-netpol`: Generate a NetworkPolicy allowing trigger pod egress to the workflow Service on port 8080

### Modified Capabilities
- `k8s-manifest-gen`: Fix import map path, add PSA security context to proxy-prewarm and CronJob trigger, fix secrets check logic

## Impact

- `pkg/builder/k8s.go`: Fix import map `./engine/mod.ts` to `./mod.ts`; add securityContext to proxy-prewarm initContainer; add securityContext to CronJob trigger pod; generate trigger egress NetworkPolicy; fix secrets volume mount condition to check contract dependencies
- `pkg/builder/k8s_test.go`: Add/update tests for all four fixes
