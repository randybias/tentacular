## 1. Go Spec Changes

- [x] 1.1 Add `Subject string` field to `Trigger` in `pkg/spec/types.go`
- [x] 1.2 Add `"queue"` to validTriggerTypes and validate subject required in `pkg/spec/parse.go`
- [x] 1.3 Add queue trigger tests in `pkg/spec/parse_test.go`

## 2. TypeScript Type Changes

- [x] 2.1 Add `"queue"` to Trigger type union, `subject?` and `name?` fields in `engine/types.ts`
- [x] 2.2 Add `@nats-io/transport-deno` to `engine/deno.json`

## 3. NATS Trigger Implementation

- [x] 3.1 Create `engine/triggers/nats.ts` with `startNATSTriggers()` and `NATSTriggerHandle`
- [x] 3.2 Create `engine/triggers/nats_test.ts` with unit tests

## 4. Engine Wiring

- [x] 4.1 Wire up NATS triggers in `engine/main.ts` after server starts
- [x] 4.2 Add graceful shutdown signal handlers in `engine/main.ts`

## 5. Verification

- [x] 5.1 Run `go test ./pkg/spec/...` — all pass
- [x] 5.2 Run `go test ./pkg/...` — all pass (spec, builder, cli, k8s)
- [x] 5.3 Run Deno engine tests — 47 pass, 0 failures (41 existing + 6 new)
