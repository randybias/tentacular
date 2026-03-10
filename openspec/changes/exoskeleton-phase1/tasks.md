# Exoskeleton Phase 1 - Tasks (tentacular CLI)

## Step 1: Add s3 Protocol
- [x] Add `"s3": true` to `validProtocols` map in `pkg/spec/parse.go`
- [x] Add default port for s3 if applicable (none needed - uses endpoint URL)

## Step 2: Skip Validation for tentacular-* Dependencies
- [x] In `ValidateContract()`, after protocol validation, skip protocol-specific field checks for deps with `tentacular-` prefix
- [x] Only `protocol` is required for these deps
- [x] Auth validation also skipped (MCP provisions auth)

## Step 3: Skip tentacular-* in Egress Rules
- [x] In `DeriveEgressRules()`, skip deps where name has `tentacular-` prefix
- [x] Need to change loop to range over name AND dep (currently ranges over dep only)

## Step 4: Skip tentacular-* in Deno Flags
- [x] In `DeriveDenoFlags()`, skip deps where name has `tentacular-` prefix in host collection loop
- [x] Need to change loop to range over name AND dep

## Step 5: Unit Tests
- [x] Test: tentacular-postgres with only protocol passes validation
- [x] Test: tentacular-nats with only protocol passes validation
- [x] Test: tentacular-rustfs with only protocol passes validation
- [x] Test: s3 protocol accepted
- [x] Test: non-tentacular deps still require all fields
- [x] Test: egress rules skip tentacular-* deps
- [x] Test: deno flags skip tentacular-* deps
- [x] Test: mixed tentacular-* and regular deps work correctly
