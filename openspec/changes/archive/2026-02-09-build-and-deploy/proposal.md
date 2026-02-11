## Why

Tentacular workflows must be packaged as container images and deployed to Kubernetes. The Go CLI currently has stub implementations for `build`, `deploy`, and `status` commands. This change replaces those stubs with working implementations that generate Dockerfiles, build container images, generate K8s manifests, apply them via client-go, and report deployment status. This is the sixth change in the dependency chain, building on the project foundation (01), workflow spec (02), DAG engine (03), context/secrets (04), and dev command (05).

## What Changes

- **`tntc build [dir]`** — generates a Dockerfile using `denoland/deno:distroless` base, copies the engine and workflow files into the build context, and invokes `docker build` to produce a tagged container image
- **`tntc deploy [dir]`** — generates K8s manifests (Deployment with gVisor RuntimeClass, ClusterIP Service), applies them to the target namespace via client-go, with secrets mounted as K8s Secret volumes (not environment variables)
- **`tntc status <name>`** — queries the K8s API for deployment health, reporting replica counts, availability, and readiness in text or JSON format
- **Dockerfile generation** (`pkg/builder/dockerfile.go`) — produces a minimal Dockerfile: distroless Deno base, engine + workflow + nodes copied in, Deno permission flags locked down
- **K8s manifest generation** (`pkg/builder/k8s.go`) — produces Deployment (gVisor RuntimeClass, resource limits, secret volume mounts) and Service (ClusterIP) manifests with tentacular labels
- **K8s client wrapper** (`pkg/k8s/client.go`) — wraps client-go for create-or-update apply semantics and deployment status queries

## Capabilities

### New Capabilities
- `container-build`: Dockerfile generation from distroless Deno base image, engine directory copying into build context, container image building and tagging via `docker build`, image tag derivation from workflow name + version
- `k8s-deploy`: K8s manifest generation (Deployment with gVisor RuntimeClass, ClusterIP Service), secret volume mounts from K8s Secrets, client-go apply with create-or-update semantics, deployment status checking with replica counts and readiness

### Modified Capabilities
_(none -- existing stubs are replaced with working implementations)_

## Impact

- **Modified files**: `pkg/cli/build.go`, `pkg/cli/deploy.go`, `pkg/cli/status.go`, `pkg/builder/dockerfile.go`, `pkg/builder/k8s.go`, `pkg/k8s/client.go`
- **Dependencies**: `k8s.io/client-go`, `k8s.io/apimachinery` (already in go.mod), Docker CLI (runtime dependency for builds)
- **CLI**: `tntc build`, `tntc deploy`, and `tntc status` commands transition from stubs/initial implementations to fully specified behavior
- **Security**: Secrets are mounted as K8s Secret volumes at `/app/secrets` (read-only), never exposed as environment variables. Containers run under gVisor sandbox via RuntimeClass. Deno permissions are locked to `--allow-net`, `--allow-read=/app`, `--allow-write=/tmp`.
- **Downstream**: Enables end-to-end workflow lifecycle from `init` through `build` and `deploy` to production
