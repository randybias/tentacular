## 1. Go WorkflowConfig Changes

- [x] 1.1 Add `Extras map[string]interface{}` with `yaml:",inline"` tag to `WorkflowConfig` in `pkg/spec/types.go`
- [x] 1.2 Add `ToMap() map[string]interface{}` method on `WorkflowConfig` that merges typed fields + extras

## 2. Go Tests

- [x] 2.1 Add test: custom config keys parse into `Extras` (e.g. `nats_url`, `custom_key`)
- [x] 2.2 Add test: `ToMap()` includes both typed and extra fields
- [x] 2.3 Add test: `ToMap()` omits zero-valued typed fields
- [x] 2.4 Add test: typed fields still work alongside extras

## 3. TypeScript Engine Types

- [x] 3.1 Add index signature `[key: string]: unknown` to `WorkflowConfig` in `engine/types.ts`

## 4. Verification

- [x] 4.1 Run `go test ./pkg/spec/...` — all tests pass
- [x] 4.2 Run Deno engine tests — no regressions
