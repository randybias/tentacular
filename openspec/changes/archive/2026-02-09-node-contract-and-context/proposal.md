## Why

Workflow nodes need a defined contract (function signature, default export) and a runtime Context object that provides HTTP fetching with automatic auth injection, structured logging, config access, and secrets. Without this, node authors have no standard API to code against, and the executor has no contract to invoke. This change defines the node-to-engine interface that all workflow nodes depend on.

## What Changes

- **Node function contract** — every node file must `export default async function run(ctx: Context, input: T): Promise<U>`. The engine imports and calls this default export with a Context object and the upstream node's output.
- **Context interface** — a plain object (not a class) with four members:
  - `ctx.fetch(service, path, init?)` — abstracted HTTP calls that resolve service names to URLs and inject auth headers from secrets automatically
  - `ctx.log` — structured logger with `info`, `warn`, `error`, `debug` methods, prefixed with the node ID
  - `ctx.config` — node-specific configuration from `workflow.yaml`
  - `ctx.secrets` — workflow-level secrets loaded from file or K8s volume mount
- **Secrets loading** — `.secrets.yaml` file for local development, K8s Secret volume mount directory for production. Single `loadSecrets(path)` function handles both formats.
- **Public module exports** — `import { Context } from "tentacular"` resolves via the Deno import map to `engine/mod.ts`, which re-exports all node-author-facing types
- **In-memory data passing** — node outputs are passed to downstream nodes in-memory via the executor; no persistence layer needed

## Capabilities

### New Capabilities
- `node-contract`: Node function signature, default export convention, async function receiving Context and input, returning output
- `context-api`: Context interface with fetch/log/config/secrets members, secret loading from file and K8s volume mount directory

### Modified Capabilities
_(none)_

## Impact

- **Files**: `engine/context/mod.ts`, `engine/context/types.ts`, `engine/context/secrets.ts`, `engine/mod.ts`
- **Dependencies**: Deno std/yaml (already in import map for YAML parsing of secrets files)
- **APIs**: Establishes the `Context` interface and `NodeFunction` type that all node files and the executor depend on
- **Downstream changes**: The executor (Change 03) invokes the node contract; the testing framework (Change 07) provides mock Context objects; the dev command (Change 05) creates live Context instances
