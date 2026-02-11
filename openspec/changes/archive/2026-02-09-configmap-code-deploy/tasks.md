## 1. ConfigMap Generation

- [x] 1.1 Add `GenerateCodeConfigMap(wf *spec.Workflow, workflowDir, namespace string) (Manifest, error)` to `pkg/builder/k8s.go`
- [x] 1.2 Read `workflow.yaml` from `workflowDir` and add as ConfigMap data key `workflow.yaml`
- [x] 1.3 Read all `*.ts` files from `workflowDir/nodes/` and add as ConfigMap data keys (e.g., `nodes/fetch.ts`)
- [x] 1.4 Handle missing `nodes/` directory gracefully (no error, just workflow.yaml in ConfigMap)
- [x] 1.5 Skip non-`.ts` files in `nodes/` directory
- [x] 1.6 Set ConfigMap name to `{wf.Name}-code`, namespace, and labels (`app.kubernetes.io/name`, `app.kubernetes.io/managed-by`)
- [x] 1.7 Calculate total data size and return error if > 900KB (921600 bytes) with descriptive message including actual size

## 2. Deployment Volume Mount

- [x] 2.1 Add `code` volume to Deployment pod spec in `GenerateK8sManifests()`: `configMap: { name: {wf.Name}-code }`
- [x] 2.2 Add volumeMount to engine container: `name: code, mountPath: /app/workflow, readOnly: true`
- [x] 2.3 Add container `args: ["--workflow", "/app/workflow/workflow.yaml", "--port", "8080"]` to engine container
- [x] 2.4 Verify existing volumes (secrets, tmp) are preserved in the Deployment template

## 3. Deploy Command Updates

- [x] 3.1 Add `--image` flag to `NewDeployCmd()` in `deploy.go`
- [x] 3.2 Implement image resolution cascade: `--image` flag > read `.tentacular/base-image.txt` > fallback `tentacular-engine:latest`
- [x] 3.3 Remove `--cluster-registry` flag usage from image tag derivation; add deprecation error if flag is used
- [x] 3.4 Call `builder.GenerateCodeConfigMap(wf, absDir, namespace)` and prepend ConfigMap to manifest list
- [x] 3.5 Return early with error if `GenerateCodeConfigMap` fails (e.g., size limit exceeded)
- [x] 3.6 After successful `client.Apply()`, call `client.RolloutRestart(namespace, wf.Name)` and print restart confirmation

## 4. K8s Client: RolloutRestart

- [x] 4.1 Add `RolloutRestart(namespace, deploymentName string) error` method to `pkg/k8s/client.go`
- [x] 4.2 Use strategic merge patch to set `spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]` to current timestamp
- [x] 4.3 Use 30-second timeout context consistent with other K8s operations

## 5. Tests

- [x] 5.1 Add `TestConfigMapGeneration` to `pkg/builder/k8s_test.go` — verify ConfigMap name, namespace, labels, data keys for workflow.yaml and nodes/*.ts
- [x] 5.2 Add `TestConfigMapSizeValidation` — verify error returned when data exceeds 900KB
- [x] 5.3 Add `TestConfigMapMissingNodesDir` — verify ConfigMap generated with only workflow.yaml when nodes/ doesn't exist
- [x] 5.4 Add `TestConfigMapSkipsNonTsFiles` — verify non-.ts files in nodes/ are excluded
- [x] 5.5 Add `TestDeploymentHasCodeVolumeMount` — verify code volume and mount in Deployment manifest
- [x] 5.6 Add `TestDeploymentContainerArgs` — verify Deployment container HAS `args: ["--workflow", "/app/workflow/workflow.yaml", "--port", "8080"]`
- [x] 5.7 Verify existing CronJob tests still pass (no regression from Deployment template changes)

## 6. Verification

- [x] 6.1 Run `go test ./pkg/builder/...` and verify all tests pass
- [x] 6.2 Run `go test ./pkg/cli/...` and verify deploy command compiles with new flags
- [x] 6.3 Run `go test ./pkg/k8s/...` if applicable
- [x] 6.4 Run `go build -o tntc ./cmd/tntc/` and verify binary compiles
