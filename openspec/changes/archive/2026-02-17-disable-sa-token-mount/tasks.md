## 1. Implementation

- [ ] 1.1 Add `automountServiceAccountToken: false` to pod spec in `pkg/builder/k8s.go` Deployment template, after the `securityContext` block

## 2. Testing

- [ ] 2.1 Add `TestK8sManifestDisableSATokenMount` test in `pkg/builder/k8s_test.go` asserting `automountServiceAccountToken: false` appears in generated Deployment content
