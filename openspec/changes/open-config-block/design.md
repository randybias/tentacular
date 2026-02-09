## Context

The `WorkflowConfig` struct in Go has only two typed fields: `timeout` and `retries`. Go's `yaml.v3` ignores unknown YAML keys during unmarshaling, so custom config like `nats_url: "..."` is silently dropped. The Deno engine side already handles arbitrary keys — `main.ts` casts `spec.config as Record<string, unknown>`. The fix is Go-only.

## Goals / Non-Goals

**Goals:**
- Allow arbitrary keys in the YAML `config:` block to survive Go parsing
- Provide a `ToMap()` method for downstream consumers to get a flat key-value map
- Add TypeScript index signature for type-safety on the engine side
- Maintain backward compatibility with existing `timeout`/`retries` fields

**Non-Goals:**
- Config validation beyond YAML parsing (consumers validate their own keys)
- Environment variable interpolation in config values
- Config file hot-reload

## Decisions

### Use `yaml:",inline"` for extras map
Go's `yaml.v3` supports an `inline` tag on `map[string]interface{}` fields. Unknown YAML keys flow into this map while typed fields (`timeout`, `retries`) still bind normally. Typed fields take precedence — if both exist, the typed field wins.

**Alternative**: Custom `UnmarshalYAML` — more control but significantly more code for the same result. The inline tag is well-tested in yaml.v3.

### ToMap() merges typed + extras
A single method produces a flat `map[string]interface{}` containing both typed fields and extras. This is what gets passed to the engine as `ctx.config`. Typed fields are included only if non-zero to avoid polluting the map.

**Alternative**: Expose Extras directly — forces consumers to check two places. ToMap() provides a single unified view.

### Index signature on TypeScript interface
Adding `[key: string]: unknown` to `WorkflowConfig` allows TypeScript code to access custom keys without casting. This matches how the engine already uses config.

## Risks / Trade-offs

- **Key collision**: If a user defines `timeout` as a custom extra, the typed field wins. This is the correct behavior — documented typed fields take precedence. Low risk since field names are well-known.
- **yaml.v3 inline behavior**: Well-tested in the Go ecosystem, used by many projects. No compatibility concerns.
