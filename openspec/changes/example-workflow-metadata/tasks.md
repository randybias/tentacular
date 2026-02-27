## 1. Update Example Workflow

- [x] 1.1 Add `metadata:` section to `example-workflows/sep-tracker/workflow.yaml` with owner, team, tags, and environment
- [x] 1.2 Verify the updated workflow.yaml parses correctly with `tntc validate` or parser test

## 2. Verification

- [x] 2.1 Run `go test ./pkg/spec/...` with example workflow parsing -- no regressions
