# Pipedreamer v2

Durable workflow execution engine. Build, test, and deploy TypeScript workflow DAGs on Kubernetes with Deno + gVisor sandboxing.

## Quick Start

```bash
# Build the CLI
go build -o pipedreamer ./cmd/pipedreamer/

# Scaffold a new workflow
./pipedreamer init my-workflow

# Validate a workflow spec
./pipedreamer validate my-workflow

# Run locally with hot-reload
./pipedreamer dev my-workflow

# Run tests
./pipedreamer test my-workflow

# Build container and deploy
./pipedreamer build my-workflow
./pipedreamer deploy my-workflow --namespace prod
```

## Architecture

**Go CLI** (`cmd/pipedreamer/`, `pkg/`) manages the full lifecycle: scaffolding, validation, local dev, testing, container builds, and K8s deployments.

**Deno Engine** (`engine/`) executes workflow DAGs inside hardened containers with five layers of defense-in-depth: distroless base, Deno permission locking, gVisor kernel isolation, K8s SecurityContext, and secrets-as-volumes.

See [docs/architecture.md](docs/architecture.md) for the full architecture reference.

## Node Contract

```typescript
import type { Context } from "pipedreamer";

export default async function run(ctx: Context, input: T): Promise<U> {
  const resp = await ctx.fetch("github", "/user/repos");
  ctx.log.info("Fetched repos");
  return { repos: await resp.json() };
}
```

## Testing

```bash
# Go tests
go test ./pkg/...

# Deno engine tests
cd engine && ~/.deno/bin/deno test --allow-read --allow-write=/tmp --allow-net --allow-env
```

## Examples

Working examples in `examples/`: `github-digest`, `hn-digest`, `pr-digest`.

## License

Copyright Mirantis, Inc. All rights reserved.
