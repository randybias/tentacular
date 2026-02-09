## 1. Extract Cascade Module

- [x] 1.1 Create `engine/context/cascade.ts` with `resolveSecrets()` function
- [x] 1.2 Define `ResolveSecretsOptions` interface (explicitPath?, workflowDir)
- [x] 1.3 Implement cascade: explicit path → .secrets/ dir → .secrets.yaml → /app/secrets

## 2. Refactor main.ts

- [x] 2.1 Replace inline cascade logic with `resolveSecrets()` call
- [x] 2.2 Remove unused `loadSecrets` import from main.ts
- [x] 2.3 Remove unused `NodeFunction` type import

## 3. Cascade Tests

- [x] 3.1 Create `engine/context/cascade_test.ts`
- [x] 3.2 Test explicit path takes precedence
- [x] 3.3 Test .secrets/ directory loaded as base layer
- [x] 3.4 Test .secrets.yaml merges on top of .secrets/
- [x] 3.5 Test empty when no sources exist
- [x] 3.6 Test loads from .secrets.yaml when no .secrets/ dir
- [x] 3.7 Test non-overlapping keys from both sources preserved
- [x] 3.8 Test explicit path skips .secrets/ and .secrets.yaml

## 4. Verification

- [x] 4.1 Run `deno test` — 41 tests passing (34 existing + 7 new)
- [x] 4.2 Run `deno check main.ts` — type checks pass
