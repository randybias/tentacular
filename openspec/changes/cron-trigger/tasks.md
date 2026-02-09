## 1. Spec Changes

- [x] 1.1 Add `Name string` field to `Trigger` in `pkg/spec/types.go`
- [x] 1.2 Add trigger name validation in `pkg/spec/parse.go`: unique names, identRe match
- [x] 1.3 Add trigger name validation tests in `pkg/spec/parse_test.go`

## 2. CronJob Manifest Generation

- [x] 2.1 Add `generateCronJobManifest()` function in `pkg/builder/k8s.go`
- [x] 2.2 Modify `GenerateK8sManifests()` to iterate triggers and append CronJobs
- [x] 2.3 Add CronJob generation tests in `pkg/builder/k8s_test.go`

## 3. K8s Client Updates

- [x] 3.1 Add `CronJob` to `findResource()` map in `pkg/k8s/client.go`
- [x] 3.2 Add label-based CronJob deletion to `DeleteResources()` in `pkg/k8s/client.go`
- [x] 3.3 Add RBAC checks for `batch/cronjobs` and `batch/jobs` in `pkg/k8s/preflight.go`

## 4. Engine Changes

- [x] 4.1 Update executor to accept initial input for root nodes (`engine/executor/simple.ts` + `engine/executor/types.ts`)
- [x] 4.2 Update `engine/server.ts` to parse POST body and pass as input to executor
- [x] 4.3 Update `engine/main.ts` to pass input through the runner (not needed — server handles it)

## 5. CLI and Dockerfile

- [x] 5.1 Add auto-preflight before deploy in `pkg/cli/deploy.go`
- [x] 5.2 Add `--allow-env` to Dockerfile ENTRYPOINT in `pkg/builder/dockerfile.go`

## 6. Verification

- [x] 6.1 Run `go test ./pkg/spec/...` — all pass
- [x] 6.2 Run `go test ./pkg/builder/...` — all pass
- [x] 6.3 Run `go test ./pkg/...` — all pass (spec, builder, cli, k8s)
- [x] 6.4 Run Deno engine tests — 41 pass, 0 failures
