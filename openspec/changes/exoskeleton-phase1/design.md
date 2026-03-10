# Exoskeleton Phase 1 - Design (tentacular CLI)

## Changes to pkg/spec/parse.go

### ValidateContract()
In the dependency validation loop, add an early check:
```go
if strings.HasPrefix(name, "tentacular-") {
    // Exoskeleton-managed: only protocol is required.
    // host/port/database/user/auth are filled by MCP server.
    continue
}
```
This skip happens AFTER protocol validation but BEFORE protocol-specific field checks.

### validProtocols map
Add `"s3": true` alongside existing `"blob": true`.

## Changes to pkg/spec/derive.go

### DeriveEgressRules()
Skip `tentacular-*` dependencies (MCP server enriches host/port at deploy time):
```go
if strings.HasPrefix(name, "tentacular-") {
    continue
}
```

### DeriveDenoFlags()
Same skip for `tentacular-*` dependencies in the host collection loop.

## No Engine Changes
The engine's `loadSecretsFromDir` already JSON-parses file contents. The `createDependencyAccessor` reads from the enriched contract. Zero engine changes needed.
