## 1. Implementation

- [ ] 1.1 Change `emptyDir: {}` to multi-line `emptyDir` with `sizeLimit: 512Mi` in `pkg/builder/k8s.go` Deployment template

## 2. Testing

- [ ] 2.1 Update `TestK8sManifestVolumes` in `pkg/builder/k8s_test.go` to assert `sizeLimit: 512Mi` instead of `emptyDir: {}`
