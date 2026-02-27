## 1. wf_run Implementation

- [ ] 1.1 Add `WfRunParams` struct with Name, Namespace (required), Input (optional JSON string) in `run.go`.
- [ ] 1.2 Add `WfRunResult` struct with JobName, Namespace, Status, Message fields.
- [ ] 1.3 Implement `handleWfRun()`: guard namespace check, read workflow Deployment, create Job from Deployment PodSpec with input env var.
- [ ] 1.4 Set `ttlSecondsAfterFinished: 3600` on created Jobs for auto-cleanup.
- [ ] 1.5 Generate unique Job name: `<workflow-name>-run-<timestamp>`.

## 2. K8s Client Support

- [ ] 2.1 Add `CreateJob(ctx, namespace, *batchv1.Job)` helper to `pkg/k8s/client.go` if not present.
- [ ] 2.2 Add `GetDeployment(ctx, namespace, name)` helper if not present.

## 3. Registration

- [ ] 3.1 Create `registerRunTools()` function in `run.go`.
- [ ] 3.2 Add `registerRunTools(srv, client)` call in `register.go`.

## 4. Tests

- [ ] 4.1 Create `run_test.go` with test for successful wf_run (mock Deployment, verify Job spec).
- [ ] 4.2 Test wf_run with invalid namespace (guard rejection).
- [ ] 4.3 Test wf_run with missing Deployment (not found error).
- [ ] 4.4 Test wf_run with input parameter (verify env var injection).
- [ ] 4.5 Run `go test ./pkg/tools/...` -- all pass.
- [ ] 4.6 Run `go test ./...` -- all pass.
