## 1. Implementation

- [ ] 1.1 Add `COPY deno.lock /app/deno.lock` line to `GenerateDockerfile()` in `pkg/builder/dockerfile.go`, after the `COPY .engine/deno.json` line
- [ ] 1.2 Change the `deno cache` RUN instruction from `--no-lock` to `--lock=deno.lock` in `pkg/builder/dockerfile.go`

## 2. Testing

- [ ] 2.1 Update `TestGenerateDockerfileNoLockOnCache` in `pkg/builder/dockerfile_test.go` to assert `--lock=deno.lock` on the cache line instead of `--no-lock`
- [ ] 2.2 Add test asserting `COPY deno.lock` appears in the generated Dockerfile
- [ ] 2.3 Add test asserting runtime ENTRYPOINT still contains `--no-lock`
