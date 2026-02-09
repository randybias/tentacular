## 1. Dockerfile Generation

- [x] 1.1 Implement `pkg/builder/dockerfile.go` `GenerateDockerfile()` — returns a Dockerfile string using `denoland/deno:distroless` base, `WORKDIR /app`, copies `.engine/`, `workflow.yaml`, `nodes/`, and `deno.json` import map
- [x] 1.2 Verify Dockerfile includes `RUN deno cache engine/main.ts` for dependency pre-caching
- [x] 1.3 Verify ENTRYPOINT uses locked-down Deno permissions: `--allow-net`, `--allow-read=/app`, `--allow-write=/tmp`
- [x] 1.4 Verify ENTRYPOINT passes `--workflow /app/workflow.yaml --port 8080` to `engine/main.ts`
- [x] 1.5 Verify no CLI binary, `cmd/`, or `pkg/` directories are copied into the container

## 2. Build Command

- [x] 2.1 Implement `pkg/cli/build.go` `NewBuildCmd()` — cobra command for `build [dir]` with `--tag` flag
- [x] 2.2 Implement `runBuild()` — reads `workflow.yaml`, parses and validates spec, derives image tag from workflow name + version
- [x] 2.3 Implement engine directory discovery via `findEngineDir()` — locates `engine/` relative to the pipedreamer binary
- [x] 2.4 Implement build context setup — copy engine into `.engine/` temp dir within workflow directory
- [x] 2.5 Implement Dockerfile generation and `docker build` invocation with `-f Dockerfile.pipedreamer -t <tag>`
- [x] 2.6 Implement deferred cleanup — remove `Dockerfile.pipedreamer` and `.engine/` after build (success or failure)
- [x] 2.7 Implement `--registry` flag support — prefix image tag with registry URL when provided
- [x] 2.8 Verify `pipedreamer build` fails with clear error when `workflow.yaml` is missing or invalid

## 3. K8s Manifest Generation

- [x] 3.1 Implement `pkg/builder/k8s.go` `GenerateK8sManifests(wf, imageTag, namespace)` — returns slice of `Manifest` structs
- [x] 3.2 Generate Deployment manifest with gVisor RuntimeClass (`runtimeClassName: gvisor`), pipedreamer labels, and resource limits (64Mi/256Mi memory, 100m/500m CPU)
- [x] 3.3 Generate Deployment with secret volume mount at `/app/secrets` (read-only) from K8s Secret `<name>-secrets` (optional: true)
- [x] 3.4 Generate Deployment with `emptyDir` tmp volume mounted at `/tmp`
- [x] 3.5 Generate Service manifest with ClusterIP type, port 8080 mapping, and matching selector labels
- [x] 3.6 Verify no env var or envFrom fields reference secrets in generated manifests

## 4. K8s Client

- [x] 4.1 Implement `pkg/k8s/client.go` `NewClient()` — creates client from in-cluster config or kubeconfig fallback (`$KUBECONFIG` or `~/.kube/config`)
- [x] 4.2 Implement `Apply(namespace, manifests)` — create-or-update semantics using dynamic client: Get existing, Create if NotFound, Update with resourceVersion if exists
- [x] 4.3 Implement `findResource(group, version, kind)` — maps K8s kinds (Deployment, Service, ConfigMap, Secret) to API resource names
- [x] 4.4 Implement `GetStatus(namespace, name)` — queries Deployment via typed clientset, returns `DeploymentStatus` with readiness, replica counts
- [x] 4.5 Implement `DeploymentStatus.JSON()` and `DeploymentStatus.Text()` — formatted output methods for status reporting

## 5. Deploy Command

- [x] 5.1 Implement `pkg/cli/deploy.go` `NewDeployCmd()` — cobra command for `deploy [dir]`
- [x] 5.2 Implement `runDeploy()` — reads and validates workflow spec, constructs image tag with optional registry prefix
- [x] 5.3 Generate K8s manifests via `builder.GenerateK8sManifests()`
- [x] 5.4 Apply manifests via `k8s.Client.Apply()` targeting the `--namespace` flag value
- [x] 5.5 Print deployment confirmation with workflow name and namespace

## 6. Status Command

- [x] 6.1 Implement `pkg/cli/status.go` `NewStatusCmd()` — cobra command for `status <name>` (exactly 1 arg required)
- [x] 6.2 Implement `runStatus()` — queries deployment via `k8s.Client.GetStatus()` using workflow name and `--namespace`
- [x] 6.3 Support `--output json` for JSON formatted status, default to text format
- [x] 6.4 Verify status reports "ready" when `ReadyReplicas == Replicas`, "not ready" otherwise

## 7. Verification

- [x] 7.1 Verify `go build ./cmd/pipedreamer/` compiles with all build/deploy/status changes
- [x] 7.2 Verify `pipedreamer build --help` shows usage with `--tag` flag
- [x] 7.3 Verify `pipedreamer deploy --help` shows usage with namespace flag
- [x] 7.4 Verify `pipedreamer status --help` shows usage requiring name argument
- [x] 7.5 Verify generated Dockerfile produces a valid container image (manual test with a sample workflow)
- [x] 7.6 Verify generated K8s manifests are valid YAML that `kubectl apply --dry-run=client` accepts
