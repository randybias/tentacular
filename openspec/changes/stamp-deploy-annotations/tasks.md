## 1. Annotation Builder

- [x] 1.1 Add `buildDeployAnnotations(meta *spec.WorkflowMetadata) string` function in `pkg/builder/k8s.go`
- [x] 1.2 Derive annotations from meta.Owner, meta.Team, meta.Tags (comma-joined), meta.Environment
- [x] 1.3 Return empty string when metadata is nil
- [x] 1.4 Omit annotations with empty string values

## 2. Manifest Integration

- [x] 2.1 Inject `buildDeployAnnotations(wf.Metadata)` into Deployment metadata YAML in `GenerateK8sManifests()`
- [x] 2.2 Inject `buildDeployAnnotations(wf.Metadata)` into Service metadata YAML

## 3. Tests

- [x] 3.1 Add test for `buildDeployAnnotations()` with full metadata
- [x] 3.2 Add test for `buildDeployAnnotations()` with nil metadata (empty string returned)
- [x] 3.3 Add test verifying empty values are omitted
- [x] 3.4 Run `go test ./pkg/builder/...` -- all pass
- [x] 3.5 Run `go test ./pkg/...` -- all pass
