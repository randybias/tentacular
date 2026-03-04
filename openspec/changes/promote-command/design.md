# Design: tntc promote command

## Command Structure

```
tntc promote --from <env> --to <env> [workflow-name]
  --verify    Run workflow once after promotion to verify
  --force     Skip confirmation prompt
```

If `workflow-name` is omitted, it is inferred from the current directory's
`workflow.yaml` (same pattern as `tntc deploy`).

## Implementation

### File: `pkg/cli/promote.go`

```go
func NewPromoteCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "promote [workflow-name]",
        Short: "Promote a workflow from one environment to another",
        Args:  cobra.MaximumNArgs(1),
        RunE:  runPromote,
    }
    cmd.Flags().String("from", "", "Source environment (required)")
    cmd.Flags().String("to", "", "Target environment (required)")
    cmd.Flags().Bool("verify", false, "Run workflow after promotion to verify")
    cmd.Flags().Bool("force", false, "Skip confirmation prompt")
    cmd.MarkFlagRequired("from")
    cmd.MarkFlagRequired("to")
    return cmd
}
```

### Promote Flow

1. **Resolve source and target environments** using `ResolveEnvironment`.
2. **Create MCP clients** for both source and target.
3. **Verify source is healthy** via `wf_health` on the source MCP server. This
   is the health gate: only healthy workflows can be promoted.
4. **Re-build manifests locally** from the workflow source directory using the
   target environment's settings (namespace, runtime class, image). This reuses
   the existing `buildManifests()` function with `InternalDeployOptions` populated
   from the target environment config.
5. **Apply to target** via `wf_apply` on the target MCP server.
6. **(Optional) Verify** via `wf_run` on the target.

### Why Re-Build Instead of Transfer

The `wf_describe` tool returns deployment metadata (annotations, replica status,
nodes, triggers) but does NOT return the original applied manifests. Manifests
are not stored or reconstructable from the MCP server.

Re-building from local source is the correct approach because:
- The workflow source (workflow.yaml, nodes/, .secrets.yaml) is the single source
  of truth for manifest generation.
- `buildManifests()` already handles all manifest generation logic (ConfigMap,
  Deployment, Service, NetworkPolicy, import map, secrets).
- Environment-specific settings (namespace, runtime class, image) are applied
  via `InternalDeployOptions`, which can be populated from the target env config.
- This makes `promote` equivalent to `deploy --env <target>` but with an
  explicit source health gate and promotion intent.

### Health Gate

Before building and applying, the promote command calls `wf_health` on the
source environment to verify the workflow is running and healthy:

```go
healthResult, err := sourceMCP.WfHealth(ctx, sourceNS, workflowName, false)
if err != nil || healthResult.Status != "GREEN" {
    return fmt.Errorf("source workflow %s is not healthy (status: %s), aborting promotion", ...)
}
```

This prevents promoting a broken or degraded workflow.

### Secrets Handling

Secrets are NOT transferred between environments. The promote command:
1. Checks if the source deployment has an associated Secret.
2. If yes, warns the user that secrets must be provisioned separately in the
   target environment.
3. Does not fail -- the workflow may still work if secrets are already
   provisioned in the target.

## Registration

Add to the root command in `cmd/tntc/main.go`:

```go
rootCmd.AddCommand(cli.NewPromoteCmd())
```
