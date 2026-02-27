## 1. Architecture Section

- [ ] 1.1 Add "Architecture" section to SKILL.md explaining CLI (build/ship) vs MCP (operate/enforce) boundary.
- [ ] 1.2 Add diagram or structured description of the workflow lifecycle: init -> validate -> test -> build -> deploy (MCP) -> run (MCP) -> monitor (MCP).

## 2. MCP Tool Reference

- [ ] 2.1 Add "MCP Tools" section listing all available tools with descriptions.
- [ ] 2.2 Document `wf_list`, `wf_describe`, `wf_run`, `wf_deploy` with parameters and examples.
- [ ] 2.3 Document `wf_pods`, `wf_logs`, `wf_events`, `wf_trigger` with parameters and examples.
- [ ] 2.4 Document `wf_apply`, `wf_remove` with parameters and examples.
- [ ] 2.5 Document `ns_list`, `ns_ensure`, `cluster_health`, `cluster_audit`, `gvisor_status`.
- [ ] 2.6 Document `credential_validate`.

## 3. Update Existing Sections

- [ ] 3.1 Update deployment workflow to show MCP-routed deploy flow.
- [ ] 3.2 Update "Querying Running Workflows" to reference MCP tools.
- [ ] 3.3 Update troubleshooting section with MCP-specific guidance.
- [ ] 3.4 Clarify which `tntc` commands are build-time only (init, validate, test, build, dev).

## 4. Review

- [ ] 4.1 Verify all MCP tool names and parameters match actual implementation.
- [ ] 4.2 Review for accuracy against tentacular-mcp `pkg/tools/register.go`.
- [ ] 4.3 Ensure examples are copy-pasteable and correct.
