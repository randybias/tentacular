## Context

The tentacular-mcp server has workflow tools for listing, describing, deploying, and monitoring workflows. It is missing a `wf_run` tool to trigger workflow execution. Currently, `tntc run` creates a K8s Job directly. The existing `wf_trigger` tool in `workflow.go` creates a one-shot Job for CronJob-triggered workflows, but `wf_run` is a more general tool for on-demand execution with input parameters.

## Goals / Non-Goals

**Goals:**
- Add `wf_run` MCP tool that triggers a workflow execution by creating a K8s Job.
- Accept workflow name, namespace, and optional JSON input.
- Return Job name, creation status, and initial pod status.
- Follow existing handler pattern: params struct, result struct, guard namespace check.

**Non-Goals:**
- Streaming execution output (use `wf_logs` for that).
- Waiting for Job completion (caller polls with `wf_pods`/`wf_logs`).
- Supporting DAG-level node selection (runs the full workflow).

## Decisions

### Separate from wf_trigger
`wf_trigger` is designed for CronJob-style triggers. `wf_run` is for on-demand execution with arbitrary input. They create different Job specs (trigger uses the CronJob's template, run creates a fresh Job from the workflow Deployment spec).

Alternative: Extend `wf_trigger` -- mixes two different execution models.

### Job creation from Deployment spec
`wf_run` reads the workflow Deployment to get the container image and env config, then creates a Job with a matching PodSpec plus the user-provided input as an environment variable (`TNTC_INPUT`).

### Input as JSON string
User input is passed as a JSON string in the `TNTC_INPUT` environment variable. The workflow engine already supports reading input from this env var.

## Risks / Trade-offs

- **Job cleanup**: Jobs created by `wf_run` accumulate. Use `ttlSecondsAfterFinished` to auto-clean. Default 3600 (1 hour).
- **Resource limits**: Jobs inherit resource limits from the Deployment spec. No additional resource configuration in `wf_run`.
- **Concurrent runs**: Multiple `wf_run` calls create multiple Jobs. No built-in mutual exclusion.
