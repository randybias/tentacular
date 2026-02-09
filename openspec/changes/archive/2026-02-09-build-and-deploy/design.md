## Context

Pipedreamer v2 has a two-component architecture: a Go CLI for management/K8s operations and a Deno/TypeScript engine for workflow execution. The project foundation (change 01) established the CLI skeleton with command stubs. The workflow spec (change 02), DAG engine (change 03), context/secrets (change 04), and dev command (change 05) provide the runtime pieces. This change closes the loop by enabling container image builds and Kubernetes deployments.

The existing codebase already has initial implementations in `pkg/cli/build.go`, `pkg/cli/deploy.go`, `pkg/cli/status.go`, `pkg/builder/dockerfile.go`, `pkg/builder/k8s.go`, and `pkg/k8s/client.go`. This change specifies the expected behavior of these implementations and ensures they meet the project's security and operational requirements.

## Goals / Non-Goals

**Goals:**
- Generate a secure, minimal Dockerfile based on `denoland/deno:distroless`
- Build container images with workflow name + version as image tag
- Copy only the engine directory (not the CLI binary) into the container
- Generate K8s Deployment manifests with gVisor RuntimeClass
- Generate K8s Service manifests with ClusterIP type
- Mount secrets from K8s Secrets as volume mounts (not env vars)
- Apply manifests via client-go with create-or-update semantics
- Report deployment status including readiness, replica counts, and availability
- Support `--namespace`, `--registry`, `--tag`, and `--output` flags

**Non-Goals:**
- Multi-architecture image builds (single arch for now)
- Helm chart generation or Kustomize overlays
- Ingress/Gateway API configuration (users manage external access)
- Container registry authentication (users configure `docker login` separately)
- Horizontal Pod Autoscaler (HPA) configuration
- CI/CD pipeline integration (build/deploy are local CLI commands)

## Decisions

### Decision 1: Distroless Deno base image
**Choice:** Use `denoland/deno:distroless` as the container base image.
**Rationale:** Distroless images contain only the runtime and its dependencies -- no shell, no package manager, no debugging tools. This minimizes attack surface. The Deno distroless image includes the Deno binary and nothing else. Combined with gVisor at the K8s level, this creates defense-in-depth: sandboxed runtime inside a minimal container inside a sandboxed kernel. Alternative considered: `denoland/deno:alpine` -- rejected because alpine includes a shell and package manager, which increases attack surface without providing operational benefit (debugging happens locally via `pipedreamer dev`).

### Decision 2: Engine copied into container, CLI excluded
**Choice:** Copy the `engine/` directory into the container as `.engine/`. The Go CLI binary is never included.
**Rationale:** The container only needs the Deno engine to execute workflows. The CLI is a developer/operator tool that runs on the host machine and talks to K8s via client-go. Including the CLI in the container would add ~30MB of unnecessary binary and create confusion about the execution model. The build command copies the engine from the pipedreamer installation directory into a temporary `.engine/` directory within the workflow's build context, then the Dockerfile `COPY`s it into `/app/engine/`.

### Decision 3: K8s manifests with gVisor RuntimeClass
**Choice:** Generated Deployment manifests include `runtimeClassName: gvisor`.
**Rationale:** gVisor provides an application kernel that intercepts syscalls, adding a security boundary between the container and the host kernel. This is the "Fortress" pattern from v1. The `pipedreamer cluster check` command (already implemented in `pkg/k8s/preflight.go`) validates that the gVisor RuntimeClass exists before deployment. Alternative considered: making gVisor optional -- rejected because it is a core security guarantee of the platform. Operators who cannot run gVisor should not use pipedreamer in production.

### Decision 4: Secrets as K8s Secret volume mounts
**Choice:** Secrets are mounted from a K8s Secret named `<workflow-name>-secrets` as a read-only volume at `/app/secrets`.
**Rationale:** Environment variables are visible in `kubectl describe pod`, process listings, and crash dumps. Volume mounts from K8s Secrets are only visible inside the container filesystem. The engine reads secrets from `/app/secrets` at startup. The Secret is marked `optional: true` so workflows without secrets can deploy without creating an empty Secret. Alternative considered: using env vars from Secret `envFrom` -- rejected because env vars leak in logs and process tables.

### Decision 5: Image tag derived from workflow name + version
**Choice:** Default image tag is `<workflow-name>:<version-with-dots-replaced-by-dashes>`. With `--registry`, it becomes `<registry>/<workflow-name>:<version>`.
**Rationale:** Predictable, deterministic tags that map directly to the workflow spec. Version dots are replaced with dashes in the tag to avoid confusion with Docker tag conventions. The `--tag` flag allows override for CI/CD scenarios. Example: workflow `data-pipeline` version `1.0` becomes `data-pipeline:1-0`, or with registry `gcr.io/myproject/data-pipeline:1-0`.

### Decision 6: client-go for K8s operations
**Choice:** Use `k8s.io/client-go` directly for all K8s API operations.
**Rationale:** Same approach as v1. client-go is the official Go client for K8s with full API coverage. The dynamic client is used for apply operations (create-or-update pattern via Get/Create/Update), and the typed clientset is used for status queries. Alternative considered: shelling out to `kubectl` -- rejected because it adds a runtime dependency, parsing kubectl output is brittle, and client-go provides programmatic error handling.

### Decision 7: Build context temporary directory
**Choice:** The `build` command creates a temporary `.engine/` directory in the workflow's build context, copies the engine into it, builds the image, then cleans up.
**Rationale:** Docker build context must contain all files referenced by COPY instructions. The engine lives outside the workflow directory (it's part of the pipedreamer installation), so it must be copied into the build context. Using `.engine/` as the directory name and cleaning up with `defer os.RemoveAll()` ensures no artifacts are left behind. The generated `Dockerfile.pipedreamer` is also cleaned up after the build.

## Risks / Trade-offs

- **Docker CLI dependency** -- The `build` command shells out to `docker build`. This requires Docker to be installed and running. Mitigated by providing a clear error message if the docker command fails. Future work could support `podman` or `buildah` as alternatives.
- **gVisor requirement** -- Mandating gVisor limits the set of K8s clusters that can run pipedreamer. Mitigated by the `pipedreamer cluster check` preflight command that detects missing RuntimeClass and provides remediation steps.
- **Single-replica default** -- Deployment defaults to 1 replica. This is appropriate for workflow engines that maintain in-memory DAG state. Scaling out requires stateless execution or external state management, which is out of scope.
- **No image push** -- The `build` command builds locally but does not push to a registry. Users must `docker push` separately. This is intentional: keeping build and push as separate steps gives operators control over their CI/CD pipeline.
- **Resource limits are hardcoded** -- Memory (64Mi request, 256Mi limit) and CPU (100m request, 500m limit) are hardcoded in the manifest. Future work could make these configurable via workflow.yaml `config` section.
