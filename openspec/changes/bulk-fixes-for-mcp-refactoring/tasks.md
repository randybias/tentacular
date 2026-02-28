## 1. Import Map Path Fix

- [ ] 1.1 Fix `./engine/mod.ts` to `./mod.ts` in import map generation in `pkg/builder/k8s.go`
- [ ] 1.2 Add test verifying correct import map path in `pkg/builder/k8s_test.go`

## 2. PSA Security Context

- [ ] 2.1 Add securityContext to proxy-prewarm initContainer in `pkg/builder/k8s.go`
- [ ] 2.2 Add securityContext to CronJob trigger pod spec in `pkg/builder/k8s.go`
- [ ] 2.3 Add tests verifying securityContext in proxy-prewarm and CronJob trigger

## 3. Trigger Egress NetworkPolicy

- [ ] 3.1 Add `generateTriggerEgressNetPol()` function in `pkg/builder/k8s.go`
- [ ] 3.2 Call from `GenerateK8sManifests()` when cron triggers exist
- [ ] 3.3 Add tests for trigger egress NetworkPolicy generation

## 4. Secrets Check Fix

- [ ] 4.1 Change secrets volume mount condition from `len(spec.Secrets) > 0` to check contract dependencies
- [ ] 4.2 Add test verifying secrets mount with contract dependencies

## 5. Verification

- [ ] 5.1 Run `go test ./pkg/builder/...` -- all pass
- [ ] 5.2 Run `go test ./pkg/...` -- all pass
