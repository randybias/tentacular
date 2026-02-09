## Why

The `WorkflowConfig` struct in Go only has `timeout` and `retries` fields. Custom keys like `nats_url` are silently dropped during YAML unmarshaling. This blocks any config-driven approach for passing non-secret configuration to workflows, which is needed for upcoming trigger implementations (NATS queue triggers, etc.).

## What Changes

- Add `yaml:",inline"` extras map to `WorkflowConfig` in Go so arbitrary keys survive YAML parsing
- Add `ToMap()` method that merges typed fields + extras into a flat map for downstream consumption
- Add index signature to `WorkflowConfig` in TypeScript engine types for type-safety
- No runtime engine changes needed — `main.ts` already casts `spec.config as Record<string, unknown>`

## Capabilities

### New Capabilities
- `open-config`: Allow arbitrary keys in workflow `config:` block to flow through to `ctx.config` in nodes

### Modified Capabilities
<!-- None — this is purely additive -->

## Impact

- `pkg/spec/types.go`: WorkflowConfig struct gains Extras map and ToMap() method
- `pkg/spec/parse_test.go`: New tests for custom config keys and ToMap()
- `engine/types.ts`: WorkflowConfig interface gains index signature
- Backward compatible — existing `timeout`/`retries` fields continue to work unchanged
