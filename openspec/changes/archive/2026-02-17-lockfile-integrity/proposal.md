## Why

The generated Dockerfile uses `--no-lock` on both the `deno cache` build step and the runtime `ENTRYPOINT`. This means dependency integrity is never verified -- a compromised registry could serve different code at build vs run time, or between builds. Adding `deno.lock` to the image and using `--lock=deno.lock` on the cache step pins dependencies at build time. The runtime ENTRYPOINT keeps `--no-lock` since the cache is already verified and the filesystem is read-only.

## What Changes

- Add `COPY deno.lock /app/deno.lock` instruction to the Dockerfile
- Change the `deno cache` RUN instruction from `--no-lock` to `--lock=deno.lock`
- Keep `--no-lock` on the runtime ENTRYPOINT (read-only filesystem, cache already verified)
- Update tests to reflect the new Dockerfile content

## Capabilities

### New Capabilities
- `lockfile-integrity`: Use deno.lock for dependency integrity verification at build time

### Modified Capabilities
<!-- None -->

## Impact

- `pkg/builder/dockerfile.go`: Add COPY deno.lock, change cache step flag
- `pkg/builder/dockerfile_test.go`: Update test assertions for new COPY and lock flag behavior
