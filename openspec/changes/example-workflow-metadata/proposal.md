## Why

The new `metadata:` section needs a real-world example to serve as documentation and a template for users. Adding metadata to an existing example workflow demonstrates the feature and validates the parser end-to-end.

## What Changes

- Add `metadata:` section to the `sep-tracker` example workflow (`example-workflows/sep-tracker/workflow.yaml`)
- Include owner, team, tags, and environment fields as a reference example
- No code changes -- this is a YAML-only update to an existing example

## Capabilities

### New Capabilities
- `example-metadata`: Add metadata section to an example workflow to serve as documentation and parser validation.

### Modified Capabilities
<!-- None -->

## Impact

- `example-workflows/sep-tracker/workflow.yaml`: Add metadata section
- Can run in parallel with Phase 2 (no code dependency, only Phase 1 struct needed for parsing)
