## 1. Context Type Definitions

- [x] 1.1 Update `engine/context/types.ts` — define `ContextOptions` interface with `nodeId`, `secrets` (SecretsConfig), and `config` (Record) fields; define `SecretsConfig` type as `Record<string, Record<string, string>>`
- [x] 1.2 Verify `engine/types.ts` — ensure `Context` interface has `fetch`, `log`, `config`, `secrets` members; ensure `Logger` has `info`, `warn`, `error`, `debug` methods; ensure `NodeFunction` type is `(ctx: Context, input: unknown) => Promise<unknown>`; ensure `NodeModule` has `default: NodeFunction`

## 2. Context Implementation

- [x] 2.1 Update `engine/context/mod.ts` — implement `createContext(opts)` factory function returning a plain Context object with `fetch`, `log`, `config`, `secrets`
- [x] 2.2 Implement `createLogger(nodeId)` — returns Logger object with `info`, `warn`, `error`, `debug` methods that prefix output with `[<nodeId>] <LEVEL>`
- [x] 2.3 Implement `createFetch(secrets)` — returns fetch function that resolves service names to URLs (`https://api.<service>.com<path>` or full URL passthrough), injects `Authorization: Bearer` from `secrets[service].token`, injects `X-API-Key` from `secrets[service].api_key`

## 3. Secrets Loading

- [x] 3.1 Update `engine/context/secrets.ts` — implement `loadSecrets(path)` that auto-detects file vs directory
- [x] 3.2 Implement `loadSecretsFromFile(path)` — parse YAML file keyed by service name, return empty object on missing file or invalid YAML
- [x] 3.3 Implement `loadSecretsFromDir(dirPath)` — read each non-hidden file in directory, parse JSON content as service secrets, fall back to `{ value: content }` for plain text files

## 4. Public API Exports

- [x] 4.1 Update `engine/mod.ts` — export `Context`, `Logger`, `NodeFunction`, `NodeModule` types from `./types.ts`; export `WorkflowSpec`, `ExecutionResult`, and other engine types for advanced users

## 5. Unit Tests

- [x] 5.1 Write `engine/context/context_test.ts` — test `createContext` returns object with all four members; test logger prefixes with node ID; test fetch URL resolution (convention and full URL passthrough); test fetch auth injection (bearer token, API key, no secrets); test custom headers preserved alongside injected auth
- [x] 5.2 Write `engine/context/secrets_test.ts` — test `loadSecrets` from YAML file; test `loadSecrets` from directory; test missing file returns empty; test hidden files skipped; test invalid YAML returns empty; test plain text file handling

## 6. Verification

- [x] 6.1 Verify `deno check engine/main.ts` passes with no type errors
- [x] 6.2 Verify `deno test` passes in engine directory for context and secrets tests
- [x] 6.3 Verify `import type { Context, Logger, NodeFunction } from "pipedreamer"` resolves via import map
