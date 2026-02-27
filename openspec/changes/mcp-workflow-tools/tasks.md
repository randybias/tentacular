## 1. wf_list Implementation

- [x] 1.1 Add `WfListParams` struct with Namespace, Tag, Owner optional fields in `discover.go`
- [x] 1.2 Add `WfListEntry` (name, namespace, version, owner, team, environment, ready, age) and `WfListResult` structs
- [x] 1.3 Implement `handleWfList()`: query Deployments with managed-by label, use `deploymentToListEntry()` helper, apply client-side tag/owner filters
- [x] 1.4 Register `wf_list` tool in `registerDiscoverTools()`

## 2. wf_describe Implementation

- [x] 2.1 Add `WfDescribeParams` struct with Name and Namespace required fields
- [x] 2.2 Add `WfDescribeResult` struct with annotation fields, replica status, image, nodes, triggers, tentacular.dev annotations
- [x] 2.3 Implement `handleWfDescribe()`: read Deployment annotations, best-effort parse -code ConfigMap for node names and trigger descriptions
- [x] 2.4 Add `minimalWorkflow` struct for lightweight ConfigMap YAML parsing (avoids cross-repo dependency on spec package)
- [x] 2.5 Register `wf_describe` tool in `registerDiscoverTools()`

## 3. Helpers

- [x] 3.1 Add `containsTag()` for comma-separated tag matching
- [x] 3.2 Add `derefInt32()` for replica pointer dereferencing
- [x] 3.3 Add `wrapListError()` and `wrapGetError()` error helpers

## 4. Tests

- [x] 4.1 Complete test cases in `workflow_meta_test.go` for wf_list (all workflows, filtered by namespace/tag/owner)
- [x] 4.2 Complete test cases for wf_describe (full detail, missing ConfigMap)
- [x] 4.3 Run `go test ./pkg/tools/...` -- all pass
- [x] 4.4 Run `go test ./...` -- all pass
