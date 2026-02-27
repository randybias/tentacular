## Why

MCP tools need to discover workflow metadata from running K8s Deployments without parsing ConfigMaps. By stamping `tentacular.dev/*` annotations on Deployment metadata during `tntc deploy`, MCP tools can read workflow details directly from the Deployment resource via the K8s API.

## What Changes

- Add `buildDeployAnnotations()` helper in `pkg/builder/k8s.go` that converts `WorkflowMetadata` into a YAML annotations block
- Annotations include: `tentacular.dev/owner`, `tentacular.dev/team`, `tentacular.dev/tags`, `tentacular.dev/environment`
- Inject annotation block into Deployment and Service metadata in `GenerateK8sManifests()`
- Only emit annotations for non-empty values (no empty annotations)
- Returns empty string when metadata is nil (backwards-compatible)

## Capabilities

### New Capabilities
- `deploy-annotations`: Stamp workflow metadata as `tentacular.dev/*` annotations on K8s Deployment and Service manifests during generation. Annotations derived from `WorkflowMetadata` fields (owner, team, tags, environment).

### Modified Capabilities
<!-- None -->

## Impact

- `pkg/builder/k8s.go`: New `buildDeployAnnotations()` function, modified `GenerateK8sManifests()` to inject annotations into both Deployment and Service
- `pkg/builder/k8s_test.go`: Tests for annotation generation
- Depends on Phase 1 (workflow-metadata-spec) for `WorkflowMetadata` struct
