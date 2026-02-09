## Why

The secrets cascade logic in `engine/main.ts` (lines 67-95) is complex multi-source merging behavior that is currently inline and untested. This makes it fragile — any change to the cascade order or merge behavior could silently break secrets loading. Extracting it into a testable module and adding dedicated tests ensures the cascade contract is verified.

## What Changes

- **`engine/context/cascade.ts`** — new module exporting `resolveSecrets()` that implements the secrets cascade: explicit path → .secrets/ directory → .secrets.yaml → /app/secrets K8s volume mount
- **`engine/main.ts`** — refactored to import and call `resolveSecrets()` instead of inline cascade logic
- **`engine/context/cascade_test.ts`** — 7 tests covering all cascade paths: explicit precedence, directory loading, YAML merge on top, empty state, YAML-only fallback, non-overlapping key preservation, explicit skips other sources

## Capabilities

### New Capabilities
- `secrets-cascade-module`: Extracted, testable secrets resolution with configurable explicit path and workflow directory inputs
- `deno-test-cascade`: 7 tests covering secrets cascade precedence, merge behavior, and edge cases

### Modified Capabilities
- `engine-startup`: main.ts now delegates to resolveSecrets() instead of inline cascade logic (behavior unchanged)

## Impact

- **New files**: `engine/context/cascade.ts`, `engine/context/cascade_test.ts`
- **Modified files**: `engine/main.ts` (replaced inline cascade with resolveSecrets() call, removed unused NodeFunction import)
- **Test count**: 34 existing → 41 total (7 new cascade tests)
- **Dependencies**: none new (uses existing loadSecrets, std/path)
