## Why

Seven gaps remain in the tier 0-2 roadmap that affect the new-developer critical path: a preflight secret check bug that fails deploys when local secrets will be auto-provisioned during the same deploy, fixture and guard issues in example workflows that cause test failures, ConfigMap leaks on undeploy, stale roadmap entries, missing CLI and Deno test coverage, nodes with external dependencies (Postgres, Slack, Azure) that throw instead of degrading gracefully in mock test contexts, and documentation inaccuracies in the node-development guidance. Closing these gaps is the final step before tier 0-2 is complete.

## What Changes

- **Phase 1: Preflight secret check bug** -- `deploy.go` checks for secret existence in the cluster even when local `.secrets.yaml` or `.secrets/` will be auto-provisioned during the same deploy. The check should be skipped when local secrets are present, since they will be applied as part of the deploy manifests.
- **Phase 2: Fixture and guard fixes** -- The uptime-prober `notify-slack.json` fixture uses emoji characters that cause JSON parse issues in some environments; replace with ASCII. The hn-digest `filter-stories.ts` node accesses `data.stories` without a guard; add a defensive check for missing or non-array input.
- **Phase 3: Undeploy ConfigMap cleanup** -- `DeleteResources()` in `pkg/k8s/client.go` does not delete the `<name>-code` ConfigMap created by `tntc deploy`. Add ConfigMap deletion to the undeploy flow.
- **Phase 4: Roadmap verification** -- Verify `docs/roadmap.md` accurately reflects the current state: resolved items are in the Archive section, open items have correct tier placement, and no stale entries remain.
- **Phase 5: Minor test gaps** -- Add a Cobra dispatch test in `cmd/tntc/` or `pkg/cli/` that verifies all registered subcommands (init, validate, dev, test, build, deploy, etc.) are wired to the root command. Add a Deno test that validates fixture loading and mock context creation with config/secrets fields.
- **Phase 6: Graceful degradation for external-dep nodes** -- Four nodes with external dependencies (Postgres, Slack, Azure Blob) throw errors when credentials are missing in mock test context instead of degrading gracefully: `cluster-health-collector/store-health-data.ts`, `cluster-health-reporter/query-health-history.ts`, `sep-tracker/store-report.ts` (Postgres portion). Note: `sep-tracker/diff-seps.ts` and `cluster-health-reporter/analyze-trends.ts` already degrade gracefully. The remaining nodes should return stub data when secrets are missing rather than throwing.
- **Phase 7: Node-development doc accuracy** -- Verify and update the tentacular-skill `SKILL.md` and any node-development documentation to accurately reflect the current node contract, fixture format (including config/secrets fields), graceful degradation patterns, and fan-in input shape.

## Capabilities

### New Capabilities

(none -- all changes are fixes and improvements to existing capabilities)

### Modified Capabilities

- `k8s-deploy`: Undeploy flow now deletes the `<name>-code` ConfigMap alongside Service, Deployment, Secret, and CronJobs. Preflight secret check is skipped when local secrets will be auto-provisioned.

## Impact

- **Code**: `pkg/cli/deploy.go` (preflight logic), `pkg/k8s/client.go` (DeleteResources), `example-workflows/uptime-prober/tests/fixtures/notify-slack.json`, `example-workflows/hn-digest/nodes/filter-stories.ts`, `example-workflows/cluster-health-collector/nodes/store-health-data.ts`, `example-workflows/cluster-health-reporter/nodes/query-health-history.ts`, `example-workflows/sep-tracker/nodes/store-report.ts`
- **Tests**: New Cobra dispatch test, new Deno fixture test, updated fixture files
- **Docs**: `docs/roadmap.md`, `tentacular-skill/SKILL.md`, `tentacular-skill/references/testing-guide.md`
- **APIs**: No API changes. All changes are to internal logic, test infrastructure, and documentation.
- **Dependencies**: None. All changes use existing imports and stdlib.
- **Breaking**: None. Undeploy now deletes more resources (ConfigMap), but this is additive cleanup that users currently do manually.
