## 1. Core Implementation

- [ ] 1.1 Add `DeriveDenoFlags(c *Contract) []string` function to `pkg/spec/derive.go` that extracts host:port pairs from contract dependencies and builds the full `deno run` argument list
- [ ] 1.2 Add `Contract *spec.Contract` field to `DeployOptions` struct in `pkg/builder/k8s.go`
- [ ] 1.3 Modify `GenerateK8sManifests` in `pkg/builder/k8s.go` to call `DeriveDenoFlags` when `opts.Contract` is non-nil and inject `command`/`args` into the Deployment container spec

## 2. Testing

- [ ] 2.1 Add `TestDeriveDenoFlags_HTTPSDependencies` in `pkg/spec/derive_test.go` for HTTPS deps producing correct `--allow-net` flag
- [ ] 2.2 Add `TestDeriveDenoFlags_PostgreSQLDependency` in `pkg/spec/derive_test.go` for PostgreSQL deps
- [ ] 2.3 Add `TestDeriveDenoFlags_NilContract` in `pkg/spec/derive_test.go` returning nil
- [ ] 2.4 Add `TestDeriveDenoFlags_DynamicTargetFallback` in `pkg/spec/derive_test.go` for dynamic-target deps returning nil
- [ ] 2.5 Add `TestDeriveDenoFlags_DefaultPorts` in `pkg/spec/derive_test.go` for protocol default ports
- [ ] 2.6 Add `TestDeriveDenoFlags_StaticFlags` in `pkg/spec/derive_test.go` verifying all static flags are present
- [ ] 2.7 Add `TestK8sManifestDynamicDenoFlags` in `pkg/builder/k8s_test.go` for Deployment with contract-derived command/args
- [ ] 2.8 Add `TestK8sManifestNoContractNoCommandArgs` in `pkg/builder/k8s_test.go` verifying no command/args without contract
- [ ] 2.9 Update `TestDeploymentNoContainerArgs` in `pkg/builder/k8s_test.go` if affected by new contract field

## 3. Documentation

- [ ] 3.1 Update `docs/architecture.md` to document Deno-level permission hardening as part of the defense-in-depth model
- [ ] 3.2 Update `docs/node-contract.md` to note that contract dependencies affect runtime Deno `--allow-net` flags
- [ ] 3.3 Update `docs/workflow-spec.md` to mention that contracts drive Deno permission flags
- [ ] 3.4 Update `docs/secrets.md` to note security hardening interaction with Deno permissions
- [ ] 3.5 Update `docs/roadmap.md` to reflect security hardening progress
- [ ] 3.6 Update `docs/testing.md` to note testing of permission flag derivation
