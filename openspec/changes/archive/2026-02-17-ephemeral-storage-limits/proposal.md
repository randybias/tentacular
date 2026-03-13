## Why

The `/tmp` emptyDir volume in generated Deployments has no size limit (`emptyDir: {}`). A compromised or buggy workflow could fill the node's ephemeral storage, causing eviction of other pods. Adding a `sizeLimit` caps the volume and triggers pod eviction before node-level damage occurs.

## What Changes

- Change `emptyDir: {}` to `emptyDir` with `sizeLimit: 512Mi` in the Deployment template in `pkg/builder/k8s.go`
- Update test assertion in `pkg/builder/k8s_test.go` to expect the new format

## Capabilities

### New Capabilities
- `ephemeral-storage-limit`: Set sizeLimit on emptyDir tmp volume in generated Deployment pod specs

### Modified Capabilities
<!-- None -->

## Impact

- `pkg/builder/k8s.go`: Change the tmp volume definition from `emptyDir: {}` to multi-line `emptyDir` with `sizeLimit: 512Mi`
- `pkg/builder/k8s_test.go`: Update `TestK8sManifestVolumes` assertion from `emptyDir: {}` to `sizeLimit: 512Mi`
